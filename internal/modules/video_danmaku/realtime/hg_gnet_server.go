package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	VideoDanmakuServicePackage "MLC_GO/internal/modules/video_danmaku/service"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gobwas/ws"
	"github.com/panjf2000/gnet/v2"
)

// hgConnection 的协议、房间和限流字段只在所属 event-loop 内修改。pending 避免票据校验期间
// 重复解析同一握手；原子活动时间与时间轮元数据按各自锁边界供后台心跳线程访问。
type hgConnection struct {
	upgraded, pending bool
	videoID, userID   string
	handshake         []byte
	fragmentPayload   []byte
	fragmentOpCode    ws.OpCode
	memberShard       uint32
	commandTokens     float64
	commandRefillNano int64
	// pendingBytes 是跨 goroutine 使用的出站预算。gnet 的 WriteBufferCap 超过后会转入弹性缓冲，
	// 所以必须在应用层设置硬上限，避免慢客户端在热门房间中持续堆积广播直至 OOM。
	pendingBytes atomic.Int64
	lastActive   atomic.Int64
	counted      atomic.Bool
	closed       atomic.Bool
	// draining 在服务端发送 Close Frame 前置位；RFC 6455 Close 后禁止再发送业务数据帧。
	draining atomic.Bool
	// websocketCounted 只在 Upgrade 成功后置位，并由关闭路径原子释放一次，避免异步升级与断连重叠时 gauge 变负。
	websocketCounted atomic.Bool
	// heartbeatConn 在 Upgrade 成功后只写一次；时间轮后台线程不得调用非并发安全的
	// gnet.Conn.Context，因此直接保存连接及其时间轮分片位置。
	heartbeatConn      gnet.Conn
	heartbeatShard     uint32
	heartbeatSlot      uint32
	heartbeatScheduled bool
	heartbeatClosed    atomic.Bool
}

type hgRoomMemberShard struct {
	mu      sync.RWMutex
	members map[string]map[gnet.Conn]*hgConnection
}

type hgRoom struct {
	memberCount atomic.Int64
	nextShard   atomic.Uint32
}

type hgRoomDirectoryShard struct {
	mu      sync.RWMutex
	rooms   map[string]*hgRoom
	members []hgRoomMemberShard
}

type hgBroadcastEnvelope struct {
	Type string                                 `json:"type"`
	Data VideoDanmakuDtoPackage.DanmakuResponse `json:"data"`
}

type hgCommandAckEnvelope struct {
	Type      string                                 `json:"type"`
	RequestID string                                 `json:"requestId"`
	Data      VideoDanmakuDtoPackage.DanmakuResponse `json:"data"`
}

type hgCommandErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type hgCommandErrorEnvelope struct {
	Type      string             `json:"type"`
	RequestID string             `json:"requestId,omitempty"`
	Data      hgCommandErrorData `json:"data"`
}

// hgCommand 将 event-loop 已完成协议解析的创建请求交给有界 worker。
// requestID 同时用于数据库幂等和 WebSocket ack/error 关联，客户端断线回退 HTTP 时必须保持不变。
type hgCommand struct {
	conn      gnet.Conn
	state     *hgConnection
	userID    string
	requestID string
	request   VideoDanmakuDtoPackage.CreateRequest
}

var (
	hgErrPendingWriteLimit  = errors.New("danmaku connection pending write limit exceeded")
	hgErrConnectionDraining = errors.New("danmaku connection is draining")
)

var (
	hgErrFragmentMessageTooBig = errors.New("danmaku fragmented message exceeds limit")
	hgErrUnsupportedData       = errors.New("danmaku websocket data type is unsupported")
)

type hgLifecycleState int32

const (
	hgLifecycleStarting hgLifecycleState = iota
	hgLifecycleServing
	hgLifecycleDraining
	hgLifecycleStopped
)

// Server 是独立端口的 gnet WebSocket 弹幕网关；事件循环不执行 MySQL/Redis 阻塞 I/O。
type Server struct {
	gnet.BuiltinEventEngine
	service   *VideoDanmakuServicePackage.Service
	redis     *PersistenceRedisPackage.RedisService
	config    ConfigPackage.HGVideoDanmakuConfig
	engine    gnet.Engine
	lifecycle atomic.Int32 //原子存储服务生命周期状态
	// lifecycleMu 只协调 Serving->Draining 与异步握手激活，避免连接在 Drain 快照之后才加入房间。
	// 房间扫描和网络写不持有该锁，防止百万连接关闭阶段形成长临界区。
	lifecycleMu    sync.RWMutex
	connections    atomic.Int64
	queue          chan hgCommand
	broadcastQueue []chan VideoDanmakuDtoPackage.DanmakuResponse //预分配的广播任务队列数组
	roomShards     []hgRoomDirectoryShard
	roomRouter     *hgRoomRouter   //房间路由模块，管理直播间、用户连接路由
	workers        sync.WaitGroup  //sync.WaitGroup，管控业务 worker 协程
	background     sync.WaitGroup  //管控后台常驻协程
	workerCtx      context.Context //带取消的 context，用于整体关闭通知
	cancel         context.CancelFunc
	handshakeSlots chan struct{}
	heartbeatPing  []byte
	heartbeatWheel *hgHeartbeatWheel
	metrics        hgRealtimeMetrics
	drainStartedAt atomic.Int64
	drainObserved  atomic.Bool
	closeStarted   atomic.Bool
}

