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

// hgConnection 只在所属 event-loop 内修改。pending 避免票据校验期间重复解析同一握手；
// handshake 保存完整且有硬上限的请求，Redis 校验完成后再回到 event-loop 生成 101 响应。
type hgConnection struct {
	upgraded, pending bool
	videoID, userID   string
	handshake         []byte
	memberShard       uint32
	commandTokens     float64
	commandRefillNano int64
	// pendingBytes 是跨 goroutine 使用的出站预算。gnet 的 WriteBufferCap 超过后会转入弹性缓冲，
	// 所以必须在应用层设置硬上限，避免慢客户端在热门房间中持续堆积广播直至 OOM。
	pendingBytes atomic.Int64
	lastActive   atomic.Int64
	counted      atomic.Bool
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
	userID    string
	requestID string
	request   VideoDanmakuDtoPackage.CreateRequest
}

// Server 是独立端口的 gnet WebSocket 弹幕网关；事件循环不执行 MySQL/Redis 阻塞 I/O。
type Server struct {
	gnet.BuiltinEventEngine
	service        *VideoDanmakuServicePackage.Service
	redis          *PersistenceRedisPackage.RedisService
	config         ConfigPackage.HGVideoDanmakuConfig
	engine         gnet.Engine
	ready          atomic.Bool
	connections    atomic.Int64
	queue          chan hgCommand
	broadcastQueue []chan VideoDanmakuDtoPackage.DanmakuResponse
	roomShards     []hgRoomDirectoryShard
	roomRouter     *hgRoomRouter
	workers        sync.WaitGroup
	background     sync.WaitGroup
	workerCtx      context.Context
	cancel         context.CancelFunc
	handshakeSlots chan struct{}
	heartbeatPing  []byte
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
	server.roomRouter = newHGRoomRouter(redis, config.BroadcastQueueSize, server.hgEnqueueBroadcast)
	return server
}

// Addr 返回 gnet 监听地址。
func (s *Server) Addr() string { return "tcp://" + s.config.Host + ":" + s.config.Port }

// Serve 启动 worker 后阻塞运行 gnet。
func (s *Server) Serve() error {
	for i := 0; i < s.config.WorkerCount; i++ {
		s.workers.Add(1)
		go s.hgWorker()
	}
	for index := range s.broadcastQueue {
		s.workers.Add(1)
		go s.hgBroadcastWorker(s.broadcastQueue[index])
	}
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		if err := s.roomRouter.Run(s.workerCtx); err != nil && s.workerCtx.Err() == nil {
			s.ready.Store(false)
		}
	}()
	s.background.Add(1)
	go s.hgHeartbeatLoop()
	return gnet.Run(s, s.Addr(), gnet.WithMulticore(true), gnet.WithReadBufferCap(s.config.MaxHandshakeBytes), gnet.WithWriteBufferCap(16<<10), gnet.WithTCPKeepAlive(90*time.Second))
}

// Ready 表示 gnet 已完成 OnBoot 且未进入关闭状态。
func (s *Server) Ready() error {
	if s == nil || !s.ready.Load() {
		return errors.New("danmaku realtime not ready")
	}
	if err := s.roomRouter.Ready(); err != nil {
		return err
	}
	return nil
}

// Close 停止接入、关闭现存连接并等待有界 worker 退出。
func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.ready.Store(false)
	conns := make([]gnet.Conn, 0, int(s.connections.Load()))
	for index := range s.roomShards {
		directory := &s.roomShards[index]
		directory.mu.RLock()
		for memberIndex := range directory.members {
			members := &directory.members[memberIndex]
			members.mu.RLock()
			for _, roomMembers := range members.members {
				for conn := range roomMembers {
					conns = append(conns, conn)
				}
			}
			members.mu.RUnlock()
		}
		directory.mu.RUnlock()
	}
	for _, conn := range conns {
		_ = conn.AsyncWrite(ws.CompiledCloseGoingAway, nil)
		_ = conn.Close()
	}
	s.cancel()
	done := make(chan struct{})
	go func() { s.workers.Wait(); s.background.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := s.engine.Validate(); err == nil {
		return s.engine.Stop(ctx)
	}
	return nil
}

func (s *Server) OnBoot(engine gnet.Engine) gnet.Action {
	s.engine = engine
	s.ready.Store(true)
	return gnet.None
}
func (s *Server) OnShutdown(gnet.Engine) { s.ready.Store(false); s.cancel() }
func (s *Server) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	state := &hgConnection{}
	state.counted.Store(true)
	state.lastActive.Store(time.Now().UnixNano())
	c.SetContext(state)
	if s.connections.Add(1) > int64(s.config.MaxConnections) || !s.ready.Load() {
		return nil, gnet.Close
	}
	return nil, gnet.None
}
func (s *Server) OnClose(c gnet.Conn, _ error) gnet.Action {
	state, _ := c.Context().(*hgConnection)
	if state != nil && state.counted.CompareAndSwap(true, false) {
		s.connections.Add(-1)
	}
	if state != nil && state.videoID != "" {
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
		if ws.CheckHeader(header, ws.StateServerSide) != nil || !header.Fin || header.Length > int64(s.config.MaxFrameBytes) {
			_ = c.AsyncWrite(ws.CompiledCloseProtocolError, nil)
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
			_ = c.AsyncWrite(pong, nil)
		case ws.OpPong:
		case ws.OpClose:
			_ = c.AsyncWrite(ws.CompiledCloseNormalClosure, nil)
			return gnet.Close
		case ws.OpText:
			if !utf8.Valid(payload) {
				_ = c.AsyncWrite(ws.CompiledCloseInvalidFramePayloadData, nil)
				return gnet.Close
			}
			var command struct {
				Type    string                               `json:"type"`
				Request VideoDanmakuDtoPackage.CreateRequest `json:"data"`
			}
			if json.Unmarshal(payload, &command) != nil || command.Type != "danmaku.create" {
				s.hgError(c, "invalid_command", "弹幕命令无效")
				continue
			}
			if !s.hgAllowCommand(state, time.Now()) {
				s.hgCommandError(c, command.Request.RequestID, "rate_limited", "弹幕发送过于频繁，请稍后重试")
				continue
			}
			command.Request.VideoID = state.videoID
			select {
			case s.queue <- hgCommand{conn: c, userID: state.userID, requestID: command.Request.RequestID, request: command.Request}:
			default:
				s.hgCommandError(c, command.Request.RequestID, "busy", "弹幕服务繁忙，请稍后重试")
			}
		default:
			_ = c.AsyncWrite(ws.CompiledCloseUnsupportedData, nil)
			return gnet.Close
		}
	}
	return gnet.None
}

