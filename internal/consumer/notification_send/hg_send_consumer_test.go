package notification_send

import (
	"MLC_GO/internal/events"
	notificationevents "MLC_GO/internal/events/notification"
	hgkafka "MLC_GO/internal/pkg/kafka"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type hgReadyRecorder struct {
	called int
	err    error
}

func (recorder *hgReadyRecorder) HGDispatchReady(_ context.Context, _ notificationevents.HGSendReadyEvent) error {
	recorder.called++
	return recorder.err
}

func TestHGConsumerRejectsMalformedAndMismatchedEvents(t *testing.T) {
	event, err := notificationevents.HGNewSendReadyEvent(context.Background(), 42, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, modify := range []func(*events.EventEnvelope){
		func(e *events.EventEnvelope) { e.EventKey = "43" },
		func(e *events.EventEnvelope) { e.EventName = "user.follow.committed" },
		func(e *events.EventEnvelope) {
			e.Payload = json.RawMessage(`{"version":1,"taskId":"42","nextIndex":0,"generation":1,"receiver":"private"}`)
		},
		func(e *events.EventEnvelope) {
			e.Payload = json.RawMessage(`{"version":1,"taskId":"42","nextIndex":0,"generation":0}`)
		},
	} {
		recorder := new(hgReadyRecorder)
		message := envelope
		modify(&message)
		if err := HGNewConsumer(recorder).Handle(context.Background(), message); !hgkafka.HGIsTerminalError(err) || recorder.called != 0 {
			t.Fatalf("恶意或不一致事件不得访问任务: %v, calls=%d", err, recorder.called)
		}
	}
}

func TestHGConsumerOnlyCommitsAfterWorkerAck(t *testing.T) {
	event, err := notificationevents.HGNewSendReadyEvent(context.Background(), 42, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	expected := errors.New("数据库回执失败")
	recorder := &hgReadyRecorder{err: expected}
	if err := HGNewConsumer(recorder).Handle(context.Background(), envelope); !errors.Is(err, expected) || recorder.called != 1 {
		t.Fatalf("消费失败不得提交 offset: %v, calls=%d", err, recorder.called)
	}
}