// NewServer 创建具有有界工作队列、连接上限和帧上限的实时网关。
func NewServer(service *VideoDanmakuServicePackage.Service, redis *PersistenceRedisPackage.RedisService, config ConfigPackage.HGVideoDanmakuConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	if config.BroadcastWorkerCount <= 0 {
		config.BroadcastWorkerCount = 1
	}
	if config.BroadcastQueueSize <= 0 {
		config.BroadcastQueueSize = 64
	}
	if config.HeartbeatShardCount <= 0 {
		config.HeartbeatShardCount = 64
	}
	roomShards := make([]hgRoomDirectoryShard, config.RoomShardCount)
	for index := range roomShards {
		roomShards[index].rooms = make(map[string]*hgRoom)
		roomShards[index].members = make([]hgRoomMemberShard, config.MemberShardCount)
		for memberIndex := range roomShards[index].members {
			roomShards[index].members[memberIndex].members = make(map[string]map[gnet.Conn]*hgConnection)
		}
	}
	broadcastQueue := make([]chan VideoDanmakuDtoPackage.DanmakuResponse, config.BroadcastWorkerCount)
	for index := range broadcastQueue {
		broadcastQueue[index] = make(chan VideoDanmakuDtoPackage.DanmakuResponse, config.BroadcastQueueSize)
	}
	heartbeatPing, _ := ws.CompileFrame(ws.NewPingFrame(nil))
	server := &Server{service: service, redis: redis, config: config, queue: make(chan hgCommand, config.QueueSize), broadcastQueue: broadcastQueue, roomShards: roomShards, workerCtx: ctx, cancel: cancel, handshakeSlots: make(chan struct{}, config.WorkerCount*4), heartbeatPing: heartbeatPing}
	server.heartbeatWheel = newHGHeartbeatWheel(config.HeartbeatShardCount, config.HeartbeatInterval, config.HeartbeatTimeout)
	server.roomRouter = newHGRoomRouter(redis, config.BroadcastQueueSize, server.hgEnqueueBroadcast)
	return server
}

// Addr 返回 gnet 监听地址。
func (s *Server) Addr() string { return "tcp://" + s.config.Host + ":" + s.config.Port }

// DrainTimeout 返回 SIGTERM 后保留现有 WebSocket 完成关闭握手和重连的最长窗口。
func (s *Server) DrainTimeout() time.Duration {
	if s == nil {
		return 0
	}
	return s.config.DrainTimeout
}

// Serve 服务入口，启动多组后台 worker 协程，最后调用 gnet.Run() 启动网络事件循环，阻塞。
func (s *Server) Serve() error {
	/** 1. 启动普通业务worker池
	根据配置 WorkerCount 启动 N 个 hgWorker goroutine。
	s.workers WaitGroup，所有 worker 启动前 Add (1)；worker 退出内部调用 s.workers.Done()。
	hgWorker()：一般是消费任务 channel，处理弹幕消息、业务逻辑。
	*/
	for i := 0; i < s.config.WorkerCount; i++ {
		s.workers.Add(1)
		go s.hgWorker()
	}

	/** 2. 为每一个broadcastQueue队列启动广播worker
	- broadcastQueue 是一组队列（数组），每个队列绑定一个独立广播协程。
	- 设计意图：广播弹幕给直播间大量用户，做分片、分片隔离，避免单条广播队列阻塞全部直播间。
	  - 比如：把直播间 hash 分配到不同 broadcastQueue，每个 queue 一个 goroutine 负责推送消息。
	- workers WaitGroup 同时管控普通 worker + broadcast worker；服务关闭时 s.workers.Wait() 等待全部业务 worker 退出。
	*/
	for index := range s.broadcastQueue {
		s.workers.Add(1)
		go s.hgBroadcastWorker(s.broadcastQueue[index])
	}
	// 3. 启动房间路由后台协程
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		// 房间路由主循环，管理直播间、用户连接映射、房间过期清理、rebalance 等逻辑。
		/**
		- s.workerCtx.Err() == nil → context还没有被主动取消。
		- 含义：不是我们主动关闭上下文导致的退出，而是 roomRouter 内部发生意外错误，此时原子把服务生命周期标记为 hgLifecycleStopped，上层感知服务异常停止。
		- 如果是主动关闭（workerCtx cancel），即使 roomRouter 返回 err，也不会修改 lifecycle 状态，属于正常关闭。

		defer s.background.Done()：协程无论 panic 与否？❗⚠️注意：如果 roomRouter.Run() 内部 panic，defer s.background.Done() 不会执行，WaitGroup 永久等待，goroutine 泄漏。生产代码这里需要 recover 捕获 panic。
		*/
		if err := s.roomRouter.Run(s.workerCtx); err != nil && s.workerCtx.Err() == nil {
			s.lifecycle.Store(int32(hgLifecycleStopped))
		}
	}()
	/** 4. 心跳循环后台协程
	- hgHeartbeatLoop()：后台心跳循环，维护客户端心跳、超时踢连接、定时上报指标等。
	- 内部逻辑应当监听 s.workerCtx.Done()，收到取消信号退出，内部调用 s.background.Done()。
	*/
	s.background.Add(1)
	go s.hgHeartbeatLoop()

	// 5. 启动gnet事件驱动网络，阻塞，直到服务停止返回error； gnet 是 evio 风格 Reactor 高性能网络库。
	return gnet.Run(s,
		s.Addr(),
		gnet.WithMulticore(true), // 多核模式，goroutine 绑定 CPU，充分利用多核。
		gnet.WithReadBufferCap(s.config.MaxHandshakeBytes), // 每个连接读缓冲区最大容量，握手阶段最大字节限制
		gnet.WithWriteBufferCap(16<<10),                    // (16<<10) = 16KB，每个连接写缓冲区大小。
		gnet.WithTCPKeepAlive(90*time.Second))              // TCP keep‑alive 90 秒，检测死连接。
}

