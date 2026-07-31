package VideoInteractionServicePackage

import (
	"MLC_GO/internal/events"
	VideoInteractionDtoPackage "MLC_GO/internal/modules/video_interaction/dto"
	"context"
	"errors"
	"testing"
)

type hgFakeEventBus struct {
	event events.DomainEvent
	err   error
}

func (f *hgFakeEventBus) Publish(_ context.Context, event events.DomainEvent) error {
	f.event = event
	return f.err
}

type hgFakeCache struct {
	state VideoInteractionDtoPackage.StateResponse
}

func (f *hgFakeCache) GetState(context.Context, string, string, string) (VideoInteractionDtoPackage.StateResponse, error) {
	return f.state, nil
}

func (f *hgFakeCache) ApplyOptimistic(context.Context, string, string, string, string, bool, int) error {
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
	service := NewService(&hgFakeEventBus{}, &hgFakeCache{})

	_, err := service.SetVideoInteraction(context.Background(), "user-1", VideoInteractionDtoPackage.ActionRequest{
		SubmissionID: "submission-1",
		Action:       "coin",
		Quantity:     3,
	})
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("error = %v, want ErrInvalidQuantity", err)
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
