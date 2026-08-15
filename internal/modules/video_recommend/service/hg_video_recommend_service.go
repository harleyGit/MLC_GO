package VideoRecommendServicePackage

import (
	VideoRecommendDtoPackage "MLC_GO/internal/modules/video_recommend/dto"
	VideoRecommendModelPackage "MLC_GO/internal/modules/video_recommend/model"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// 分页和候选倍率共同限定单请求的 Redis、MySQL、CPU 与内存成本。
	hgDefaultPageSize     = 20
	hgMaxPageSize         = 50
	hgCandidateMultiplier = 3
	hgRecommendTimeout    = 250 * time.Millisecond
)

var (
	// ErrInvalidRequest 表示 pageSize 或游标不符合推荐 API 约束。
	ErrInvalidRequest = errors.New("invalid video recommend request")
	// ErrUnavailable 表示推荐依赖不可用，调用方应快速失败而不是回源扫描主表。
	ErrUnavailable = errors.New("video recommend unavailable")
)

type hgCandidateCache interface {
	ListCandidates(context.Context, VideoRecommendModelPackage.HGCursor, int) ([]VideoRecommendModelPackage.HGCandidate, error)
	GetCards(context.Context, []string) (map[string]VideoRecommendDtoPackage.HGFeedItem, []string, error)
	SetCards(context.Context, map[string]VideoRecommendDtoPackage.HGFeedItem) error
	FillInteractionCounts(context.Context, []VideoRecommendDtoPackage.HGFeedItem) error
}

type hgCardRepository interface {
	BatchGetPublicVideoCards(context.Context, []string) (map[string]VideoRecommendDtoPackage.HGFeedItem, error)
}

// Service 编排分片召回、批量卡片补全、互动特征和轻量重排。
type Service struct {
	cache      hgCandidateCache
	repository hgCardRepository
	generation string
	now        func() time.Time
}

// NewService 创建视频推荐服务。
func NewService(cache hgCandidateCache, repository hgCardRepository, generation string) *Service {
	return &Service{cache: cache, repository: repository, generation: generation, now: time.Now}
}

// Feed 返回稳定游标推荐页。请求期只执行有界 Redis Pipeline 和一次批量 MySQL 冷回源。
func (s *Service) Feed(ctx context.Context, userID, encodedCursor string, pageSize int) (VideoRecommendDtoPackage.HGFeedResponse, error) {
	response := VideoRecommendDtoPackage.HGFeedResponse{Generation: s.generation, Items: []VideoRecommendDtoPackage.HGFeedItem{}}
	if strings.TrimSpace(userID) == "" || s.cache == nil || s.repository == nil {
		return response, ErrInvalidRequest
	}
	if pageSize == 0 {
		pageSize = hgDefaultPageSize
	}
	if pageSize < 1 || pageSize > hgMaxPageSize {
		return response, ErrInvalidRequest
	}
	cursor, err := s.decodeCursor(encodedCursor)
	if err != nil {
		return response, ErrInvalidRequest
	}
	requestCtx, cancel := context.WithTimeout(ctx, hgRecommendTimeout)
	defer cancel()

	// 多取有限候选用于过滤已删除/下架内容；额外一条用于判断是否可能存在下一页。
	candidateLimit := pageSize*hgCandidateMultiplier + 1
	candidates, err := s.cache.ListCandidates(requestCtx, cursor, candidateLimit)
	if err != nil {
		return response, fmt.Errorf("%w: recall candidates: %v", ErrUnavailable, err)
	}
	if len(candidates) == 0 {
		response.PageSize = pageSize
		return response, nil
	}
	ids := make([]string, len(candidates))
	for i := range candidates {
		ids[i] = candidates[i].SubmissionID
	}
	cards, misses, err := s.cache.GetCards(requestCtx, ids)
	if err != nil {
		return response, fmt.Errorf("%w: read cards: %v", ErrUnavailable, err)
	}
	if len(misses) > 0 {
		loaded, loadErr := s.repository.BatchGetPublicVideoCards(requestCtx, misses)
		if loadErr != nil {
			return response, fmt.Errorf("%w: load cards: %v", ErrUnavailable, loadErr)
		}
		for id, item := range loaded {
			cards[id] = item
		}
		_ = s.cache.SetCards(requestCtx, loaded)
	}

	items := make([]VideoRecommendDtoPackage.HGFeedItem, 0, pageSize)
	lastScanned := -1
	for index, candidate := range candidates {
		// 游标必须推进到最后扫描候选，而不是最后返回候选，否则 stale 候选会被下一页反复读取。
		lastScanned = index
		item, ok := cards[candidate.SubmissionID]
		if !ok {
			continue
		}
		item.Reason = hgRecommendationReason(item, s.now())
		items = append(items, item)
		if len(items) == pageSize {
			break
		}
	}
	if err := s.cache.FillInteractionCounts(requestCtx, items); err != nil {
		return response, fmt.Errorf("%w: load interaction counts: %v", ErrUnavailable, err)
	}
	hgRerank(items)
	response.PageSize = pageSize
	response.Items = items
	// candidateLimit 已包含一条探测记录；等于上限时只能表示“可能有更多”，由下一页继续确认。
	response.HasMore = lastScanned >= 0 && (lastScanned+1 < len(candidates) || len(candidates) == candidateLimit)
	if lastScanned >= 0 {
		last := candidates[lastScanned]
		response.NextCursor, err = s.encodeCursor(VideoRecommendModelPackage.HGCursor{Generation: s.generation, Score: last.Score, SubmissionID: last.SubmissionID})
		if err != nil {
			return response, fmt.Errorf("%w: encode cursor", ErrUnavailable)
		}
	}
	return response, nil
}

func (s *Service) decodeCursor(encoded string) (VideoRecommendModelPackage.HGCursor, error) {
	if strings.TrimSpace(encoded) == "" {
		return VideoRecommendModelPackage.HGCursor{Generation: s.generation}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return VideoRecommendModelPackage.HGCursor{}, err
	}
	var cursor VideoRecommendModelPackage.HGCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return cursor, err
	}
	if cursor.Generation != s.generation || cursor.Score <= 0 || strings.TrimSpace(cursor.SubmissionID) == "" {
		// generation 不一致时拒绝旧游标，避免 Feed 切代后跨版本读取产生重复或遗漏。
		return cursor, ErrInvalidRequest
	}
	return cursor, nil
}

func (s *Service) encodeCursor(cursor VideoRecommendModelPackage.HGCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func hgRecommendationReason(item VideoRecommendDtoPackage.HGFeedItem, now time.Time) string {
	publishedAt, err := time.Parse(time.RFC3339Nano, item.PublishTime)
	if err == nil && now.Sub(publishedAt) <= 72*time.Hour {
		return "新鲜发布"
	}
	if item.Category != "" {
		return "热门" + item.Category + "内容"
	}
	return "热门内容"
}

// hgRerank 在当前页内做稳定的作者/分区交错，不跨页挪动候选，保证游标不丢不重。
func hgRerank(items []VideoRecommendDtoPackage.HGFeedItem) {
	for i := 1; i < len(items); i++ {
		if items[i].AuthorID != items[i-1].AuthorID && items[i].Category != items[i-1].Category {
			continue
		}
		for j := i + 1; j < len(items); j++ {
			if items[j].AuthorID != items[i-1].AuthorID && items[j].Category != items[i-1].Category {
				items[i], items[j] = items[j], items[i]
				break
			}
		}
	}
}