// Ready 表示 gnet 已完成 OnBoot、房间路由可用且尚未进入 Drain。
func (s *Server) Ready() error {
	if s == nil || hgLifecycleState(s.lifecycle.Load()) != hgLifecycleServing {
		return errors.New("danmaku realtime not ready")
	}
	if err := s.roomRouter.Ready(); err != nil {
		return err
	}
	return nil
}

// BeginDrain 原子撤销 readiness 和新连接准入，并向现有 WebSocket 发送标准 Going Away。
// 这里只发起 RFC 6455 关闭握手而不立即关闭 TCP，让配合标准 Close 的客户端有机会主动重连到 Ready Pod。
func (s *Server) BeginDrain() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	for {
		state := hgLifecycleState(s.lifecycle.Load())
		if state == hgLifecycleDraining || state == hgLifecycleStopped {
			s.lifecycleMu.Unlock()
			return
		}
		if state != hgLifecycleServing {
			s.lifecycleMu.Unlock()
			return
		}
		if s.lifecycle.CompareAndSwap(int32(state), int32(hgLifecycleDraining)) {
			s.drainStartedAt.Store(time.Now().UnixNano())
			s.metrics.drainStarts.Add(1)
			break
		}
	}
	s.lifecycleMu.Unlock()
	connections := s.hgWebSocketConnections()
	for _, connection := range connections {
		connection.state.draining.Store(true)
		if err := s.hgWriteFrame(connection.conn, connection.state, ws.CompiledCloseGoingAway); err != nil {
			_ = connection.conn.Close()
		}
	}
}

// WaitForDrain 等待已认证 WebSocket 自然完成关闭握手；ctx 到期后由 Close 强制回收残余连接。
func (s *Server) WaitForDrain(ctx context.Context) error {
	if s == nil || s.metrics.websocketConnections.Load() == 0 {
		return nil
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.metrics.drainTimeouts.Add(1)
			return ctx.Err()
		case <-ticker.C:
			if s.metrics.websocketConnections.Load() == 0 {
				return nil
			}
		}
	}
}

type hgConnectionState struct {
	conn  gnet.Conn
	state *hgConnection
}

func (s *Server) hgWebSocketConnections() []hgConnectionState {
	conns := make([]hgConnectionState, 0, int(s.metrics.websocketConnections.Load()))
	for index := range s.roomShards {
		directory := &s.roomShards[index]
		directory.mu.RLock()
		for memberIndex := range directory.members {
			members := &directory.members[memberIndex]
			members.mu.RLock()
			for _, roomMembers := range members.members {
				for conn, state := range roomMembers {
					conns = append(conns, hgConnectionState{conn: conn, state: state})
				}
			}
			members.mu.RUnlock()
		}
		directory.mu.RUnlock()
	}
	return conns
}

// Close 强制关闭 Drain 后的残余连接，并等待有界 worker 与 gnet 退出。
func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.BeginDrain()
	conns := s.hgWebSocketConnections()
	for _, connection := range conns {
		if connection.state.closed.CompareAndSwap(false, true) {
			s.metrics.forceClosedConnections.Add(1)
			_ = connection.conn.Close()
		}
	}
	if s.closeStarted.CompareAndSwap(false, true) {
		s.cancel()
	}
	done := make(chan struct{})
	go func() { s.workers.Wait(); s.background.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := s.engine.Validate(); err == nil {
		if err := s.engine.Stop(ctx); err != nil {
			return err
		}
	}
	s.hgObserveDrainDuration()
	s.lifecycle.Store(int32(hgLifecycleStopped))
	return nil
}

func (s *Server) OnBoot(engine gnet.Engine) gnet.Action {
	s.engine = engine
	s.lifecycle.CompareAndSwap(int32(hgLifecycleStarting), int32(hgLifecycleServing))
	return gnet.None
}
func (s *Server) OnShutdown(gnet.Engine) { s.lifecycle.Store(int32(hgLifecycleStopped)); s.cancel() }
func (s *Server) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	state := &hgConnection{}
	state.counted.Store(true)
	state.lastActive.Store(time.Now().UnixNano())
	c.SetContext(state)
	if s.connections.Add(1) > int64(s.config.MaxConnections) || hgLifecycleState(s.lifecycle.Load()) != hgLifecycleServing {
		return nil, gnet.Close
	}
	return nil, gnet.None
}
func (s *Server) OnClose(c gnet.Conn, _ error) gnet.Action {
	state, _ := c.Context().(*hgConnection)
	if state != nil {
		// Upgrade 校验在后台执行，连接可能在 runnable 回到 event-loop 前关闭。
		// 先发布 closed 状态，阻止迟到的 runnable 再注册房间和增加 WebSocket gauge。
		state.closed.Store(true)
	}
	if state != nil && state.counted.CompareAndSwap(true, false) {
		s.connections.Add(-1)
	}
	if state != nil && state.videoID != "" {
		s.hgReleaseWebSocketConnection(state)
		s.heartbeatWheel.hgCancel(state)
		s.hgLeave(state.videoID, c)
		s.roomRouter.Leave(state.videoID)
	}
	return gnet.None
}

