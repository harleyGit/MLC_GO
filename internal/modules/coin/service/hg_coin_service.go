package CoinServicePackage

import (
	"MLC_GO/internal/events"
	CoinEventsPackage "MLC_GO/internal/events/coin"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

var (
	ErrHGInvalidIdentity  = errors.New("coin user and request IDs are required")
	ErrHGInvalidAmount    = errors.New("coin amount must be positive")
	ErrHGInvalidReference = errors.New("coin refund reference is required")
	ErrHGInvalidReason    = errors.New("coin audit reason is required")
)

// HGMaxMutationAmount bounds one command so a debit never requires more than the repository's 1000 locked FEFO lots.
const HGMaxMutationAmount uint64 = 1000

type hgCoinRepository interface {
	Mutate(context.Context, CoinModelPackage.HGCommand) (CoinModelPackage.HGMutationResult, error)
	Balance(context.Context, string) (uint64, error)
}

// HGService 是硬币权威资产的应用边界。
//
// Recharge、Grant、Refund 和 Correct 故意不直接暴露 HTTP Handler。当前项目的普通 JWT/角色头不足以
// 作为资金操作授权依据，只有完成支付签名校验、服务身份认证或管理员强授权的可信 Adapter 才能调用这些方法。
// Service 负责参数边界和业务命令构造，余额、幂等、锁、流水、lot 与 Outbox 原子性由 Repository 保证。
type HGService struct{ repository hgCoinRepository }

// NewHGService 创建权威硬币服务；repository 必须连接同一个承载 wallet、transaction 和 Outbox 的 MySQL。
func NewHGService(repository hgCoinRepository) *HGService { return &HGService{repository: repository} }

// HGCreditCommand 表示充值或赠币命令；ExpiresAt 为空时生成永久 lot。
type HGCreditCommand struct {
	UserID, RequestID, Reason, BusinessType, BusinessKey string
	Amount                                               uint64
	ExpiresAt                                            *time.Time
}

// HGDebitCommand 表示消费硬币命令；BusinessLimit 为同一业务键的累计消费上限。
type HGDebitCommand struct {
	UserID, RequestID, Reason, BusinessType, BusinessKey string
	Amount, BusinessLimit                                uint64
	Event                                                events.DomainEvent
}

// HGRefundCommand 表示关联原 debit transaction 的退款命令，累计退款不能超过原扣减额。
type HGRefundCommand struct {
	UserID, RequestID, Reason string
	Amount                    uint64
	ReferenceTransactionID    uint64
}

// HGCorrectionCommand 表示人工或对账修正；Reason 必填并写入不可变审计流水。
type HGCorrectionCommand struct {
	UserID, RequestID, Reason string
	Delta                     int64
}

// Initialize 幂等建立零余额钱包审计状态；批量历史初始化任务只调用 EnsureWallet，避免制造海量零值流水。
func (s *HGService) Initialize(ctx context.Context, userID, requestID string) (CoinModelPackage.HGMutationResult, error) {
	command := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationInitialize, UserID: userID, RequestID: requestID}
	command.Event = hgEvent(ctx, command)
	return s.hgMutate(ctx, command)
}

// Recharge 为通过支付回调验签的可信 Adapter 入账，并可生成带有效期的 lot。
func (s *HGService) Recharge(ctx context.Context, command HGCreditCommand) (CoinModelPackage.HGMutationResult, error) {
	return s.hgCredit(ctx, CoinModelPackage.HGOperationRecharge, command)
}

// Grant 为活动、补偿等可信来源赠币，Reason 应能关联活动或工单。
func (s *HGService) Grant(ctx context.Context, command HGCreditCommand) (CoinModelPackage.HGMutationResult, error) {
	return s.hgCredit(ctx, CoinModelPackage.HGOperationGrant, command)
}

func (s *HGService) hgCredit(ctx context.Context, operation CoinModelPackage.HGOperation, command HGCreditCommand) (CoinModelPackage.HGMutationResult, error) {
	if command.Amount == 0 || command.Amount > HGMaxMutationAmount {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidAmount
	}
	coinCommand := CoinModelPackage.HGCommand{Operation: operation, UserID: command.UserID, RequestID: command.RequestID, Amount: command.Amount, Reason: command.Reason, BusinessType: command.BusinessType, BusinessKey: command.BusinessKey, ExpiresAt: command.ExpiresAt}
	coinCommand.Event = hgEvent(ctx, coinCommand)
	return s.hgMutate(ctx, coinCommand)
}

