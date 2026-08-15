package VideoRecommendServicePackage

import (
	VideoRecommendDtoPackage "MLC_GO/internal/modules/video_recommend/dto"
	VideoRecommendModelPackage "MLC_GO/internal/modules/video_recommend/model"
	"context"
	"errors"
	"testing"
	"time"
)

type hgTestCache struct {
	candidates []VideoRecommendModelPackage.HGCandidate
	cards      map[string]VideoRecommendDtoPackage.HGFeedItem
	err        error
}

func (c *hgTestCache) ListCandidates(context.Context, VideoRecommendModelPackage.HGCursor, int) ([]VideoRecommendModelPackage.HGCandidate, error) {
	return c.candidates, c.err
}
func (c *hgTestCache) GetCards(context.Context, []string) (map[string]VideoRecommendDtoPackage.HGFeedItem, []string, error) {
	return c.cards, nil, c.err
}
func (c *hgTestCache) SetCards(context.Context, map[string]VideoRecommendDtoPackage.HGFeedItem) error {
	return nil
}
func (c *hgTestCache) FillInteractionCounts(context.Context, []VideoRecommendDtoPackage.HGFeedItem) error {
	return c.err
}

type hgTestRepository struct{}

func (hgTestRepository) BatchGetPublicVideoCards(context.Context, []string) (map[string]VideoRecommendDtoPackage.HGFeedItem, error) {
	return map[string]VideoRecommendDtoPackage.HGFeedItem{}, nil
}

func TestServiceFeedUsesStableCursorAndReranksDiversity(t *testing.T) {
	published := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	cache := &hgTestCache{
		candidates: []VideoRecommendModelPackage.HGCandidate{{SubmissionID: "s3", Score: 300}, {SubmissionID: "s2", Score: 200}, {SubmissionID: "s1", Score: 100}, {SubmissionID: "s0", Score: 50}},
		cards: map[string]VideoRecommendDtoPackage.HGFeedItem{
			"s3": {SubmissionID: "s3", AuthorID: "a1", Category: "tech", PublishTime: published},
			"s2": {SubmissionID: "s2", AuthorID: "a1", Category: "tech", PublishTime: published},
			"s1": {SubmissionID: "s1", AuthorID: "a2", Category: "life", PublishTime: published},
			"s0": {SubmissionID: "s0", AuthorID: "a3", Category: "music", PublishTime: published},
		},
	}
	service := NewService(cache, hgTestRepository{}, "v2")
	response, err := service.Feed(context.Background(), "u1", "", 3)
	if err != nil {
		t.Fatalf("Feed() error = %v", err)
	}
	if len(response.Items) != 3 || response.Items[0].SubmissionID != "s3" || response.Items[1].SubmissionID != "s1" || response.Items[2].SubmissionID != "s2" {
		t.Fatalf("unexpected rerank: %#v", response.Items)
	}
	if response.NextCursor == "" || !response.HasMore || response.Items[0].Reason != "新鲜发布" {
		t.Fatalf("unexpected response: %#v", response)
	}
	cursor, err := service.decodeCursor(response.NextCursor)
	if err != nil || cursor.Score != 100 || cursor.SubmissionID != "s1" {
		t.Fatalf("cursor = %#v, err = %v", cursor, err)
	}
}

func TestServiceFeedRejectsInvalidInputAndFailsFast(t *testing.T) {
	service := NewService(&hgTestCache{}, hgTestRepository{}, "v2")
	if _, err := service.Feed(context.Background(), "u1", "invalid", 20); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid cursor error = %v", err)
	}
	if _, err := service.Feed(context.Background(), "u1", "", 51); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid page size error = %v", err)
	}
	service = NewService(&hgTestCache{err: errors.New("redis down")}, hgTestRepository{}, "v2")
	if _, err := service.Feed(context.Background(), "u1", "", 20); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("dependency error = %v", err)
	}
}

func TestServiceFeedAdvancesPastStaleCandidates(t *testing.T) {
	cache := &hgTestCache{candidates: []VideoRecommendModelPackage.HGCandidate{{SubmissionID: "deleted", Score: 300}}}
	service := NewService(cache, hgTestRepository{}, "v2")
	response, err := service.Feed(context.Background(), "u1", "", 20)
	if err != nil {
		t.Fatalf("Feed() error = %v", err)
	}
	if len(response.Items) != 0 || response.NextCursor == "" || response.HasMore {
		t.Fatalf("stale candidate response = %#v", response)
	}
	cursor, err := service.decodeCursor(response.NextCursor)
	if err != nil || cursor.SubmissionID != "deleted" || cursor.Score != 300 {
		t.Fatalf("cursor = %#v, err = %v", cursor, err)
	}
}
