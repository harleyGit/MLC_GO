package send

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"
)

// HGReview 是待审批任务的只读展示协议；不暴露接收用户或任务变量。
type HGReview struct {
	TaskID          string     `json:"taskId"`                // 十进制任务主键。
	TemplateID      string     `json:"templateId"`            // 模板主键。
	TemplateVersion uint32     `json:"templateVersion"`       // 不可变已发布版本。
	Status          string     `json:"status"`                // 固定 pending_approval。
	Total           int        `json:"total"`                 // 最多 100 名接收人。
	Title           string     `json:"title,omitempty"`       // 服务端渲染后的最终标题；列表省略。
	Body            string     `json:"body,omitempty"`        // 服务端渲染后的最终正文；列表省略。
	Category        string     `json:"category,omitempty"`    // system/marketing，列表省略。
	ScheduledAt     *time.Time `json:"scheduledAt,omitempty"` // UTC 计划发送时间；nil 表示审批后立即。
	CreatedAt       time.Time  `json:"createdAt"`             // 列表记录的创建时间。
}

// HGReviewPage 按状态与主键有界分页，排除审批人自己创建的任务。
func (repo *HGRepository) HGReviewPage(ctx context.Context, reviewer string, cursor uint64) ([]HGReview, string, error) {
	if !hgUserID.MatchString(reviewer) || cursor == 0 || cursor > math.MaxInt64 {
		return nil, "", HGErrInvalid
	}
	rows, err := repo.db.QueryContext(ctx, queries.HGNotificationSendReviewPageSQL, cursor, reviewer, 21)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]HGReview, 0, 21)
	for rows.Next() {
		var item HGReview
		var id, template uint64
		var processed int
		var attempts uint8
		if err := rows.Scan(&id, &template, &item.TemplateVersion, &item.Status,
			&processed, &attempts, &item.CreatedAt, &item.Total); err != nil {
			return nil, "", err
		}
		item.TaskID, item.TemplateID = strconv.FormatUint(id, 10), strconv.FormatUint(template, 10)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) > 20 {
		items = items[:20]
		return items, items[len(items)-1].TaskID, nil
	}
	return items, "", nil
}

// HGReviewDetail 读取已发布版本的文案供另一名管理员复核；不含用户 ID 和变量值。
func (repo *HGRepository) HGReviewDetail(ctx context.Context, id uint64, reviewer string) (HGReview, error) {
	var item HGReview
	if !hgUserID.MatchString(reviewer) || id == 0 {
		return item, HGErrInvalid
	}
	var task, template uint64
	var scheduled sql.NullTime
	var v HGVersion
	var raw []byte
	err := repo.db.QueryRowContext(ctx, queries.HGNotificationSendReviewDetailSQL, id, reviewer).Scan(
		&task, &template, &item.TemplateVersion, &item.Status, &item.Total, &item.CreatedAt,
		&v.Category, &v.Title, &v.Body, &v.Schema, &v.Action, &v.Config, &v.Channels, &raw, &scheduled)
	if errors.Is(err, sql.ErrNoRows) {
		return item, HGErrMissing
	}
	if err != nil {
		return item, err
	}
	var vars map[string]string
	if err := json.Unmarshal(raw, &vars); err != nil {
		return item, err
	}
	rendered, err := HGRender(ctx, v, vars)
	if err != nil {
		return item, err
	}
	item.Title, item.Body, item.Category = rendered.Title, rendered.Content, rendered.Category
	item.TaskID, item.TemplateID = strconv.FormatUint(task, 10), strconv.FormatUint(template, 10)
	if scheduled.Valid {
		value := scheduled.Time.UTC()
		item.ScheduledAt = &value
	}
	return item, nil
}

// HGApprove 单任务行锁内校验双人复核，状态更新与审计同一事务提交。
func (repo *HGRepository) HGApprove(ctx context.Context, id uint64, reviewer string, audit ...HGAuditContext) error {
	if id == 0 || !hgUserID.MatchString(reviewer) {
		return HGErrInvalid
	}
	meta, err := hgAuditContext(audit)
	if err != nil {
		return err
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var creator string
	var template uint64
	var version uint32
	var scheduled sql.NullTime
	err = tx.QueryRowContext(ctx, queries.HGNotificationSendApproveLockSQL, id).
		Scan(&creator, &template, &version, &scheduled)
	if errors.Is(err, sql.ErrNoRows) {
		return HGErrConflict
	}
	if err != nil {
		return err
	}
	if creator == reviewer {
		return HGErrConflict
	}
	// 发布状态在审批时重新确认；后台 worker 也会再次校验该不可变版本。
	var bound HGVersion
	bound, err = hgScanVersion(tx.QueryRowContext(ctx, queries.HGNotificationSendBoundVersionSQL, version, template))
	if errors.Is(err, sql.ErrNoRows) {
		return HGErrConflict
	}
	if err != nil {
		return err
	}
	if err = HGValidateVersion(bound); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, queries.HGNotificationSendApproveSQL, reviewer, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return HGErrConflict
	}
	if _, err = tx.ExecContext(ctx, queries.HGNotificationSendAuditSQL, id, "approved", reviewer, version,
		meta.RequestID, meta.SourceIP); err != nil {
		return err
	}
	return tx.Commit()
}
