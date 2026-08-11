package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	ConfigPackage "MLC_GO/internal/pkg/config"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gobwas/ws"
)

func TestCommandFramesCorrelateRequestID(t *testing.T) {
	item := VideoDanmakuDtoPackage.DanmakuResponse{DanmakuID: "DMK_1", VideoID: "video-1"}
	ack := hgCommandAckPayload("request-1", item)
	failed := hgCommandErrorPayload("request-2", "busy", "service busy")

	var ackMessage struct {
		Type      string                                 `json:"type"`
		RequestID string                                 `json:"requestId"`
		Data      VideoDanmakuDtoPackage.DanmakuResponse `json:"data"`
	}
	if err := json.Unmarshal(ack, &ackMessage); err != nil || ackMessage.Type != "danmaku.ack" || ackMessage.RequestID != "request-1" || ackMessage.Data.DanmakuID != "DMK_1" {
		t.Fatalf("ack payload = %s, error = %v", ack, err)
	}

	var errorMessage struct {
		Type      string            `json:"type"`
		RequestID string            `json:"requestId"`
		Data      map[string]string `json:"data"`
	}
	if err := json.Unmarshal(failed, &errorMessage); err != nil || errorMessage.Type != "error" || errorMessage.RequestID != "request-2" || errorMessage.Data["code"] != "busy" {
		t.Fatalf("error payload = %s, error = %v", failed, err)
	}
}

func TestHandshakeMetadataParsesWebSocketOrigin(t *testing.T) {
	request := []byte("GET /api/v1/video_danmaku/ws?ticket=abc HTTP/1.1\r\nHost: localhost:5174\r\nOrigin: http://localhost:5174/\r\n\r\n")
	requestURL, origin, err := hgHandshakeMetadata(request)
	if err != nil || requestURL.Path != "/api/v1/video_danmaku/ws" || origin != "http://localhost:5174" {
		t.Fatalf("hgHandshakeMetadata() url=%v origin=%q error=%v", requestURL, origin, err)
	}
}

func TestCommandPayloadCompilesAsTextFrame(t *testing.T) {
	payload := hgCommandAckPayload("request-1", VideoDanmakuDtoPackage.DanmakuResponse{DanmakuID: "DMK_1"})
	frame, err := ws.CompileFrame(ws.NewTextFrame(payload))
	if err != nil || !bytes.Contains(frame, []byte("danmaku.ack")) {
		t.Fatalf("CompileFrame() frame=%q error=%v", frame, err)
	}
}

func TestRoomDirectoryDistributesVideoIDs(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 4, BroadcastQueueSize: 64})
	seen := make(map[*hgRoomDirectoryShard]struct{})
	for _, videoID := range []string{"video-1", "video-2", "video-3", "video-4", "video-5", "video-6"} {
		seen[server.hgRoomDirectory(videoID)] = struct{}{}
	}
	if len(seen) < 2 {
		t.Fatalf("video IDs used only %d room directory shard", len(seen))
	}
}

func TestBroadcastQueueUsesStableVideoShard(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 4, BroadcastQueueSize: 64})
	item := VideoDanmakuDtoPackage.DanmakuResponse{VideoID: "video-1"}
	server.hgEnqueueBroadcast(item)
	index := hgHashVideoID(item.VideoID) & uint32(len(server.broadcastQueue)-1)
	select {
	case received := <-server.broadcastQueue[index]:
		if received.VideoID != item.VideoID {
			t.Fatalf("received video = %q", received.VideoID)
		}
	default:
		t.Fatal("broadcast was not routed to stable shard")
	}
}

func TestCommandRateLimiterAllowsBurstAndRefill(t *testing.T) {
	server := &Server{config: ConfigPackage.HGVideoDanmakuConfig{CommandRatePerSecond: 2, CommandBurst: 3}}
	state := &hgConnection{}
	now := time.Unix(1_000, 0)
	for index := 0; index < 3; index++ {
		if !server.hgAllowCommand(state, now) {
			t.Fatalf("burst command %d was rejected", index)
		}
	}
	if server.hgAllowCommand(state, now) {
		t.Fatal("command above burst was accepted")
	}
	if !server.hgAllowCommand(state, now.Add(500*time.Millisecond)) {
		t.Fatal("one refilled token was not accepted")
	}
}

