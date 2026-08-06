package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	ConfigPackage "MLC_GO/internal/pkg/config"
	"bytes"
	"encoding/json"
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

	if router.hgShouldUnsubscribe(command) {
		t.Fatal("stale unsubscribe remained valid after room rejoin")
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

func BenchmarkCommandAckPayload(b *testing.B) {
	item := VideoDanmakuDtoPackage.DanmakuResponse{DanmakuID: "DMK_1", VideoID: "video-1", Content: "benchmark danmaku", ProgressMS: 12_345, Mode: "scroll", Color: "#FFFFFF", FontSize: 25}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		_ = hgCommandAckPayload("request-1", item)
	}
}
