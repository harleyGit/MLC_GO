package send

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

var hgUserID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
var hgRequestID = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

// HGCommand 是运维发送协议；仅支持至多 100 名明确用户的立即站内发送。
type HGCommand struct {
	TemplateID      string            `json:"templateId"`      // 十进制模板主键。
	TemplateVersion uint32            `json:"templateVersion"` // 当前发布版版本号。
	UserIDs         []string          `json:"userIds"`         // 明确指定的 1-100 个用户 ID。
	Variables       map[string]string `json:"variables"`       // 全体接收人共享的字符串变量。
}

// HGTask 是任务回执；pending 表示已持久化，不代表已送达。
type HGTask struct {
	ID              string    `json:"taskId"`          // 十进制任务 ID。
	TemplateID      string    `json:"templateId"`      // 模板主键。
	TemplateVersion uint32    `json:"templateVersion"` // 创建任务时绑定的不可变版本。
	Status          string    `json:"status"`          // pending/leased/done/failed/cancelled。
	Total           int       `json:"total"`           // 原始接收人数，最多 100。
	Processed       int       `json:"processed"`       // 已处理人数；营销偏好拒收也计入，不代表实际入箱。
	Attempts        uint8     `json:"attempts"`        // 当前用户的连续失败次数。
	CreatedAt       time.Time `json:"createdAt"`       // 数据库保存的创建时间。
}

// HGRepository 使用共享数据库保存事务任务与租约；不接触网络或另建连接池。
type HGRepository struct {
	db *sql.DB // 不持有连接池关闭权。
}

// HGNewRepository 复用已有 MySQL 与站内通知权威存储。
func HGNewRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

func hgID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 || id > math.MaxInt64 {
		return 0, HGErrInvalid
	}
	return id, nil
}

func hgScanVersion(row interface{ Scan(...any) error }) (HGVersion, error) {
	var v HGVersion
	err := row.Scan(&v.Category, &v.Title, &v.Body, &v.Schema, &v.Action, &v.Config, &v.Channels)
	return v, err
}

// HGPublish 以版本锁与双人复核保护可发送模板；不改变 iam.role.updated 的既有发布入口。
func (repo *HGRepository) HGPublish(ctx context.Context, id uint64, version uint32, reviewer string) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var latest uint32
	var status, code, author string
	var v HGVersion
	err = tx.QueryRowContext(ctx, queries.HGNotificationSendPublishLockSQL, id).Scan(
		&latest, &status, &code, &author, &v.Category, &v.Title, &v.Body, &v.Schema, &v.Action, &v.Config, &v.Channels)
	if errors.Is(err, sql.ErrNoRows) {
		return HGErrMissing
	}
	if err != nil {
		return err
	}
	if latest != version || code == "iam.role.updated" || status == "deleted" || author == reviewer || reviewer == "" {
		return HGErrConflict
	}
	if err := HGValidateVersion(v); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, queries.HGNotificationSendPublishVersionSQL, id, version)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return HGErrConflict
	}
	result, err = tx.ExecContext(ctx, queries.HGNotificationTemplatePublishSQL, version, reviewer, id, version)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return HGErrConflict
	}
	return tx.Commit()
}

func hgPrepare(cmd *HGCommand) (uint64, []byte, []byte, error) {
	id, err := hgID(cmd.TemplateID)
	if err != nil || cmd.TemplateVersion == 0 || len(cmd.UserIDs) < 1 || len(cmd.UserIDs) > 100 || len(cmd.Variables) > 30 {
		return 0, nil, nil, HGErrInvalid
	}
	for _, user := range cmd.UserIDs {
		if !hgUserID.MatchString(user) {
			return 0, nil, nil, HGErrInvalid
		}
	}
	slices.Sort(cmd.UserIDs)
	for i := 1; i < len(cmd.UserIDs); i++ {
		if cmd.UserIDs[i] == cmd.UserIDs[i-1] {
			return 0, nil, nil, HGErrInvalid
		}
	}
	audience, err := json.Marshal(cmd.UserIDs)
	if err != nil {
		return 0, nil, nil, err
	}
	variables, err := json.Marshal(cmd.Variables)
	if err != nil || len(variables) > 8192 {
		return 0, nil, nil, HGErrInvalid
	}
	return id, audience, variables, nil
}