// OnTraffic 仅执行内存解析、票据任务触发和有界队列投递；不阻塞事件循环访问外部依赖。
func (s *Server) OnTraffic(c gnet.Conn) gnet.Action {
	state, _ := c.Context().(*hgConnection)
	if state == nil {
		return gnet.Close
	}
	state.lastActive.Store(time.Now().UnixNano())
	if state.pending {
		if c.InboundBuffered() > s.config.MaxFrameBytes {
			return gnet.Close
		}
		return gnet.None
	}
	if !state.upgraded {
		return s.hgUpgrade(c, state)
	}
	if state.draining.Load() {
		// 服务端已发送 Close Frame 后只等待客户端 Close，其他数据帧直接结束连接，避免违反 RFC 6455。
		return gnet.Close
	}
	for c.InboundBuffered() >= 2 {
		buffered := c.InboundBuffered()
		raw, err := c.Peek(buffered)
		if err != nil {
			return gnet.Close
		}
		reader := bytes.NewReader(raw)
		header, err := ws.ReadHeader(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return gnet.None
			}
			return gnet.Close
		}
		if ws.CheckHeader(header, state.hgWebSocketState()) != nil {
			_ = s.hgWriteFrame(c, state, ws.CompiledCloseProtocolError)
			return gnet.Close
		}
		if header.Length > int64(s.config.MaxFrameBytes) {
			_ = s.hgWriteFrame(c, state, ws.CompiledCloseMessageTooBig)
			return gnet.Close
		}
		frameSize := buffered - reader.Len() + int(header.Length)
		if buffered < frameSize {
			return gnet.None
		}
		frameBytes, _ := c.Next(frameSize)
		payload := append([]byte(nil), frameBytes[frameSize-int(header.Length):]...)
		ws.Cipher(payload, header.Mask, 0)
		switch header.OpCode {
		case ws.OpPing:
			pong, _ := ws.CompileFrame(ws.NewPongFrame(payload))
			if s.hgWriteFrame(c, state, pong) != nil {
				return gnet.Close
			}
		case ws.OpPong:
		case ws.OpClose:
			_ = s.hgWriteFrame(c, state, ws.CompiledCloseNormalClosure)
			return gnet.Close
		case ws.OpText, ws.OpBinary, ws.OpContinuation:
			message, acceptErr := state.hgAcceptDataFrame(header, payload, s.config.MaxFrameBytes)
			if acceptErr != nil {
				closeFrame := ws.CompiledCloseProtocolError
				if errors.Is(acceptErr, hgErrFragmentMessageTooBig) {
					closeFrame = ws.CompiledCloseMessageTooBig
				} else if errors.Is(acceptErr, hgErrUnsupportedData) {
					closeFrame = ws.CompiledCloseUnsupportedData
				}
				_ = s.hgWriteFrame(c, state, closeFrame)
				return gnet.Close
			}
			if message == nil {
				continue
			}
			if !utf8.Valid(message) {
				_ = s.hgWriteFrame(c, state, ws.CompiledCloseInvalidFramePayloadData)
				return gnet.Close
			}
			s.metrics.commandsReceived.Add(1)
			var command struct {
				Type    string                               `json:"type"`
				Request VideoDanmakuDtoPackage.CreateRequest `json:"data"`
			}
			if json.Unmarshal(message, &command) != nil || command.Type != "danmaku.create" {
				s.metrics.commandRejections[hgCommandRejectionInvalid].Add(1)
				if s.hgError(c, state, "invalid_command", "弹幕命令无效") != nil {
					return gnet.Close
				}
				continue
			}
			if !s.hgAllowCommand(state, time.Now()) {
				s.metrics.commandRejections[hgCommandRejectionRateLimited].Add(1)
				if s.hgCommandError(c, state, command.Request.RequestID, "rate_limited", "弹幕发送过于频繁，请稍后重试") != nil {
					return gnet.Close
				}
				continue
			}
			command.Request.VideoID = state.videoID
			select {
			case s.queue <- hgCommand{conn: c, state: state, userID: state.userID, requestID: command.Request.RequestID, request: command.Request}:
			default:
				s.metrics.commandRejections[hgCommandRejectionQueueFull].Add(1)
				if s.hgCommandError(c, state, command.Request.RequestID, "busy", "弹幕服务繁忙，请稍后重试") != nil {
					return gnet.Close
				}
			}
		default:
			_ = s.hgWriteFrame(c, state, ws.CompiledCloseUnsupportedData)
			return gnet.Close
		}
	}
	return gnet.None
}

// hgWebSocketState 将 event-loop 内的重组状态交给 gobwas 校验，确保分片期间只接受
// Continuation 数据帧，同时仍允许 RFC 6455 控制帧穿插在分片消息之间。
func (state *hgConnection) hgWebSocketState() ws.State {
	webSocketState := ws.StateServerSide
	if state.fragmentOpCode != 0 {
		webSocketState = webSocketState.Set(ws.StateFragmented)
	}
	return webSocketState
}

