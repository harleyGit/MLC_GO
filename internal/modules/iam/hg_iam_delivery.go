package iam

import (
	Notification "MLC_GO/internal/modules/notification"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// HGUserPublisher 由 IM 网关注入；只允许服务端选择本人频道，不能接受客户端指定频道。
type HGUserPublisher interface {
	HGPublishUser(context.Context, string, json.RawMessage) error
}

// HGAccessDelivery 从事务性工作队列按角色游标投递；多 Pod 通过数据库短租约互斥。
type HGAccessDelivery struct {
	db        *sql.DB                 // 权威 MySQL 连接池；不由工作者关闭。
	iam       *HGService              // 读取当前授权快照，禁止把旧事件作为权限依据。
	notices   *Notification.HGService // 幂等站内通知；失败时保留任务重试。
	publisher HGUserPublisher         // 用户专属 IM 频道；失败时保留任务重试。
}

// HGNewAccessDelivery 复用现有 MySQL、IAM 和通知模块；网关未配置时不允许处理任务。
func HGNewAccessDelivery(db *sql.DB, iam *HGService, notices *Notification.HGService, publisher HGUserPublisher) *HGAccessDelivery {
	return &HGAccessDelivery{db: db, iam: iam, notices: notices, publisher: publisher}
}

type hgDeliveryJob struct {
	id     uint64 // 稳定主键，事件 ID 与此绑定，不使用递增 ID 推断提交顺序。
	kind   string // user/role，两者以外的值拒绝处理。
	target string // 当前用户 ID 或角色 ID。
	name   string // 审计时角色名/编码，可为空；发送前以角色表为准。
	cursor string // role 用户游标，空串表示首页。
	token  string // 独占租约随机数；过期工作者不能推进游标。
}

// HGDispatchOnce 每次最多投递 50 个用户；网络 I/O 不在数据库事务中，失败留给有界重试。
func (worker *HGAccessDelivery) HGDispatchOnce(ctx context.Context) error {
	if worker == nil || worker.db == nil || worker.iam == nil || worker.notices == nil || worker.publisher == nil {
		return errors.New("IAM 投递依赖未就绪")
	}
	job, err := worker.hgClaim(ctx)
	if err != nil || job == nil {
		return err
	}
	users, more, err := worker.hgUsers(ctx, job)
	if err == nil && len(users) > 0 {
		var title, body string
		title, body, err = worker.hgTemplate(ctx)
		if err == nil {
			for _, user := range users {
				if err = worker.hgDeliverUser(ctx, job, user, title, body); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		_, _ = worker.db.ExecContext(ctx, SQL.HGRetryIAMDeliveryJob, job.id, job.token)
		return err
	}
	next := job.cursor
	if len(users) != 0 {
		next = users[len(users)-1]
	}
	status := "done"
	if more {
		status = "pending"
	}
	result, err := worker.db.ExecContext(ctx, SQL.HGFinishIAMDeliveryJob, next, status, job.id, job.token, job.cursor)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return errors.New("IAM 投递租约失效，等待其他工作者续投")
	}
	return nil
}

func (worker *HGAccessDelivery) hgClaim(ctx context.Context) (*hgDeliveryJob, error) {
	tx, err := worker.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job := new(hgDeliveryJob)
	err = tx.QueryRowContext(ctx, SQL.HGClaimIAMDeliveryJob).Scan(&job.id, &job.kind, &job.target, &job.name, &job.cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	token := make([]byte, 16)
	if _, err = rand.Read(token); err != nil {
		return nil, err
	}
	job.token = hex.EncodeToString(token)
	if _, err = tx.ExecContext(ctx, SQL.HGLeaseIAMDeliveryJob, job.token, job.id); err != nil {
		return nil, err
	}
	return job, tx.Commit()
}

func (worker *HGAccessDelivery) hgUsers(ctx context.Context, job *hgDeliveryJob) ([]string, bool, error) {
	switch job.kind {
	case "user":
		if !hgValidText(job.target, 64) {
			return nil, false, ErrHGInvalid
		}
		return []string{job.target}, false, nil
	case "role":
		id, err := strconv.ParseUint(job.target, 10, 64)
		if err != nil || id == 0 {
			return nil, false, ErrHGInvalid
		}
		rows, err := worker.db.QueryContext(ctx, SQL.HGRoleDeliveryUsers, id, job.cursor)
		if err != nil {
			return nil, false, err
		}
		defer rows.Close()
		users := make([]string, 0, 51)
		for rows.Next() {
			var user string
			if err = rows.Scan(&user); err != nil {
				return nil, false, err
			}
			users = append(users, user)
		}
		if err = rows.Err(); err != nil {
			return nil, false, err
		}
		more := len(users) > 50
		if more {
			users = users[:50]
		}
		return users, more, nil
	default:
		return nil, false, ErrHGInvalid
	}
}

func (worker *HGAccessDelivery) hgTemplate(ctx context.Context) (string, string, error) {
	var title, body string
	var schema, channels []byte
	err := worker.db.QueryRowContext(ctx, SQL.HGNotificationRolePublishedSQL).Scan(&title, &body, &schema, &channels)
	if errors.Is(err, sql.ErrNoRows) {
		// 未配置模板不能阻塞权限撤销；使用受控纯文本兜底，仍先幂等落库再推送。
		return "角色权限已更新", "您的角色 {{role_name}} 权限已更新，请重新查看当前权限。", nil
	}
	if err != nil {
		return "", "", err
	}
	if err := Notification.HGValidateRolePublished(title, body, schema, channels); err != nil {
		return "", "", err
	}
	return title, body, nil
}

func (worker *HGAccessDelivery) hgDeliverUser(ctx context.Context, job *hgDeliveryJob, user, title, body string) error {
	name := job.name
	if job.kind == "role" {
		id, _ := strconv.ParseUint(job.target, 10, 64)
		if err := worker.db.QueryRowContext(ctx, SQL.HGRoleDeliveryName, id).Scan(&name); err != nil {
			return err
		}
	} else if name != "" {
		if err := worker.db.QueryRowContext(ctx, SQL.HGUserDeliveryName, name).Scan(&name); err != nil {
			return err
		}
	}
	if name == "" {
		name = "角色"
	}
	name = strings.NewReplacer("<", " ", ">", " ").Replace(name)
	content := strings.ReplaceAll(body, "{{role_name}}", name)
	hash := sha256.Sum256([]byte(user))
	eventID := fmt.Sprintf("iam-%d-%x", job.id, hash[:8])
	if err := worker.notices.HGDeliver(ctx, eventID, user, Notification.HGNotice{
		Category: "system", Title: title, Content: content,
	}); err != nil {
		return err
	}
	snapshot, err := worker.iam.AccessSnapshot(ctx, user)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"event_type": "iam.access.updated.v1", "snapshot": snapshot,
		"notice":   map[string]string{"category": "system", "title": title, "content": content},
		"event_id": eventID,
	})
	if err != nil {
		return err
	}
	return worker.publisher.HGPublishUser(ctx, user, payload)
}