func TestFragmentedTextMessageReassemblesWithinLimit(t *testing.T) {
	state := &hgConnection{}
	first := ws.Header{Fin: false, OpCode: ws.OpText}
	last := ws.Header{Fin: true, OpCode: ws.OpContinuation}

	message, err := state.hgAcceptDataFrame(first, []byte(`{"type":"danmaku.`), 64)
	if err != nil || message != nil || !state.hgWebSocketState().Fragmented() {
		t.Fatalf("first fragment message=%q fragmented=%t error=%v", message, state.hgWebSocketState().Fragmented(), err)
	}
	message, err = state.hgAcceptDataFrame(last, []byte(`create"}`), 64)
	if err != nil || string(message) != `{"type":"danmaku.create"}` {
		t.Fatalf("reassembled message=%q error=%v", message, err)
	}
	if state.hgWebSocketState().Fragmented() || state.fragmentPayload != nil {
		t.Fatal("fragment state was not released after final continuation")
	}
}

func TestFragmentedTextMessageRejectsAggregateOverflow(t *testing.T) {
	state := &hgConnection{}
	_, err := state.hgAcceptDataFrame(ws.Header{Fin: false, OpCode: ws.OpText}, []byte("123456"), 8)
	if err != nil {
		t.Fatalf("first fragment error = %v", err)
	}
	_, err = state.hgAcceptDataFrame(ws.Header{Fin: true, OpCode: ws.OpContinuation}, []byte("789"), 8)
	if !errors.Is(err, hgErrFragmentMessageTooBig) {
		t.Fatalf("overflow error = %v, want %v", err, hgErrFragmentMessageTooBig)
	}
	if state.fragmentOpCode != 0 || state.fragmentPayload != nil {
		t.Fatal("overflowed fragment state was retained")
	}
}

func TestFragmentedMessageHeaderStateAllowsControlFrames(t *testing.T) {
	state := &hgConnection{}
	_, err := state.hgAcceptDataFrame(ws.Header{Fin: false, OpCode: ws.OpText}, []byte("part"), 16)
	if err != nil {
		t.Fatalf("first fragment error = %v", err)
	}
	webSocketState := state.hgWebSocketState()
	if err = ws.CheckHeader(ws.Header{Fin: true, OpCode: ws.OpPing, Masked: true}, webSocketState); err != nil {
		t.Fatalf("ping interleaved with fragments was rejected: %v", err)
	}
	if err = ws.CheckHeader(ws.Header{Fin: true, OpCode: ws.OpText, Masked: true}, webSocketState); !errors.Is(err, ws.ErrProtocolContinuationExpected) {
		t.Fatalf("new text frame error = %v, want %v", err, ws.ErrProtocolContinuationExpected)
	}
}

func TestBinaryDataFrameRemainsUnsupported(t *testing.T) {
	state := &hgConnection{}
	_, err := state.hgAcceptDataFrame(ws.Header{Fin: true, OpCode: ws.OpBinary}, []byte("binary"), 16)
	if !errors.Is(err, hgErrUnsupportedData) {
		t.Fatalf("binary frame error = %v, want %v", err, hgErrUnsupportedData)
	}
}

func TestRoomRouterRejoinInvalidatesQueuedUnsubscribe(t *testing.T) {
	router := &hgRoomRouter{refs: map[string]*hgRoomSubscriptionState{
		"video-1": {refs: 0, generation: 2, subscribed: true},
	}}
	command := hgRoomSubscriptionCommand{videoID: "video-1", generation: 2}

	router.refsMu.Lock()
	state := router.refs["video-1"]
	state.generation++
	state.refs++
	router.refsMu.Unlock()

	if router.hgBeginUnsubscribe(command) {
		t.Fatal("stale unsubscribe remained valid after room rejoin")
	}
}

func TestRoomRouterMarksUnsubscribeBeforeRedisIO(t *testing.T) {
	router := &hgRoomRouter{refs: map[string]*hgRoomSubscriptionState{
		"video-1": {refs: 0, generation: 2, subscribed: true},
	}}
	command := hgRoomSubscriptionCommand{videoID: "video-1", generation: 2}

	if !router.hgBeginUnsubscribe(command) {
		t.Fatal("valid unsubscribe was rejected")
	}
	router.refsMu.Lock()
	state := router.refs["video-1"]
	unsubscribing := state != nil && state.unsubscribing && state.ready != nil
	router.refsMu.Unlock()
	if !unsubscribing {
		t.Fatal("room was not marked unsubscribing before Redis I/O")
	}

	router.hgFinishUnsubscribe(command, nil)
	router.refsMu.Lock()
	_, exists := router.refs["video-1"]
	router.refsMu.Unlock()
	if exists {
		t.Fatal("unused room remained after successful unsubscribe")
	}
}