// hgAcceptDataFrame 在单连接 event-loop 内完成有界重组。MaxFrameBytes 同时限制单帧和
// 完整消息，避免攻击者用大量合法小分片绕过入站内存边界；返回 nil 表示仍等待后续分片。
func (state *hgConnection) hgAcceptDataFrame(header ws.Header, payload []byte, maxMessageBytes int) ([]byte, error) {
	switch header.OpCode {
	case ws.OpBinary:
		return nil, hgErrUnsupportedData
	case ws.OpText:
		if header.Fin {
			return payload, nil
		}
		state.fragmentOpCode = header.OpCode
		// OnTraffic 已将当前帧 payload 从 gnet 入站缓冲复制为独立 slice，连接可直接
		// 接管首分片，避免在每条分片消息开始时再复制一次相同字节。
		state.fragmentPayload = payload
		return nil, nil
	case ws.OpContinuation:
		if state.fragmentOpCode == 0 {
			return nil, ws.ErrProtocolContinuationUnexpected
		}
		if len(payload) > maxMessageBytes-len(state.fragmentPayload) {
			state.fragmentPayload = nil
			state.fragmentOpCode = 0
			return nil, hgErrFragmentMessageTooBig
		}
		state.fragmentPayload = append(state.fragmentPayload, payload...)
		if !header.Fin {
			return nil, nil
		}
		message := state.fragmentPayload
		state.fragmentPayload = nil
		state.fragmentOpCode = 0
		return message, nil
	default:
		return nil, ws.ErrProtocolOpCodeReserved
	}
}

// hgHeartbeatLoop 使用单个固定 ticker 推进分片时间轮。后台 panic 会立即撤销 readiness，
// 交由编排系统摘流重启，避免实例在失去僵尸连接清理能力后继续接收新连接。
func (s *Server) hgHeartbeatLoop() {
	defer s.background.Done()
	defer func() {
		if recover() != nil {
			s.lifecycle.Store(int32(hgLifecycleStopped))
		}
	}()
	ticker := time.NewTicker(s.heartbeatWheel.tick)
	defer ticker.Stop()
	for {
		select {
		case <-s.workerCtx.Done():
			return
		case now := <-ticker.C:
			s.hgHeartbeatTick(now)
		}
	}
}

func (s *Server) hgHeartbeatTick(now time.Time) {
	deadline := now.Add(-s.heartbeatWheel.timeout).UnixNano()
	for _, state := range s.heartbeatWheel.hgAdvance() {
		if state.heartbeatClosed.Load() {
			continue
		}
		if state.draining.Load() {
			continue
		}
		conn := state.heartbeatConn
		if conn == nil || state.lastActive.Load() <= deadline {
			if conn != nil {
				_ = conn.Close()
			}
			continue
		}
		if err := s.hgWriteDataFrame(conn, state, s.heartbeatPing); err != nil {
			_ = conn.Close()
			continue
		}
		// AsyncWrite 成功仅表示写入 gnet 异步队列，不能刷新活动时间；只有客户端后续
		// Pong 或其他有效入站帧才能证明连接仍存活。
		if !s.heartbeatWheel.hgReschedule(state, now) {
			_ = conn.Close()
		}
	}
}

func (s *Server) hgUpgrade(c gnet.Conn, state *hgConnection) gnet.Action {
	buffered := c.InboundBuffered()
	if buffered > s.config.MaxHandshakeBytes {
		return gnet.Close
	}
	raw, err := c.Peek(buffered)
	if err != nil {
		return gnet.None
	}
	end := bytes.Index(raw, []byte("\r\n\r\n"))
	if end < 0 {
		return gnet.None
	}
	request := append([]byte(nil), raw[:end+4]...)
	_, _ = c.Discard(end + 4)
	requestURL, origin, err := hgHandshakeMetadata(request)
	if err != nil || requestURL.Path != VideoDanmakuServicePackage.HGWebSocketPath || !s.hgAllowedOrigin(origin) {
		return gnet.Close
	}
	ticket := requestURL.Query().Get("ticket")
	if ticket == "" {
		return gnet.Close
	}
	state.pending, state.handshake = true, request
	// Redis 是阻塞网络 I/O，绝不能放在 gnet event-loop。校验完成后通过 Execute 回到连接所属
	// event-loop，再执行 gobwas 握手并写 101；这样未授权连接永远收不到成功升级响应。
	select {
	case s.handshakeSlots <- struct{}{}:
	default:
		return gnet.Close
	}
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		defer func() { <-s.handshakeSlots }()
		ctx, cancel := context.WithTimeout(s.workerCtx, 2*time.Second)
		defer cancel()
		binding, consumeErr := s.service.ConsumeTicket(ctx, ticket)
		if consumeErr != nil {
			_ = c.Close()
			return
		}
		if subscribeErr := s.roomRouter.Join(ctx, binding.VideoID); subscribeErr != nil {
			_ = c.Close()
			return
		}
		executeErr := c.EventLoop().Execute(context.Background(), gnet.RunnableFunc(func(context.Context) error {
			var output bytes.Buffer
			_, upgradeErr := (ws.Upgrader{OnRequest: func(uri []byte) error {
				parsed, parseErr := url.ParseRequestURI(string(uri))
				if parseErr != nil || parsed.Path != VideoDanmakuServicePackage.HGWebSocketPath {
					return ws.RejectConnectionError(ws.RejectionReason("invalid websocket path"))
				}
				return nil
			}}).Upgrade(struct {
				io.Reader
				io.Writer
			}{Reader: bytes.NewReader(state.handshake), Writer: &output})
			state.handshake = nil
			if upgradeErr != nil {
				s.roomRouter.Leave(binding.VideoID)
				return c.EventLoop().Close(c)
			}
			if !s.hgActivateWebSocket(binding.VideoID, binding.UserID, c, state) {
				s.roomRouter.Leave(binding.VideoID)
				return c.EventLoop().Close(c)
			}
			// 只有仍处于 Serving 且本地状态激活成功后才发送 101，避免 Drain 期间迟到的
			// Redis 票据校验把客户端留在“收到 Upgrade、服务端却未注册”的半激活状态。
			if len(output.Bytes()) > 0 {
				_, _ = c.Write(output.Bytes())
			}
			// HTTP 握手和首个 WebSocket 帧可能在同一 TCP 包内，主动唤醒以处理已缓冲帧。
			_ = c.Wake(nil)
			return nil
		}))
		if executeErr != nil {
			s.roomRouter.Leave(binding.VideoID)
			_ = c.Close()
		}
	}()
	return gnet.None
}

