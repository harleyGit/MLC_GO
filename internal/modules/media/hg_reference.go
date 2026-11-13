package media

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"regexp"
	"slices"
	"strings"
)

var hgReferenceID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// HGReferenceCommand 是业务服务的内部事务协议，不是客户端 DTO；权威业务资格须由回调核验。
type HGReferenceCommand struct {
	AssetID     string // 待绑定的 ready 媒体资源 UUID，由业务服务核实请求权限后传入。
	OwnerID     string // 上传者的受信 JWT ID，业务回调仍须校验其商家/会话等真实资格。
	Purpose     string // 与上传时目的完全一致的用途，如 user_avatar/product_main。
	BizType     string // 权威业务域，只允许与 Purpose 一致。
	BizID       string // 权威业务主键，使用稳定且不可跨租户歧义的 ID。
	Slot        string // 唯一槽位（头像 avatar、商品 main 等），不接受任意路径字符。
	ExpectedOld string // 乐观并发：预期的旧 asset_id，空代表原槽未绑定。
}

// HGAuthorizeBusiness 必须在同一 tx 中先验证业务主体、租户和版本，并完成业务主表引用更新。
// 若业务数据在其他 MySQL 实例，不得使用该接口模拟跨库原子事务。
type HGAuthorizeBusiness func(context.Context, *sql.Tx) error

func hgValidateReference(cmd HGReferenceCommand) error {
	policy, ok := hgPolicies[cmd.Purpose]
	biz := policy.biz
	if strings.HasPrefix(cmd.Purpose, "product_") {
		biz = "product"
	}
	if !ok || biz != cmd.BizType || !hgReferenceID.MatchString(cmd.OwnerID) || !hgReferenceID.MatchString(cmd.BizID) || !hgReferenceID.MatchString(cmd.Slot) || uuid.Validate(cmd.AssetID) != nil || cmd.ExpectedOld != "" && uuid.Validate(cmd.ExpectedOld) != nil {
		return HGErrInvalid
	}
	return nil
}

type hgLockedAsset struct {
	owner   string // 资源上传者/业务主体，用于与权威业务身份比对。
	purpose string // 上传用途，必须与业务引用命令匹配。
	status  string // 资源状态，仅 ready 允许绑定。
	session string // 会话状态 ready/attached；其他状态不可引用。
}

func (s *HGService) hgLockReferenceAssets(ctx context.Context, tx *sql.Tx, ids ...string) (map[string]hgLockedAsset, error) {
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	statuses := make(map[string]hgLockedAsset, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		var data hgLockedAsset
		if err := tx.QueryRowContext(ctx, SQL.HGMediaReferenceSessionSQL, id).Scan(&data.session); err != nil {
			return nil, err
		}
		statuses[id] = data
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		data := statuses[id]
		if err := tx.QueryRowContext(ctx, SQL.HGMediaReferenceAssetSQL, id).Scan(&data.owner, &data.purpose, &data.status); err != nil {
			return nil, err
		}
		if data.status != "ready" || data.session != "ready" && data.session != "attached" {
			return nil, HGErrConflict
		}
		statuses[id] = data
	}
	return statuses, nil
}

// HGReplaceReferenceTx 只供已授权业务服务调用；必须传入由业务主表验证/更新的同一 MySQL 事务。
// 失败后调用方必须回滚，不能先提交业务主表再调用本方法。
func (s *HGService) HGReplaceReferenceTx(ctx context.Context, tx *sql.Tx, cmd HGReferenceCommand, authorize HGAuthorizeBusiness) error {
	if tx == nil || authorize == nil || hgValidateReference(cmd) != nil {
		return HGErrInvalid
	}
	if err := authorize(ctx, tx); err != nil {
		return err
	}
	var old string
	err := tx.QueryRowContext(ctx, SQL.HGMediaReferenceSlotSQL, cmd.BizType, cmd.BizID, cmd.Slot).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if old != cmd.ExpectedOld {
		return HGErrConflict
	}
	statuses, err := s.hgLockReferenceAssets(ctx, tx, cmd.AssetID, old)
	if err != nil {
		return err
	}
	data := statuses[cmd.AssetID]
	if data.owner != cmd.OwnerID || data.purpose != cmd.Purpose {
		return HGErrNotFound
	}
	if old != "" && (statuses[old].owner != cmd.OwnerID || statuses[old].purpose != cmd.Purpose) {
		return HGErrNotFound
	}
	if old == cmd.AssetID {
		return nil
	}
	if old == "" {
		_, err = tx.ExecContext(ctx, SQL.HGMediaReferenceInsertSQL, cmd.AssetID, cmd.OwnerID, cmd.BizType, cmd.BizID, cmd.Slot)
	} else {
		var res sql.Result
		res, err = tx.ExecContext(ctx, SQL.HGMediaReferenceUpdateSQL, cmd.AssetID, cmd.OwnerID, cmd.BizType, cmd.BizID, cmd.Slot, old)
		if err == nil {
			var n int64
			n, err = res.RowsAffected()
			if err == nil && n != 1 {
				err = HGErrConflict
			}
		}
	}
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, SQL.HGMediaReferenceAttachSQL, cmd.AssetID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return HGErrConflict
	}
	if old != "" {
		if err = s.hgReleaseOld(ctx, tx, old); err != nil {
			return err
		}
	}
	return nil
}

func (s *HGService) hgReleaseOld(ctx context.Context, tx *sql.Tx, old string) error {
	var found int
	err := tx.QueryRowContext(ctx, SQL.HGMediaHasRefSQL, old).Scan(&found)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, SQL.HGMediaReferenceReleaseSQL, old)
	return err
}

// HGDetachReferenceTx 由业务服务完成业务主表解除绑定后调用，保留旧文件至少 24 小时以便回滚。
func (s *HGService) HGDetachReferenceTx(ctx context.Context, tx *sql.Tx, cmd HGReferenceCommand, authorize HGAuthorizeBusiness) error {
	if tx == nil || authorize == nil || hgValidateReference(cmd) != nil || cmd.ExpectedOld == "" || cmd.ExpectedOld != cmd.AssetID {
		return HGErrInvalid
	}
	if err := authorize(ctx, tx); err != nil {
		return err
	}
	var old string
	if err := tx.QueryRowContext(ctx, SQL.HGMediaReferenceSlotSQL, cmd.BizType, cmd.BizID, cmd.Slot).Scan(&old); err != nil {
		return err
	}
	if old != cmd.ExpectedOld || old == "" {
		return HGErrConflict
	}
	statuses, err := s.hgLockReferenceAssets(ctx, tx, old)
	if err != nil {
		return err
	}
	if statuses[old].owner != cmd.OwnerID || statuses[old].purpose != cmd.Purpose {
		return HGErrNotFound
	}
	res, err := tx.ExecContext(ctx, SQL.HGMediaReferenceDeleteSQL, cmd.BizType, cmd.BizID, cmd.Slot, old)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return HGErrConflict
	}
	return s.hgReleaseOld(ctx, tx, old)
}
