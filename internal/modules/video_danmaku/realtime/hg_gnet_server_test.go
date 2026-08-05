package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	"bytes"
	"encoding/json"
	"testing"

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