func (s *Server) hgWorker() {
	defer s.workers.Done()
	for {
		select {
		case <-s.workerCtx.Done():
			return
		case command := <-s.queue:
			ctx, cancel := context.WithTimeout(s.workerCtx, 3*time.Second)
			item, err := s.service.Create(ctx, command.userID, command.request)
			cancel()
			if err != nil {
				s.metrics.commandCreateFailures.Add(1)
				// 未知错误只返回稳定文案，SQL driver 和事务上下文保留在服务端，不能泄露给客户端。
				s.hgCommandError(command.conn, command.state, command.requestID, "create_failed", "弹幕发送失败，请稍后重试")
				continue
			}
			// 广播面向整个房间，ack 只返回给发送连接。requestId 让浏览器能够在并发发送时
			// 精确完成对应 Promise；若 ack 丢失，客户端可用同一 requestId 通过 HTTP 幂等回退。
			s.hgCommandAck(command.conn, command.state, command.requestID, item)
		}
	}
}

// Publish 将已提交弹幕发布到 Redis，使不同实例上的本地房间都能收到；MySQL 历史仍是权威恢复来源。
func (s *Server) Publish(ctx context.Context, item VideoDanmakuDtoPackage.DanmakuResponse) error {
	err := s.roomRouter.Publish(ctx, item)
	if err != nil {
		s.metrics.publishResults[hgPublishFailure].Add(1)
		return err
	}
	s.metrics.publishResults[hgPublishSuccess].Add(1)
	return nil
}

func (s *Server) hgEnqueueBroadcast(item VideoDanmakuDtoPackage.DanmakuResponse) {
	if len(s.broadcastQueue) == 0 {
		return
	}
	queue := s.broadcastQueue[hgHashVideoID(item.VideoID)&uint32(len(s.broadcastQueue)-1)]
	select {
	case queue <- item:
	default:
		// 实时副本允许在过载时丢弃；客户端重连或切换时间窗后从权威历史恢复。
		s.metrics.broadcastQueueDropped.Add(1)
	}
}

func (s *Server) hgBroadcastWorker(queue <-chan VideoDanmakuDtoPackage.DanmakuResponse) {
	defer s.workers.Done()
	for {
		select {
		case <-s.workerCtx.Done():
			return
		case item := <-queue:
			_ = s.hgBroadcastLocal(item)
		}
	}
}

// hgBroadcastLocal 将跨实例事件编码一次后逐成员分片广播；它不创建与房间人数线性增长的
// 目标快照，也不会持有跨视频全局锁。每连接出站预算负责隔离慢客户端。
func (s *Server) hgBroadcastLocal(item VideoDanmakuDtoPackage.DanmakuResponse) error {
	startedAt := time.Now()
	s.metrics.broadcasts.Add(1)
	defer func() { s.metrics.hgObserveBroadcastDuration(time.Since(startedAt)) }()
	payload, err := json.Marshal(hgBroadcastEnvelope{Type: "danmaku.created", Data: item})
	if err != nil {
		return err
	}
	frame, err := ws.CompileFrame(ws.NewTextFrame(payload))
	if err != nil {
		return err
	}
	directory := s.hgRoomDirectory(item.VideoID)
	directory.mu.RLock()
	room := directory.rooms[item.VideoID]
	if room == nil {
		directory.mu.RUnlock()
		return nil
	}
	var slowConnections []gnet.Conn
	for index := range directory.members {
		members := &directory.members[index]
		members.mu.RLock()
		for conn, state := range members.members[item.VideoID] {
			if err = s.hgWriteDataFrame(conn, state, frame); err != nil {
				if errors.Is(err, hgErrConnectionDraining) {
					continue
				}
				s.metrics.recipientWrites[hgRecipientFailed].Add(1)
				slowConnections = append(slowConnections, conn)
			} else {
				s.metrics.recipientWrites[hgRecipientQueued].Add(1)
			}
		}
		members.mu.RUnlock()
	}
	directory.mu.RUnlock()
	for _, conn := range slowConnections {
		_ = conn.Close()
	}
	return nil
}

