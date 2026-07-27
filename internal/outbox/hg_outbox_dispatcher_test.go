package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type hgDispatcherRepositoryStub struct {
	events       []Event
	claimErr     error
	publishedIDs []int64
	retryIDs     []int64
	deadIDs      []int64
	mu           sync.Mutex
}

func (s *hgDispatcherRepositoryStub) Claim(_ context.Context, _ int, _ time.Duration) ([]Event, error) {
	return s.events, s.claimErr
}

func (s *hgDispatcherRepositoryStub) MarkPublished(_ context.Context, id int64, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publishedIDs = append(s.publishedIDs, id)
	return true, nil
}

func (s *hgDispatcherRepositoryStub) MarkRetry(_ context.Context, id int64, _ string, _ string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retryIDs = append(s.retryIDs, id)
	return true, nil
}

func (s *hgDispatcherRepositoryStub) MarkDead(_ context.Context, id int64, _ string, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadIDs = append(s.deadIDs, id)
	return true, nil
}

type hgProducerStub struct{ err error }

func (s hgProducerStub) Send(context.Context, string, string, []byte) error { return s.err }

func TestDispatcherClaimsThenPublishesOutsideClaimTransaction(t *testing.T) {
	repo := &hgDispatcherRepositoryStub{events: []Event{{ID: 1, LeaseToken: "lease-1", Topic: "events", EventKey: "key", Payload: []byte("payload")}}}
	dispatcher := NewDispatcherWithRepository(repo, hgProducerStub{})

	if err := dispatcher.DispatchOnce(context.Background(), 8); err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if len(repo.publishedIDs) != 1 || repo.publishedIDs[0] != 1 {
		t.Fatalf("published IDs = %v, want [1]", repo.publishedIDs)
	}
}

func TestDispatcherRetriesFailedSendWithSameLease(t *testing.T) {
	repo := &hgDispatcherRepositoryStub{events: []Event{{ID: 2, LeaseToken: "lease-2", RetryCount: 1, Topic: "events"}}}
	dispatcher := NewDispatcherWithRepository(repo, hgProducerStub{err: errors.New("kafka unavailable")})

	if err := dispatcher.DispatchOnce(context.Background(), 8); err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if len(repo.retryIDs) != 1 || repo.retryIDs[0] != 2 {
		t.Fatalf("retry IDs = %v, want [2]", repo.retryIDs)
	}
}
