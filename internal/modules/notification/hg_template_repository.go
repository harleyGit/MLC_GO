package notification

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
)

var (
	HGErrTemplateMissing  = errors.New("模板不存在")
	HGErrTemplateConflict = errors.New("模板版本冲突")
)

// HGTemplate 表示管理端模板及其最新不可变草稿快照；ID 以字符串传输以避免 JS 精度丢失。
type HGTemplate struct {
	ID               string          `json:"id"`                // 数据库主键十进制字符串；空值不合法。
	TemplateCode     string          `json:"template_code"`     // 全局唯一业务编码；创建后不可改。
	Name             string          `json:"name"`              // 最新版本的展示名称；不能为空。
	Category         string          `json:"category"`          // 固定分类枚举。
	Status           string          `json:"status"`            // 模板主表状态；删除记录不对管理列表展示。
	Version          uint32          `json:"version"`           // 最新版本号，更新与删除须携带以防覆盖。
	PublishedVersion *uint32         `json:"published_version"` // 已发布版本；nil 表示从未发布。
	TitleTemplate    string          `json:"title_template"`    // 纯文本标题，允许受控变量。
	BodyTemplate     string          `json:"body_template"`     // 纯文本正文，不执行 HTML。
	VariablesSchema  json.RawMessage `json:"variables_schema"`  // JSON 变量声明；默认空对象。
	ActionType       string          `json:"action_type"`       // 受控跳转类型，none 表示无跳转。
	ActionConfig     json.RawMessage `json:"action_config"`     // 受控路由参数；默认空对象。
	Channels         json.RawMessage `json:"channels"`          // 发送渠道 JSON 数组，不代表发送已接入。
	Priority         uint8           `json:"priority"`          // 优先级 1-3，缺省 2。
	Locale           string          `json:"locale"`            // 语言区域，当前仅 zh-CN。
}

// HGTemplateRepository 只管理模板表，不触碰既有收件箱及通知投递链。
type HGTemplateRepository struct{ db *sql.DB /* 复用工程共享 MySQL 连接池，不由模板仓储关闭。 */ }

// HGNewTemplateRepository 使用既有 MySQL 连接池。
func HGNewTemplateRepository(db *sql.DB) *HGTemplateRepository { return &HGTemplateRepository{db: db} }

func hgScanTemplate(rows interface{ Scan(...any) error }) (HGTemplate, error) {
	var item HGTemplate
	var id uint64
	var version uint32
	var published sql.NullInt64
	var schema, config, channels []byte
	err := rows.Scan(&id, &item.TemplateCode, &item.Name, &item.Category, &item.Status, &version, &published,
		&item.TitleTemplate, &item.BodyTemplate, &schema, &item.ActionType, &config, &channels, &item.Priority, &item.Locale)
	if err != nil {
		return item, err
	}
	item.ID = hgTemplateID(id)
	item.Version = version
	if published.Valid {
		value := uint32(published.Int64)
		item.PublishedVersion = &value
	}
	item.VariablesSchema, item.ActionConfig, item.Channels = schema, config, channels
	return item, nil
}

func hgTemplateID(id uint64) string { return strconv.FormatUint(id, 10) }

// HGPage 使用倒序主键游标，pageSize 限制由 HTTP 入口校验。
func (repo *HGTemplateRepository) HGPage(ctx context.Context, cursor uint64, pageSize int) ([]HGTemplate, string, error) {
	rows, err := repo.db.QueryContext(ctx, queries.HGNotificationTemplatePageSQL, cursor, pageSize+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]HGTemplate, 0, pageSize)
	for rows.Next() {
		item, scanErr := hgScanTemplate(rows)
		if scanErr != nil {
			return nil, "", scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) > pageSize {
		items = items[:pageSize]
		return items, items[len(items)-1].ID, nil
	}
	return items, "", nil
}

// HGDetail 按主键读取最新版本快照，已删除记录不可见。
func (repo *HGTemplateRepository) HGDetail(ctx context.Context, id uint64) (HGTemplate, error) {
	item, err := hgScanTemplate(repo.db.QueryRowContext(ctx, queries.HGNotificationTemplateDetailSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		return item, HGErrTemplateMissing
	}
	return item, err
}

// HGCreate 在同一短事务写主表和首个不可变版本；唯一编码由数据库约束兜底。
func (repo *HGTemplateRepository) HGCreate(ctx context.Context, form HGTemplateForm, user string) (uint64, error) {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, queries.HGNotificationTemplateInsertSQL, form.TemplateCode, form.Name, form.Category, "platform", "", user, user)
	if err != nil {
		return 0, hgTemplateDBError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, queries.HGNotificationTemplateVersionInsertSQL, id, 1, form.Name, form.Category, form.TitleTemplate, form.BodyTemplate,
		form.VariablesSchema, form.ActionType, form.ActionConfig, form.Channels, form.Priority, form.Locale, user); err != nil {
		return 0, err
	}
	return uint64(id), tx.Commit()
}

// HGUpdate 锁定单个模板，验证客户端版本，再创建新版本；发布版本始终不可变。
func (repo *HGTemplateRepository) HGUpdate(ctx context.Context, id uint64, expected uint32, form HGTemplateForm, user string) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version uint32
	var status string
	var published sql.NullInt64
	err = tx.QueryRowContext(ctx, queries.HGNotificationTemplateLockSQL, id).Scan(&version, &status, &published)
	if errors.Is(err, sql.ErrNoRows) || status == "deleted" {
		return HGErrTemplateMissing
	}
	if err != nil {
		return err
	}
	if version != expected || version == math.MaxUint32 {
		return HGErrTemplateConflict
	}
	if _, err = tx.ExecContext(ctx, queries.HGNotificationTemplateVersionInsertSQL, id, version+1, form.Name, form.Category, form.TitleTemplate, form.BodyTemplate,
		form.VariablesSchema, form.ActionType, form.ActionConfig, form.Channels, form.Priority, form.Locale, user); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, queries.HGNotificationTemplateUpdateSQL, form.Name, form.Category, version+1, user, id); err != nil {
		return err
	}
	return tx.Commit()
}

// HGDelete 仅允许未发布模板软删除；有已发布版本的模板须走单独停用与审核流程。
func (repo *HGTemplateRepository) HGDelete(ctx context.Context, id uint64, expected uint32, user string) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version uint32
	var status string
	var published sql.NullInt64
	err = tx.QueryRowContext(ctx, queries.HGNotificationTemplateLockSQL, id).Scan(&version, &status, &published)
	if errors.Is(err, sql.ErrNoRows) || status == "deleted" {
		return HGErrTemplateMissing
	}
	if err != nil {
		return err
	}
	if version != expected || published.Valid || status != "draft" {
		return HGErrTemplateConflict
	}
	if _, err = tx.ExecContext(ctx, queries.HGNotificationTemplateDeleteSQL, user, id); err != nil {
		return err
	}
	return tx.Commit()
}
