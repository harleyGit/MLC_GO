package im

import (
	"encoding/json"
	"testing"
	"time"
)

// 历史消息对新客户端给出 sentAt，保留 createdAt 供旧客户端过渡；二者均为同一 UTC 时刻。
func TestHGHistoryMessageTimeJSON(t *testing.T) {
	stamp := time.Date(2026, 11, 6, 10, 30, 0, 123000000, time.UTC)
	message := HGHistoryMessage{MessageID: "10001", Seq: 7, CreatedAt: stamp, SentAt: stamp}
	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"createdAt", "sentAt"} {
		if fields[field] != "2026-11-06T10:30:00.123Z" {
			t.Errorf("%s 非预期 UTC 毫秒时间: %v", field, fields[field])
		}
	}
}
