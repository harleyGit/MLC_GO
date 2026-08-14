package VideoInteractionServicePackage

import (
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	EventBusPackage "MLC_GO/internal/infrastructure/eventbus"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	VideoInteractionDtoPackage "MLC_GO/internal/modules/video_interaction/dto"
	"MLC_GO/internal/pkg/idgen"
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidAction    = errors.New("不支持的视频互动操作")
	ErrInvalidTarget    = errors.New("互动目标不能为空")
	ErrInvalidQuantity  = errors.New("投币数量必须为 1 或 2，且单视频累计不超过 2")
	ErrCannotFollowSelf = errors.New("不能关注自己")
	ErrInvalidRequestID = errors.New("投币请求幂等标识不能为空")
	ErrInsufficientCoin = errors.New("硬币余额不足")
)

type hgInteractionCache interface {
	GetState(ctx context.Context, userID string, submissionID string, authorID string) (VideoInteractionDtoPackage.StateResponse, error)
	ApplyOptimistic(ctx context.Context, userID string, submissionID string, targetID string, action string, active bool, quantity int) error
}

type hgCoinStore interface {
	Debit(context.Context, CoinServicePackage.HGDebitCommand) (CoinModelPackage.HGMutationResult, error)
}

type hgIDGenerator interface {
	Generate(idgen.EntityType) (string, error)
}

// Service 负责校验互动命令、发布 Kafka 事件及更新实时 Redis 投影。
type Service struct {
	eventBus  EventBusPackage.EventBus
	cache     hgInteractionCache
	coinStore hgCoinStore
	idGen     hgIDGenerator
}

func NewService(eventBus EventBusPackage.EventBus, cache hgInteractionCache, coinStores ...hgCoinStore) *Service {
	var coinStore hgCoinStore
	if len(coinStores) > 0 {
		coinStore = coinStores[0]
	}
	return &Service{eventBus: eventBus, cache: cache, coinStore: coinStore}
}

// NewServiceWithIDGenerator 创建包含业务 ID 生成能力的视频互动服务。
func NewServiceWithIDGenerator(eventBus EventBusPackage.EventBus, cache hgInteractionCache, idGen hgIDGenerator, coinStores ...hgCoinStore) *Service {
	service := NewService(eventBus, cache, coinStores...)
	service.idGen = idGen
	return service
}

func (s *Service) GetState(ctx context.Context, userID string, submissionID string, authorID string) (VideoInteractionDtoPackage.StateResponse, error) {
	if strings.TrimSpace(submissionID) == "" {
		return VideoInteractionDtoPackage.StateResponse{}, ErrInvalidTarget
	}
	return s.cache.GetState(ctx, userID, submissionID, authorID)
}

func (s *Service) SetVideoInteraction(ctx context.Context, userID string, req VideoInteractionDtoPackage.ActionRequest) (VideoInteractionDtoPackage.AcceptedResponse, error) {
	req.SubmissionID = strings.TrimSpace(req.SubmissionID)
	if userID == "" || req.SubmissionID == "" {
		return VideoInteractionDtoPackage.AcceptedResponse{}, ErrInvalidTarget
	}
	switch req.Action {
	case "like", "favorite":
		req.Quantity = 0
	case "share":
		req.Active = true
		req.Quantity = 1
	case "coin":
		if req.Quantity < 1 || req.Quantity > 2 {
			return VideoInteractionDtoPackage.AcceptedResponse{}, ErrInvalidQuantity
		}
		req.RequestID = strings.TrimSpace(req.RequestID)
		if req.RequestID == "" {
			return VideoInteractionDtoPackage.AcceptedResponse{}, ErrInvalidRequestID
		}
	default:
		return VideoInteractionDtoPackage.AcceptedResponse{}, ErrInvalidAction
	}
	if s.eventBus == nil {
		return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("interaction event bus cannot be nil")
	}
	event := InteractionEventsPackage.VideoInteractionChangedEvent{
		EventMeta: events.NewEventMeta(ctx), ActorUserID: userID, SubmissionID: req.SubmissionID,
		Action: req.Action, Active: req.Active, Quantity: req.Quantity,
	}
	if req.Action == "coin" {
		if s.coinStore == nil {
			return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("coin transaction store cannot be nil")
		}
		result, err := s.coinStore.Debit(ctx, CoinServicePackage.HGDebitCommand{
			UserID: userID, RequestID: req.RequestID, Amount: uint64(req.Quantity), Reason: "video_coin",
			BusinessType: "video_coin", BusinessKey: req.SubmissionID, BusinessLimit: 2, Event: event,
		})
		if err != nil {
			return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("submit coin transaction: %w", err)
		}
		if result.Committed && s.cache != nil {
			_ = s.cache.ApplyOptimistic(ctx, userID, req.SubmissionID, "", req.Action, true, req.Quantity)
		}
		return VideoInteractionDtoPackage.AcceptedResponse{Accepted: true, Action: req.Action, Active: true, Quantity: req.Quantity}, nil
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("publish interaction event: %w", err)
	}
	if s.cache != nil {
		_ = s.cache.ApplyOptimistic(ctx, userID, req.SubmissionID, "", req.Action, req.Active, req.Quantity)
	}
	return VideoInteractionDtoPackage.AcceptedResponse{Accepted: true, Action: req.Action, Active: req.Active, Quantity: req.Quantity}, nil
}

func (s *Service) SetFollow(ctx context.Context, userID string, req VideoInteractionDtoPackage.FollowRequest) (VideoInteractionDtoPackage.AcceptedResponse, error) {
	req.FolloweeID = strings.TrimSpace(req.FolloweeID)
	if userID == "" || req.FolloweeID == "" {
		return VideoInteractionDtoPackage.AcceptedResponse{}, ErrInvalidTarget
	}
	if userID == req.FolloweeID {
		return VideoInteractionDtoPackage.AcceptedResponse{}, ErrCannotFollowSelf
	}
	if s.eventBus == nil {
		return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("interaction event bus cannot be nil")
	}
	if s.idGen == nil {
		return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("follow id generator cannot be nil")
	}
	followID, err := s.idGen.Generate(idgen.TypeFollow)
	if err != nil {
		return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("generate follow id: %w", err)
	}
	event := InteractionEventsPackage.UserFollowChangedEvent{
		EventMeta: events.NewEventMeta(ctx), FollowID: followID, FollowerID: userID, FolloweeID: req.FolloweeID, Active: req.Active,
	}
	if err := s.eventBus.Publish(ctx, event); err != nil {
		return VideoInteractionDtoPackage.AcceptedResponse{}, fmt.Errorf("publish follow event: %w", err)
	}
	if s.cache != nil {
		_ = s.cache.ApplyOptimistic(ctx, userID, "", req.FolloweeID, "follow", req.Active, 0)
	}
	return VideoInteractionDtoPackage.AcceptedResponse{Accepted: true, Action: "follow", Active: req.Active}, nil
}
