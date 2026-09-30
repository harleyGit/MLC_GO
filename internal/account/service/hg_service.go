package accountservice

import (
	"MLC_GO/internal/account"
	"context"
	"errors"
)

// HGService 提供账户权威只读适配，不重复实现 coin 的记账、幂等或事务流程。
type HGService struct {
	reader account.HGAccountReader // reader 复用账户只读仓储，不持有资金写入能力。
}

// NewHGService 创建账户读取服务；reader 为空时后续调用明确返回错误。
func NewHGService(reader account.HGAccountReader) *HGService {
	return &HGService{reader: reader}
}

// GetAccount 读取账户当前权威快照。
func (s *HGService) GetAccount(ctx context.Context, userID string) (account.HGAccount, error) {
	if s == nil || s.reader == nil {
		return account.HGAccount{}, errors.New("账户读取仓储不能为空")
	}
	return s.reader.GetAccount(ctx, userID)
}

// ListLedger 读取有界账户流水。
func (s *HGService) ListLedger(ctx context.Context, userID string, limit int) ([]account.HGAccountLedger, error) {
	if s == nil || s.reader == nil {
		return nil, errors.New("账户读取仓储不能为空")
	}
	return s.reader.ListLedger(ctx, userID, limit)
}
