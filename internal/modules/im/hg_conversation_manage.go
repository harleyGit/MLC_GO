package im

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// HGConversationByID 在创建或恢复旧会话后点查本人会话，供客户端获取权威最新 seq。
func (s *HGService) HGConversationByID(ctx context.Context, user string, id uint64) (HGConversation, error) {
	if s == nil || s.db == nil || user == "" || id == 0 {
		return HGConversation{}, HGErrInvalid
	}
	item := HGConversation{ConversationID: strconv.FormatUint(id, 10)}
	var low, high string
	err := s.db.QueryRowContext(ctx, SQL.HGIMConversationByIDSQL, id, user).Scan(&item.LastSeq, &item.LastReadSeq, &item.UpdatedAt, &low, &high, &item.Muted)
	if errors.Is(err, sql.ErrNoRows) {
		return HGConversation{}, HGErrForbidden
	}
	if err != nil {
		return HGConversation{}, err
	}
	item.UpdatedAt = item.UpdatedAt.UTC()
	if user == low {
		item.Peer = HGPeer{UserID: high, Name: high}
	} else {
		item.Peer = HGPeer{UserID: low, Name: low}
	}
	return item, nil
}

// HGSetConversationMuted 仅更新已认证用户自己的免打扰设置；不存在的成员失败关闭。
func (s *HGService) HGSetConversationMuted(ctx context.Context, user string, id uint64, muted bool) error {
	if s == nil || s.db == nil || user == "" || id == 0 {
		return HGErrInvalid
	}
	result, err := s.db.ExecContext(ctx, SQL.HGIMSetMutedSQL, muted, id, user)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// MySQL 在值相同时返回零变更行；点查成员区分幂等和越权。
		var member int
		if err := s.db.QueryRowContext(ctx, SQL.HGIMConversationMemberSQL, id, user).Scan(&member); errors.Is(err, sql.ErrNoRows) {
			return HGErrForbidden
		} else if err != nil {
			return err
		}
	}
	return nil
}

// HGDeleteConversationBatch 每次最多物理清理 20 条三天前的双方消息与关联指针；全部完成才隐藏操作者会话。
// 返回 baseSeq 供客户端后续批次保持最初水位，新到的消息不会因长时间清理被误隐藏。
func (s *HGService) HGDeleteConversationBatch(ctx context.Context, user string, id, baseSeq uint64) (uint64, bool, error) {
	if s == nil || s.db == nil || user == "" || id == 0 || baseSeq > uint64(^uint64(0)>>1) {
		return 0, false, HGErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	var low, high string
	var lastSeq uint64
	if err := tx.QueryRowContext(ctx, SQL.HGIMLockConversationSQL, id).Scan(&low, &high, &lastSeq); errors.Is(err, sql.ErrNoRows) {
		return 0, false, HGErrForbidden
	} else if err != nil {
		return 0, false, err
	}
	if user != low && user != high {
		return 0, false, HGErrForbidden
	}
	var stagedSeq uint64
	var deletedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, SQL.HGIMDeleteProgressSQL, id, user).Scan(&stagedSeq, &deletedAt); errors.Is(err, sql.ErrNoRows) {
		return 0, false, HGErrForbidden
	} else if err != nil {
		return 0, false, err
	}
	if stagedSeq == 0 || deletedAt.Valid {
		baseSeq = lastSeq
		if _, err := tx.ExecContext(ctx, SQL.HGIMStageConversationDeleteSQL, baseSeq, id, user); err != nil {
			return 0, false, err
		}
	} else if baseSeq != 0 && baseSeq != stagedSeq {
		return 0, false, HGErrInvalid
	} else {
		baseSeq = stagedSeq
	}
	// 数据库字段以 UTC 写入；截止时间使用一次请求内固定的 UTC 时间，禁止全表扫描与深分页。
	rows, err := tx.QueryContext(ctx, SQL.HGIMOldMessagesSQL, id, time.Now().UTC().Add(-72*time.Hour))
	if err != nil {
		return 0, false, err
	}
	ids := make([]uint64, 0, 21)
	for rows.Next() {
		var messageID uint64
		if err = rows.Scan(&messageID); err != nil {
			break
		}
		ids = append(ids, messageID)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return 0, false, err
	}
	more := len(ids) > 20
	if more {
		ids = ids[:20]
	}
	for _, messageID := range ids {
		for _, query := range []string{SQL.HGIMDeleteJobSQL, SQL.HGIMDeleteDeliverySQL, SQL.HGIMDeleteAttachmentLinkSQL} {
			if _, err := tx.ExecContext(ctx, query, messageID); err != nil {
				return 0, false, err
			}
		}
		if _, err := tx.ExecContext(ctx, SQL.HGIMDeleteMessageSQL, messageID, id); err != nil {
			return 0, false, err
		}
	}
	if !more {
		if _, err := tx.ExecContext(ctx, SQL.HGIMHideConversationSQL, baseSeq, id, user); err != nil {
			return 0, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return baseSeq, more, nil
}