func (s *Server) hgJoin(videoID string, c gnet.Conn, state *hgConnection) {
	directory := s.hgRoomDirectory(videoID)
	directory.mu.Lock()
	room := directory.rooms[videoID]
	if room == nil {
		room = &hgRoom{}
		directory.rooms[videoID] = room
		s.metrics.activeRooms.Add(1)
	}
	state.memberShard = room.nextShard.Add(1) % uint32(len(directory.members))
	members := &directory.members[state.memberShard]
	members.mu.Lock()
	if members.members[videoID] == nil {
		members.members[videoID] = make(map[gnet.Conn]*hgConnection)
	}
	members.members[videoID][c] = state
	members.mu.Unlock()
	room.memberCount.Add(1)
	directory.mu.Unlock()
}
func (s *Server) hgLeave(videoID string, c gnet.Conn) {
	directory := s.hgRoomDirectory(videoID)
	directory.mu.Lock()
	room := directory.rooms[videoID]
	if room != nil {
		state, _ := c.Context().(*hgConnection)
		if state != nil && int(state.memberShard) < len(directory.members) {
			members := &directory.members[state.memberShard]
			members.mu.Lock()
			roomMembers := members.members[videoID]
			if _, exists := roomMembers[c]; exists {
				delete(roomMembers, c)
				if len(roomMembers) == 0 {
					delete(members.members, videoID)
				}
				if room.memberCount.Add(-1) == 0 {
					delete(directory.rooms, videoID)
					s.metrics.activeRooms.Add(-1)
				}
			}
			members.mu.Unlock()
		}
	}
	directory.mu.Unlock()
}

func (s *Server) hgRoomDirectory(videoID string) *hgRoomDirectoryShard {
	return &s.roomShards[hgHashVideoID(videoID)&uint32(len(s.roomShards)-1)]
}

func hgHashVideoID(videoID string) uint32 {
	var hash uint32 = 2166136261
	for index := 0; index < len(videoID); index++ {
		hash = (hash ^ uint32(videoID[index])) * 16777619
	}
	return hash
}

