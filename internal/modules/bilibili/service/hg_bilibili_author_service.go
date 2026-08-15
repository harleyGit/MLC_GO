package BilibiliServicePackage

import (
	BilibiliCachePackage "MLC_GO/internal/modules/bilibili/cache"
	BilibiliDtoPackage "MLC_GO/internal/modules/bilibili/dto"
	BilibiliRepositoryPackage "MLC_GO/internal/modules/bilibili/repository"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	hgDefaultAuthorVideoPageSize = 20
	hgMaxAuthorVideoPageSize     = 50
)

var ErrInvalidAuthorRequest = errors.New("作者空间请求参数无效")

// Service 聚合作者公开资料、统计和公开视频；缓存未命中时使用 singleflight 合并同一热点请求。
type Service struct {
	repo  *BilibiliRepositoryPackage.Repository
	cache *BilibiliCachePackage.Cache
	group singleflight.Group
}

// NewService 创建作者空间服务。
func NewService(repo *BilibiliRepositoryPackage.Repository, cache *BilibiliCachePackage.Cache) *Service {
	return &Service{repo: repo, cache: cache}
}

// GetProfile 获取作者公开资料。
func (s *Service) GetProfile(ctx context.Context, userID string) (BilibiliDtoPackage.HGAuthorProfileResponse, error) {
	userID, err := hgValidateUserID(userID)
	if err != nil {
		return BilibiliDtoPackage.HGAuthorProfileResponse{}, err
	}
	if value, hit, cacheErr := s.cache.GetProfile(ctx, userID); cacheErr == nil && hit {
		return value, nil
	}

	value, err, _ := s.group.Do("profile:"+userID, func() (interface{}, error) {
		profile, repoErr := s.repo.GetProfile(ctx, userID)
		if repoErr != nil {
			return nil, repoErr
		}
		_ = s.cache.SetProfile(ctx, userID, profile)
		return profile, nil
	})
	if err != nil {
		return BilibiliDtoPackage.HGAuthorProfileResponse{}, err
	}
	return value.(BilibiliDtoPackage.HGAuthorProfileResponse), nil
}

// GetStats 获取粉丝、关注和公开视频数；三个独立读并行执行且数量固定。
func (s *Service) GetStats(ctx context.Context, userID string) (BilibiliDtoPackage.HGAuthorStatsResponse, error) {
	userID, err := hgValidateUserID(userID)
	if err != nil {
		return BilibiliDtoPackage.HGAuthorStatsResponse{}, err
	}
	if value, hit, cacheErr := s.cache.GetStats(ctx, userID); cacheErr == nil && hit {
		return value, nil
	}

	value, err, _ := s.group.Do("stats:"+userID, func() (interface{}, error) {
		var stats BilibiliDtoPackage.HGAuthorStatsResponse
		group, groupCtx := errgroup.WithContext(ctx)
		group.Go(func() error {
			count, repoErr := s.repo.CountFollowing(groupCtx, userID)
			stats.FollowingCount = count
			return repoErr
		})
		group.Go(func() error {
			count, repoErr := s.repo.CountVideos(groupCtx, userID)
			stats.VideoCount = count
			return repoErr
		})
		group.Go(func() error {
			count, hit, cacheErr := s.cache.GetFollowerCount(groupCtx, userID)
			if cacheErr == nil && hit {
				stats.FollowerCount = count
				return nil
			}
			if cacheErr != nil && !errors.Is(cacheErr, redis.Nil) {
				// Redis 读失败降级到固定分片 MySQL 聚合，不放大到全表扫描。
			}
			count, repoErr := s.repo.SumFollowers(groupCtx, userID)
			stats.FollowerCount = count
			return repoErr
		})
		if groupErr := group.Wait(); groupErr != nil {
			return nil, fmt.Errorf("load bilibili author stats: %w", groupErr)
		}
		_ = s.cache.SetStats(ctx, userID, stats)
		return stats, nil
	})
	if err != nil {
		return BilibiliDtoPackage.HGAuthorStatsResponse{}, err
	}
	return value.(BilibiliDtoPackage.HGAuthorStatsResponse), nil
}

// GetVideos 获取作者公开视频，页大小有上限且使用复合游标。
func (s *Service) GetVideos(ctx context.Context, userID, cursor string, pageSize int) (BilibiliDtoPackage.HGAuthorVideoListResponse, error) {
	userID, err := hgValidateUserID(userID)
	if err != nil {
		return BilibiliDtoPackage.HGAuthorVideoListResponse{}, err
	}
	pageSize = hgNormalizePageSize(pageSize)
	cursorTime, cursorID, err := hgParseAuthorVideoCursor(cursor)
	if err != nil {
		return BilibiliDtoPackage.HGAuthorVideoListResponse{}, err
	}
	if value, hit, cacheErr := s.cache.GetVideos(ctx, userID, cursor, pageSize); cacheErr == nil && hit {
		return value, nil
	}

	key := fmt.Sprintf("videos:%s:%d:%s", userID, pageSize, cursor)
	value, err, _ := s.group.Do(key, func() (interface{}, error) {
		videos, repoErr := s.repo.GetVideos(ctx, userID, cursorTime, cursorID, pageSize+1)
		if repoErr != nil {
			return nil, repoErr
		}
		hasMore := len(videos) > pageSize
		if hasMore {
			videos = videos[:pageSize]
		}
		// 互动计数是增强字段，Redis 故障不阻断作者视频主列表。
		_ = s.cache.FillVideoCounts(ctx, videos)
		response := BilibiliDtoPackage.HGAuthorVideoListResponse{PageSize: pageSize, HasMore: hasMore, Videos: videos}
		if hasMore && len(videos) > 0 {
			last := videos[len(videos)-1]
			response.NextCursor = last.PublishTime + "|" + last.SubmissionID
		}
		_ = s.cache.SetVideos(ctx, userID, cursor, pageSize, response)
		return response, nil
	})
	if err != nil {
		return BilibiliDtoPackage.HGAuthorVideoListResponse{}, err
	}
	return value.(BilibiliDtoPackage.HGAuthorVideoListResponse), nil
}

// GetHomepage 并行聚合作者空间首屏，后续翻页不重复请求资料和统计。
func (s *Service) GetHomepage(ctx context.Context, userID, cursor string, pageSize int) (BilibiliDtoPackage.HGAuthorHomepageResponse, error) {
	var response BilibiliDtoPackage.HGAuthorHomepageResponse
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		value, err := s.GetProfile(groupCtx, userID)
		response.Profile = value
		return err
	})
	group.Go(func() error {
		value, err := s.GetStats(groupCtx, userID)
		response.Stats = value
		return err
	})
	group.Go(func() error {
		value, err := s.GetVideos(groupCtx, userID, cursor, pageSize)
		response.Videos = value
		return err
	})
	if err := group.Wait(); err != nil {
		return response, err
	}
	return response, nil
}

func hgValidateUserID(userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" || len(userID) > 255 {
		return "", ErrInvalidAuthorRequest
	}
	return userID, nil
}

func hgNormalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return hgDefaultAuthorVideoPageSize
	}
	if pageSize > hgMaxAuthorVideoPageSize {
		return hgMaxAuthorVideoPageSize
	}
	return pageSize
}

func hgParseAuthorVideoCursor(cursor string) (*time.Time, string, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return nil, "", nil
	}
	parts := strings.SplitN(cursor, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, "", ErrInvalidAuthorRequest
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, "", ErrInvalidAuthorRequest
	}
	return &parsed, parts[1], nil
}