// hgHeartbeatLoop 使用单个固定 ticker 扫描已升级连接，避免百万连接对应百万 timer。
func (s *Server) hgHeartbeatLoop() {
	defer s.background.Done()
	interval := s.config.HeartbeatInterval
	if interval <= 0 {
		interval = 20 * time.Second
	}
	timeout := s.config.HeartbeatTimeout
	if timeout < 2*interval {
		timeout = 3 * interval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.workerCtx.Done():
			return
		case now := <-ticker.C:
			s.hgHeartbeatScan(now, timeout)
		}
	}
}

func (s *Server) hgHeartbeatScan(now time.Time, timeout time.Duration) {
	deadline := now.Add(-timeout).UnixNano()
	var stale []gnet.Conn
	for directoryIndex := range s.roomShards {
		directory := &s.roomShards[directoryIndex]
		directory.mu.RLock()
		for memberIndex := range directory.members {
			members := &directory.members[memberIndex]
			members.mu.RLock()
			for _, roomMembers := range members.members {
				for conn, state := range roomMembers {
					if state.lastActive.Load() <= deadline {
						stale = append(stale, conn)
						continue
					}
					if err := conn.AsyncWrite(s.heartbeatPing, nil); err != nil {
						stale = append(stale, conn)
					}
				}
			}
			members.mu.RUnlock()
		}
		directory.mu.RUnlock()
	}
	for _, conn := range stale {
		_ = conn.Close()
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
			if len(output.Bytes()) > 0 {
				_, _ = c.Write(output.Bytes())
			}
			if upgradeErr != nil {
				s.roomRouter.Leave(binding.VideoID)
				return c.EventLoop().Close(c)
			}
			state.pending, state.upgraded, state.videoID, state.userID = false, true, binding.VideoID, binding.UserID
			s.hgJoin(binding.VideoID, c, state)
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
				// 未知错误只返回稳定文案，SQL driver 和事务上下文保留在服务端，不能泄露给客户端。
				s.hgCommandError(command.conn, command.requestID, "create_failed", "弹幕发送失败，请稍后重试")
				continue
			}
			// 广播面向整个房间，ack 只返回给发送连接。requestId 让浏览器能够在并发发送时
			// 精确完成对应 Promise；若 ack 丢失，客户端可用同一 requestId 通过 HTTP 幂等回退。
			s.hgCommandAck(command.conn, command.requestID, item)
		}
	}
}

// Publish 将已提交弹幕发布到 Redis，使不同实例上的本地房间都能收到；MySQL 历史仍是权威恢复来源。
func (s *Server) Publish(ctx context.Context, item VideoDanmakuDtoPackage.DanmakuResponse) error {
	return s.roomRouter.Publish(ctx, item)
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
	frameBytes := int64(len(frame))
	var slowConnections []gnet.Conn
	for index := range directory.members {
		members := &directory.members[index]
		members.mu.RLock()
		for conn, state := range members.members[item.VideoID] {
			if state.pendingBytes.Add(frameBytes) > int64(s.config.MaxPendingBytes) {
				state.pendingBytes.Add(-frameBytes)
				slowConnections = append(slowConnections, conn)
				continue
			}
			if err = conn.AsyncWrite(frame, func(_ gnet.Conn, _ error) error { state.pendingBytes.Add(-frameBytes); return nil }); err != nil {
				state.pendingBytes.Add(-frameBytes)
				slowConnections = append(slowConnections, conn)
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
func (s *Server) hgError(c gnet.Conn, code, message string) {
	payload, _ := json.Marshal(hgCommandErrorEnvelope{Type: "error", Data: hgCommandErrorData{Code: code, Message: message}})
	frame, _ := ws.CompileFrame(ws.NewTextFrame(payload))
	_ = c.AsyncWrite(frame, nil)
}

func (s *Server) hgCommandAck(c gnet.Conn, requestID string, item VideoDanmakuDtoPackage.DanmakuResponse) {
	payload := hgCommandAckPayload(requestID, item)
	frame, _ := ws.CompileFrame(ws.NewTextFrame(payload))
	_ = c.AsyncWrite(frame, nil)
}

// hgCommandError 只表示某一条创建命令已被服务端明确拒绝；携带 requestID 后，客户端应结束该请求，
// 不能再自动回退 HTTP。只有断线、未连接或 ack 超时这类结果未知场景才允许以相同 requestID 重试。
func (s *Server) hgCommandError(c gnet.Conn, requestID, code, message string) {
	payload := hgCommandErrorPayload(requestID, code, message)
	frame, _ := ws.CompileFrame(ws.NewTextFrame(payload))
	_ = c.AsyncWrite(frame, nil)
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