func TestRoomRouterUnsubscribeQueueCoalescesWithoutDroppingRooms(t *testing.T) {
	router := &hgRoomRouter{pending: make(map[string]hgRoomSubscriptionCommand), wake: make(chan struct{}, 1)}
	const roomCount = 1_000
	for index := 0; index < roomCount; index++ {
		videoID := fmt.Sprintf("video-%d", index)
		router.hgQueueUnsubscribe(hgRoomSubscriptionCommand{videoID: videoID, generation: 1})
	}
	router.hgQueueUnsubscribe(hgRoomSubscriptionCommand{videoID: "video-1", generation: 2})

	seen := make(map[string]uint64, roomCount)
	for len(seen) < roomCount {
		command, ok := router.hgPopUnsubscribe()
		if !ok {
			t.Fatalf("popped %d rooms, want %d", len(seen), roomCount)
		}
		seen[command.videoID] = command.generation
	}
	if seen["video-1"] != 2 {
		t.Fatalf("video-1 generation = %d, want latest generation 2", seen["video-1"])
	}
}

func TestConnectionCountIsReleasedOnce(t *testing.T) {
	server := &Server{}
	state := &hgConnection{}
	state.counted.Store(true)
	server.connections.Store(1)
	if state.counted.CompareAndSwap(true, false) {
		server.connections.Add(-1)
	}
	if state.counted.CompareAndSwap(true, false) {
		server.connections.Add(-1)
	}
	if got := server.connections.Load(); got != 0 {
		t.Fatalf("connections = %d, want 0", got)
	}
}

func TestPendingWriteBudgetRejectsOverflowAndRecovers(t *testing.T) {
	state := &hgConnection{}
	const maxPendingBytes = int64(100)

	if !state.hgReservePendingWrite(80, maxPendingBytes) {
		t.Fatal("first pending write reservation was rejected")
	}
	if state.hgReservePendingWrite(30, maxPendingBytes) {
		t.Fatal("overflow pending write reservation was accepted")
	}
	if pending := state.pendingBytes.Load(); pending != 80 {
		t.Fatalf("pending bytes after rejected reservation = %d, want 80", pending)
	}
	state.hgReleasePendingWrite(80)
	if pending := state.pendingBytes.Load(); pending != 0 {
		t.Fatalf("pending bytes after callback release = %d, want 0", pending)
	}
}

func TestPendingWriteBudgetIsAtomicAcrossWriters(t *testing.T) {
	state := &hgConnection{}
	const (
		writerCount     = 1_000
		frameBytes      = int64(64)
		maxPendingBytes = int64(640)
	)
	start := make(chan struct{})
	release := make(chan struct{})
	var accepted atomic.Int64
	var writers sync.WaitGroup
	writers.Add(writerCount)
	for index := 0; index < writerCount; index++ {
		go func() {
			defer writers.Done()
			<-start
			if !state.hgReservePendingWrite(frameBytes, maxPendingBytes) {
				return
			}
			accepted.Add(1)
			<-release
			state.hgReleasePendingWrite(frameBytes)
		}()
	}
	close(start)
	deadline := time.Now().Add(time.Second)
	for accepted.Load() < maxPendingBytes/frameBytes && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := accepted.Load(); got != maxPendingBytes/frameBytes {
		close(release)
		writers.Wait()
		t.Fatalf("accepted writes = %d, want %d", got, maxPendingBytes/frameBytes)
	}
	close(release)
	writers.Wait()
	if pending := state.pendingBytes.Load(); pending != 0 {
		t.Fatalf("pending bytes after concurrent releases = %d, want 0", pending)
	}
}

func BenchmarkCommandAckPayload(b *testing.B) {
	item := VideoDanmakuDtoPackage.DanmakuResponse{DanmakuID: "DMK_1", VideoID: "video-1", Content: "benchmark danmaku", ProgressMS: 12_345, Mode: "scroll", Color: "#FFFFFF", FontSize: 25}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		_ = hgCommandAckPayload("request-1", item)
	}
}
