package VideoInteractionServicePackage

import (
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	VideoInteractionDtoPackage "MLC_GO/internal/modules/video_interaction/dto"
	"context"
	"errors"
	"testing"
)

type hgFakeCoinStore struct {
	calls     int
	event     InteractionEventsPackage.VideoInteractionChangedEvent
	err       error
	committed bool
}

func (f *hgFakeCoinStore) SubmitCoin(_ context.Context, requestID string, event InteractionEventsPackage.VideoInteractionChangedEvent) (bool, error) {
	f.calls++
	f.event = event
	if requestID == "" {
		return false, errors.New("request id missing")
	}
	if f.err != nil {
		return false, f.err
	}
	if !f.committed {
		return false, nil
	}
	return true, nil
}

type hgFakeEventBus struct {
	event events.DomainEvent
	err   error
}

func (f *hgFakeEventBus) Publish(_ context.Context, event events.DomainEvent) error {
	f.event = event
	return f.err
}

type hgFakeCache struct {
	state      VideoInteractionDtoPackage.StateResponse
	applyCalls int
}

func (f *hgFakeCache) GetState(context.Context, string, string, string) (VideoInteractionDtoPackage.StateResponse, error) {
	return f.state, nil
}

func (f *hgFakeCache) ApplyOptimistic(context.Context, string, string, string, string, bool, int) error {
	f.applyCalls++
	return nil
}

func TestSetVideoInteractionPublishesStableEvent(t *testing.T) {
	bus := &hgFakeEventBus{}
	service := NewService(bus, &hgFakeCache{})

	response, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1",
		Action:       "like",
		Active:       true,
	})
	if err != nil {
		t.Fatalf("SetVideoInteraction() error = %v", err)
	}
	if response.Accepted != true || response.Action != "like" || !response.Active {
		t.Fatalf("response = %#v", response)
	}
	if bus.event == nil || bus.event.EventName() != "video.interaction.changed" {
		t.Fatalf("event = %#v, want video.interaction.changed", bus.event)
	}
	if bus.event.EventKey() != "submission-1:user-1:like" {
		t.Fatalf("event key = %q", bus.event.EventKey())
	}
}

func TestCoinRejectsQuantityOutsideOneOrTwo(t *testing.T) {
	service := NewService(&hgFakeEventBus{}, &hgFakeCache{}, &hgFakeCoinStore{committed: true})

	_, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1",
		Action:       "coin",
		Quantity:     3,
	})
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("error = %v, want ErrInvalidQuantity", err)
	}
}

func TestCoinRequiresIdempotencyRequestID(t *testing.T) {
	service := NewService(&hgFakeEventBus{}, &hgFakeCache{}, &hgFakeCoinStore{committed: true})

	_, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1",
		Action:       "coin",
		Quantity:     1,
	})
	if !errors.Is(err, ErrInvalidRequestID) {
		t.Fatalf("error = %v, want ErrInvalidRequestID", err)
	}
}

func TestCoinUsesTransactionalStoreInsteadOfDirectKafka(t *testing.T) {
	bus := &hgFakeEventBus{}
	store := &hgFakeCoinStore{committed: true}
	service := NewService(bus, &hgFakeCache{}, store)

	response, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1",
		RequestID:    "request-1",
		Action:       "coin",
		Quantity:     2,
	})
	if err != nil {
		t.Fatalf("SetVideoInteraction() error = %v", err)
	}
	if !response.Accepted || store.calls != 1 || store.event.Quantity != 2 {
		t.Fatalf("response=%#v store=%#v", response, store)
	}
	if bus.event != nil {
		t.Fatalf("coin must use transactional outbox, direct event = %#v", bus.event)
	}
}

func TestCoinIdempotentReplayDoesNotApplyRedisTwice(t *testing.T) {
	cache := &hgFakeCache{}
	store := &hgFakeCoinStore{committed: false}
	service := NewService(&hgFakeEventBus{}, cache, store)

	_, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1", RequestID: "request-1", Action: "coin", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("SetVideoInteraction() error = %v", err)
	}
	if cache.applyCalls != 0 {
		t.Fatalf("optimistic Redis calls = %d, want 0 for replay", cache.applyCalls)
	}
}

func TestFollowRejectsFollowingSelf(t *testing.T) {
	service := NewService(&hgFakeEventBus{}, &hgFakeCache{})

	_, err := service.SetFollow(context.Background(), "user-1", VideoInteractionDtoPackage.FollowRequest{
		FolloweeID: "user-1",
		Active:     true,
	})
	if !errors.Is(err, ErrCannotFollowSelf) {
		t.Fatalf("error = %v, want ErrCannotFollowSelf", err)
	}
}

func TestPublishFailureDoesNotReportAccepted(t *testing.T) {
	service := NewService(&hgFakeEventBus{err: errors.New("kafka unavailable")}, &hgFakeCache{})

	_, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1",
		Action:       "favorite",
		Active:       true,
	})
	if err == nil {
		t.Fatal("expected publish error")
	}
}