// Debit 在用户钱包锁下按 FEFO 消耗 lot，并可在同事务写入调用方提供的领域事件。
func (s *HGService) Debit(ctx context.Context, command HGDebitCommand) (CoinModelPackage.HGMutationResult, error) {
	if command.Amount == 0 || command.Amount > HGMaxMutationAmount {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidAmount
	}
	coinCommand := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationDebit, UserID: command.UserID, RequestID: command.RequestID, Amount: command.Amount, Reason: command.Reason, BusinessType: command.BusinessType, BusinessKey: command.BusinessKey, BusinessLimit: command.BusinessLimit, Event: command.Event}
	if coinCommand.Event == nil {
		coinCommand.Event = hgEvent(ctx, coinCommand)
	}
	return s.hgMutate(ctx, coinCommand)
}

// Refund 关联原扣减流水创建永久补偿 lot；该策略避免把已过期 lot 恢复成可用资产。
func (s *HGService) Refund(ctx context.Context, command HGRefundCommand) (CoinModelPackage.HGMutationResult, error) {
	// Refunds create a new permanent compensation lot linked to the original debit transaction.
	if command.Amount == 0 || command.Amount > HGMaxMutationAmount {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidAmount
	}
	if command.ReferenceTransactionID == 0 {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidReference
	}
	coinCommand := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationRefund, UserID: command.UserID, RequestID: command.RequestID, Amount: command.Amount, Reason: command.Reason, ReferenceTransactionID: command.ReferenceTransactionID}
	coinCommand.Event = hgEvent(ctx, coinCommand)
	return s.hgMutate(ctx, coinCommand)
}

// Correct 通过不可变 correction 流水修正已确认的资产漂移，禁止直接覆盖 wallet.balance。
func (s *HGService) Correct(ctx context.Context, command HGCorrectionCommand) (CoinModelPackage.HGMutationResult, error) {
	if command.Delta == 0 || command.Delta == math.MinInt64 || command.Delta > int64(HGMaxMutationAmount) || command.Delta < -int64(HGMaxMutationAmount) {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidAmount
	}
	if strings.TrimSpace(command.Reason) == "" {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidReason
	}
	coinCommand := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationCorrection, UserID: command.UserID, RequestID: command.RequestID, SignedDelta: command.Delta, Reason: command.Reason}
	if command.Delta > 0 {
		coinCommand.Amount = uint64(command.Delta)
	} else {
		coinCommand.Amount = uint64(-command.Delta)
	}
	coinCommand.Event = hgEvent(ctx, coinCommand)
	return s.hgMutate(ctx, coinCommand)
}

// Balance 返回 MySQL 权威余额；缺失钱包会先做幂等零余额初始化。
func (s *HGService) Balance(ctx context.Context, userID string) (uint64, error) {
	if strings.TrimSpace(userID) == "" {
		return 0, ErrHGInvalidIdentity
	}
	return s.repository.Balance(ctx, userID)
}

func (s *HGService) hgMutate(ctx context.Context, command CoinModelPackage.HGCommand) (CoinModelPackage.HGMutationResult, error) {
	command.UserID = strings.TrimSpace(command.UserID)
	command.RequestID = strings.TrimSpace(command.RequestID)
	if command.UserID == "" || command.RequestID == "" {
		return CoinModelPackage.HGMutationResult{}, ErrHGInvalidIdentity
	}
	return s.repository.Mutate(ctx, command)
}

func hgEvent(ctx context.Context, command CoinModelPackage.HGCommand) CoinEventsPackage.HGAssetChangedEvent {
	return CoinEventsPackage.HGAssetChangedEvent{EventMeta: events.NewEventMeta(ctx), UserID: command.UserID, Operation: string(command.Operation), Amount: command.Amount, BusinessType: command.BusinessType, BusinessKey: command.BusinessKey, ReferenceTransactionID: command.ReferenceTransactionID}
}
