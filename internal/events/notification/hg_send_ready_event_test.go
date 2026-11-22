package notification

import (
	"MLC_GO/internal/events"
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestHGSendReadyEventEnvelopeOnlyContainsTaskCursor(t *testing.T) {
	event, err := HGNewSendReadyEvent(context.Background(), 42, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.EventName != HGSendReadyEventName || envelope.EventKey != "42" || envelope.Version != events.EventVersionV1 {
		t.Fatalf("事件路由不正确: %+v", envelope)
	}
	var payload map[string]any
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 6 || payload["taskId"] != "42" || payload["nextIndex"] != float64(1) ||
		payload["generation"] != float64(3) || strings.Contains(string(envelope.Payload), "userIds") {
		t.Fatalf("事件只能引用权威任务游标: %s", envelope.Payload)
	}
}

func TestHGSendReadyEventRejectsInvalidCoordinates(t *testing.T) {
	for _, coordinates := range []struct {
		id         uint64
		index      uint16
		generation uint64
	}{
		{0, 0, 1}, {math.MaxUint64, 0, 1}, {1, 100, 1}, {1, 0, 0},
	} {
		if _, err := HGNewSendReadyEvent(context.Background(), coordinates.id, coordinates.index, coordinates.generation); err == nil {
			t.Fatalf("必须拒绝无效任务坐标: %+v", coordinates)
		}
	}
	if err := (HGSendReadyEvent{TaskID: "-1", Generation: 1}).HGValidate(); err == nil {
		t.Fatal("消费侧不得接受伪造任务 ID")
	}
	if err := (HGSendReadyEvent{TaskID: "042", Generation: 1}).HGValidate(); err == nil {
		t.Fatal("同一任务不能以不同 Kafka 分区键编码")
	}
}