// HGCreate 在一个短事务里验证已发布版与接收人并插入持久任务；唯一请求键防并发重复提交。
func (repo *HGRepository) HGCreate(ctx context.Context, requestID, creator string, cmd HGCommand) (string, error) {
	if !hgRequestID.MatchString(requestID) || !hgUserID.MatchString(creator) {
		return "", HGErrInvalid
	}
	id, audience, variables, err := hgPrepare(&cmd)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%s:%s", creator, id, cmd.TemplateVersion, audience, variables)))
	hashText := hex.EncodeToString(hash[:])
	var existing uint64
	var priorHash string
	err = repo.db.QueryRowContext(ctx, queries.HGNotificationSendByRequestSQL, requestID).Scan(&existing, &priorHash)
	if err == nil {
		if priorHash != hashText {
			return "", HGErrConflict
		}
		return strconv.FormatUint(existing, 10), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	v, err := hgScanVersion(tx.QueryRowContext(ctx, queries.HGNotificationSendTemplateSQL, cmd.TemplateVersion, id))
	if errors.Is(err, sql.ErrNoRows) {
		return "", HGErrMissing
	}
	if err != nil {
		return "", err
	}
	if _, err := HGRender(ctx, v, cmd.Variables); err != nil {
		return "", err
	}
	// 有界 IN 命中 users.user_id 唯一键；不存在或并发删除的 ID 不得生成永远无法投递的任务。
	args := make([]any, len(cmd.UserIDs))
	for i, user := range cmd.UserIDs {
		args[i] = user
	}
	rows, err := tx.QueryContext(ctx, queries.HGNotificationSendUsersSQL(len(args)), args...)
	if err != nil {
		return "", err
	}
	found := 0
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			break
		}
		found++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return "", err
	}
	if found != len(args) {
		return "", HGErrInvalid
	}
	var adminID uint64
	if err := tx.QueryRowContext(ctx, queries.HGNotificationSendAdminLockSQL, creator).Scan(&adminID); errors.Is(err, sql.ErrNoRows) {
		return "", HGErrInvalid
	} else if err != nil {
		return "", err
	}
	var pending int
	if err := tx.QueryRowContext(ctx, queries.HGNotificationSendPendingLimitSQL, creator).Scan(&pending); err != nil {
		return "", err
	}
	if pending >= 20 {
		return "", HGErrBusy
	}
	result, err := tx.ExecContext(ctx, queries.HGNotificationSendInsertSQL, requestID, hashText, id, cmd.TemplateVersion, audience, variables, creator)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = tx.Rollback()
			if err := repo.db.QueryRowContext(ctx, queries.HGNotificationSendByRequestSQL, requestID).Scan(&existing, &priorHash); err != nil {
				return "", err
			}
			if priorHash != hashText {
				return "", HGErrConflict
			}
			return strconv.FormatUint(existing, 10), nil
		}
		return "", err
	}
	taskID, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return strconv.FormatInt(taskID, 10), nil
}

// HGDetail 只向任务创建者展示统计，不暴露其他管理员的接收人列表。
func (repo *HGRepository) HGDetail(ctx context.Context, id uint64, creator string) (HGTask, error) {
	var task HGTask
	var raw []byte
	var taskID, templateID uint64
	err := repo.db.QueryRowContext(ctx, queries.HGNotificationSendDetailSQL, id, creator).Scan(
		&taskID, &templateID, &task.TemplateVersion, &raw, &task.Status, &task.Processed, &task.Attempts, &task.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return task, HGErrMissing
	}
	if err != nil {
		return task, err
	}
	var users []string
	if err := json.Unmarshal(raw, &users); err != nil {
		return task, err
	}
	task.ID, task.TemplateID, task.Total = strconv.FormatUint(taskID, 10), strconv.FormatUint(templateID, 10), len(users)
	return task, nil
}

// HGMinePage 仅列出创建人自己的任务，返回稳定倒序主键游标；不暴露变量或接收人。
func (repo *HGRepository) HGMinePage(ctx context.Context, creator string, cursor uint64) ([]HGTask, string, error) {
	if !hgUserID.MatchString(creator) || cursor == 0 || cursor > math.MaxInt64 {
		return nil, "", HGErrInvalid
	}
	rows, err := repo.db.QueryContext(ctx, queries.HGNotificationSendMinePageSQL, creator, cursor, 21)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]HGTask, 0, 21)
	for rows.Next() {
		var item HGTask
		var taskID, templateID uint64
		if err := rows.Scan(&taskID, &templateID, &item.TemplateVersion, &item.Status,
			&item.Processed, &item.Attempts, &item.CreatedAt, &item.Total); err != nil {
			return nil, "", err
		}
		item.ID, item.TemplateID = strconv.FormatUint(taskID, 10), strconv.FormatUint(templateID, 10)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) > 20 {
		items = items[:20]
		return items, items[len(items)-1].ID, nil
	}
	return items, "", nil
}

// HGCancel 仅创建者能取消自己的未完成任务；已落库站内通知不撤回。
func (repo *HGRepository) HGCancel(ctx context.Context, id uint64, creator string) error {
	result, err := repo.db.ExecContext(ctx, queries.HGNotificationSendCancelSQL, id, creator)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return HGErrConflict
	}
	return nil
}