// hgAllowCommand 是连接所属 event-loop 内的无锁令牌桶，在进入共享 worker 队列和数据库前削掉突刺。
func (s *Server) hgAllowCommand(state *hgConnection, now time.Time) bool {
	nowNano := now.UnixNano()
	if state.commandRefillNano == 0 {
		state.commandTokens = float64(s.config.CommandBurst)
		state.commandRefillNano = nowNano
	}
	elapsed := float64(nowNano-state.commandRefillNano) / float64(time.Second)
	state.commandTokens = min(float64(s.config.CommandBurst), state.commandTokens+elapsed*float64(s.config.CommandRatePerSecond))
	state.commandRefillNano = nowNano
	if state.commandTokens < 1 {
		return false
	}
	state.commandTokens--
	return true
}
func (s *Server) hgAllowedOrigin(origin string) bool {
	for _, allowed := range s.config.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

// hgWriteFrame 对所有 Upgrade 后的 WebSocket 帧执行同一出站预算。预占发生在进入 gnet
// 异步队列之前，回调释放实际帧字节；这样 ACK、Error、Pong 和心跳不能绕过广播的慢连接隔离。
func (s *Server) hgWriteFrame(c gnet.Conn, state *hgConnection, frame []byte) error {
	if c == nil || state == nil {
		return errors.New("danmaku connection state cannot be nil")
	}
	frameBytes := int64(len(frame))
	if !state.hgReservePendingWrite(frameBytes, int64(s.config.MaxPendingBytes)) {
		s.metrics.outboundFailures[hgOutboundFailurePendingBudget].Add(1)
		return hgErrPendingWriteLimit
	}
	s.metrics.outboundPendingBytes.Add(frameBytes)
	err := c.AsyncWrite(frame, func(conn gnet.Conn, writeErr error) error {
		state.hgReleasePendingWrite(frameBytes)
		s.metrics.outboundPendingBytes.Add(-frameBytes)
		if writeErr != nil {
			s.metrics.outboundFailures[hgOutboundFailureCallback].Add(1)
			_ = conn.Close()
		}
		return nil
	})
	if err != nil {
		state.hgReleasePendingWrite(frameBytes)
		s.metrics.outboundPendingBytes.Add(-frameBytes)
		s.metrics.outboundFailures[hgOutboundFailureAsyncWrite].Add(1)
	}
	return err
}

func (s *Server) hgWriteDataFrame(c gnet.Conn, state *hgConnection, frame []byte) error {
	if state == nil || state.draining.Load() {
		return hgErrConnectionDraining
	}
	return s.hgWriteFrame(c, state, frame)
}

func (s *Server) hgCountWebSocketConnection(state *hgConnection) {
	if state != nil && state.websocketCounted.CompareAndSwap(false, true) {
		s.metrics.websocketConnections.Add(1)
	}
}

// hgActivateWebSocket 在连接所属 event-loop 内完成 Upgrade 后状态激活。
// closed 的原子检查用于拦截已经执行过 OnClose 的迟到 runnable；若 runnable 先执行，后续 OnClose 会正常清理。
func (s *Server) hgActivateWebSocket(videoID, userID string, c gnet.Conn, state *hgConnection) bool {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	if state == nil || state.closed.Load() || hgLifecycleState(s.lifecycle.Load()) != hgLifecycleServing {
		if hgLifecycleState(s.lifecycle.Load()) == hgLifecycleDraining {
			s.metrics.lateHandshakeRejections.Add(1)
		}
		return false
	}
	state.pending, state.upgraded, state.videoID, state.userID = false, true, videoID, userID
	s.hgCountWebSocketConnection(state)
	s.hgJoin(videoID, c, state)
	s.heartbeatWheel.hgRegister(state, c)
	return true
}

func (s *Server) hgObserveDrainDuration() {
	startedAt := s.drainStartedAt.Load()
	if startedAt == 0 || !s.drainObserved.CompareAndSwap(false, true) {
		return
	}
	elapsed := time.Since(time.Unix(0, startedAt))
	if elapsed < 0 {
		elapsed = 0
	}
	s.metrics.drainDurationNanos.Store(uint64(elapsed))
}

func (s *Server) hgReleaseWebSocketConnection(state *hgConnection) {
	if state != nil && state.websocketCounted.CompareAndSwap(true, false) {
		s.metrics.websocketConnections.Add(-1)
	}
}

func (state *hgConnection) hgReservePendingWrite(frameBytes, maxPendingBytes int64) bool {
	if frameBytes <= 0 || maxPendingBytes <= 0 {
		return false
	}
	for {
		pendingBytes := state.pendingBytes.Load()
		if frameBytes > maxPendingBytes-pendingBytes {
			return false
		}
		if state.pendingBytes.CompareAndSwap(pendingBytes, pendingBytes+frameBytes) {
			return true
		}
	}
}

func (state *hgConnection) hgReleasePendingWrite(frameBytes int64) {
	state.pendingBytes.Add(-frameBytes)
}

func (s *Server) hgError(c gnet.Conn, state *hgConnection, code, message string) error {
	payload, _ := json.Marshal(hgCommandErrorEnvelope{Type: "error", Data: hgCommandErrorData{Code: code, Message: message}})
	frame, _ := ws.CompileFrame(ws.NewTextFrame(payload))
	if err := s.hgWriteDataFrame(c, state, frame); err != nil {
		_ = c.Close()
		return err
	}
	return nil
}

func (s *Server) hgCommandAck(c gnet.Conn, state *hgConnection, requestID string, item VideoDanmakuDtoPackage.DanmakuResponse) error {
	payload := hgCommandAckPayload(requestID, item)
	frame, _ := ws.CompileFrame(ws.NewTextFrame(payload))
	if err := s.hgWriteDataFrame(c, state, frame); err != nil {
		_ = c.Close()
		return err
	}
	return nil
}

// hgCommandError 只表示某一条创建命令已被服务端明确拒绝；携带 requestID 后，客户端应结束该请求，
// 不能再自动回退 HTTP。只有断线、未连接或 ack 超时这类结果未知场景才允许以相同 requestID 重试。
func (s *Server) hgCommandError(c gnet.Conn, state *hgConnection, requestID, code, message string) error {
	payload := hgCommandErrorPayload(requestID, code, message)
	frame, _ := ws.CompileFrame(ws.NewTextFrame(payload))
	if err := s.hgWriteDataFrame(c, state, frame); err != nil {
		_ = c.Close()
		return err
	}
	return nil
}

// hgCommandAckPayload 返回发送连接专属确认；房间广播仍使用 danmaku.created，二者职责不能合并。
func hgCommandAckPayload(requestID string, item VideoDanmakuDtoPackage.DanmakuResponse) []byte {
	payload, _ := json.Marshal(hgCommandAckEnvelope{Type: "danmaku.ack", RequestID: requestID, Data: item})
	return payload
}

// hgCommandErrorPayload 保留稳定公开错误和请求关联，不向浏览器暴露数据库或 Redis 内部错误。
func hgCommandErrorPayload(requestID, code, message string) []byte {
	payload, _ := json.Marshal(hgCommandErrorEnvelope{Type: "error", RequestID: requestID, Data: hgCommandErrorData{Code: code, Message: message}})
	return payload
}

// hgHandshakeMetadata 只解析已经完整收齐且大小受限的 HTTP 请求，不读取 socket，也不产生外部 I/O。
func hgHandshakeMetadata(request []byte) (*url.URL, string, error) {
	lines := bytes.Split(request, []byte("\r\n"))
	if len(lines) < 2 {
		return nil, "", errors.New("malformed websocket handshake")
	}
	parts := strings.Split(string(lines[0]), " ")
	if len(parts) != 3 || parts[0] != "GET" || parts[2] != "HTTP/1.1" {
		return nil, "", errors.New("invalid websocket request line")
	}
	parsed, err := url.ParseRequestURI(parts[1])
	if err != nil {
		return nil, "", err
	}
	var origin string
	for _, line := range lines[1:] {
		pair := bytes.SplitN(line, []byte(":"), 2)
		if len(pair) == 2 && strings.EqualFold(strings.TrimSpace(string(pair[0])), "Origin") {
			origin = strings.TrimRight(strings.TrimSpace(string(pair[1])), "/")
		}
	}
	return parsed, origin, nil
}
