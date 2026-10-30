package notification

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// HGNotice 是站内通知快照协议；ID 为单调游标，不包含私信内容。
type HGNotice struct {
	NotificationID string    `json:"notificationId"` // 站内通知 ID，十进制游标。
	Category       string    `json:"category"`       // 通知分类，取值为受限业务枚举。
	Title          string    `json:"title"`          // 渲染后的纯文本标题。
	Content        string    `json:"content"`        // 渲染后的纯文本正文，不支持 HTML。
	TargetType     string    `json:"targetType"`     // 跳转资源类型，空串表示无目标。
	TargetID       string    `json:"targetId"`       // 资源标识，空串表示无目标。
	IsRead         bool      `json:"isRead"`         // 当前用户的已读状态。
	CreatedAt      time.Time `json:"createdAt"`      // UTC 创建时间。
}

// HGRepository 仅通过用户 ID 和有界游标访问权威 MySQL。
type HGRepository struct{ db *sql.DB }

// HGNewRepository 创建通知存储；调用方持有连接池生命周期。
func HGNewRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

// HGPage 按主键倒序分页，额外获取一条判断是否有下一页。
func (repo *HGRepository) HGPage(ctx context.Context, user, category string, cursor uint64, size int, unread bool) ([]HGNotice, string, error) {
	query := SQLQueriesPackage.HGNotificationPageSQL
	if unread {
		query = SQLQueriesPackage.HGNotificationUnreadPageSQL
	}
	rows, err := repo.db.QueryContext(ctx, query, user, category, cursor, size+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]HGNotice, 0, size)
	for rows.Next() {
		var item HGNotice
		var id uint64
		var read bool
		if err = rows.Scan(&id, &item.Category, &item.Title, &item.Content, &item.TargetType, &item.TargetID, &read, &item.CreatedAt); err != nil {
			return nil, "", err
		}
		item.NotificationID = fmt.Sprint(id)
		item.IsRead = read
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) <= size {
		return items, "", nil
	}
	return items[:size], items[size-1].NotificationID, nil
}

// HGUnread 按分类从 MySQL 权威未读索引汇总，不依赖不可恢复的缓存。
func (repo *HGRepository) HGUnread(ctx context.Context, user string) (map[string]int64, error) {
	rows, err := repo.db.QueryContext(ctx, SQLQueriesPackage.HGNotificationUnreadSQL, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int64)
	for rows.Next() {
		var category string
		var count int64
		if err = rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		counts[category] = count
	}
	return counts, rows.Err()
}

// HGDelete 仅修改 JWT 接收人的一条记录，跨用户 ID 无法命中。

// HGReadIDs 一次带绑定参数更新最多 50 条，避免逐条发送 SQL 请求。
func (repo *HGRepository) HGReadIDs(ctx context.Context, user string, ids []uint64) error {
	query := SQLQueriesPackage.HGNotificationReadIDsSQL(len(ids))
	if query == "" {
		return HGErrInvalid
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, user)
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := repo.db.ExecContext(ctx, query, args...)
	return err
}
func (repo *HGRepository) HGDelete(ctx context.Context, user string, id uint64) error {
	_, err := repo.db.ExecContext(ctx, SQLQueriesPackage.HGNotificationDeleteSQL, user, id)
	return err
}

// HGReadBatch 每次最多更新 500 条；返回 true 表示仍可能有剩余，需要由调用方再次提交。
func (repo *HGRepository) HGReadBatch(ctx context.Context, user, category string) (bool, error) {
	var high uint64
	if err := repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGNotificationHighWaterSQL, user, category).Scan(&high); err != nil {
		return false, err
	}
	if high == 0 {
		return false, nil
	}
	result, err := repo.db.ExecContext(ctx, SQLQueriesPackage.HGNotificationReadBatchSQL, user, category, high)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 500, err
}

// HGPreferences 获取用户偏好；无记录时使用服务端默认值。
func (repo *HGRepository) HGPreferences(ctx context.Context, user string) (HGPreferences, error) {
	var data []byte
	err := repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGNotificationPreferencesSQL, user).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return hgDefaultPreferences(), nil
	}
	if err != nil {
		return HGPreferences{}, err
	}
	var preferences HGPreferences
	if err = json.Unmarshal(data, &preferences); err != nil {
		return HGPreferences{}, err
	}
	return preferences, nil
}

// HGSavePreferences 保存经业务层校验的用户配置，不覆盖其他用户。
func (repo *HGRepository) HGSavePreferences(ctx context.Context, user string, preferences HGPreferences) error {
	data, err := json.Marshal(preferences)
	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, SQLQueriesPackage.HGNotificationPreferencesUpsertSQL, user, data)
	return err
}

// HGDeliver 由可信业务调用，eventID 必须稳定；唯一键实现同一事件同用户同分类的幂等。
func (repo *HGRepository) HGDeliver(ctx context.Context, eventID, user string, notice HGNotice) error {
	_, err := repo.db.ExecContext(ctx, SQLQueriesPackage.HGNotificationInsertSQL, eventID, user, notice.Category, notice.Title, notice.Content, notice.TargetType, notice.TargetID)
	return err
}

// HGDeliveries 运维按主键游标读取最近的站内成功记录，不宣称外部渠道已投递。
func (repo *HGRepository) HGDeliveries(ctx context.Context, cursor uint64, size int) ([]map[string]any, string, error) {
	rows, err := repo.db.QueryContext(ctx, SQLQueriesPackage.HGNotificationDeliveryPageSQL, cursor, size+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, size+1)
	for rows.Next() {
		var id uint64
		var eventID, user, category string
		var at time.Time
		if err = rows.Scan(&id, &eventID, &user, &category, &at); err != nil {
			return nil, "", err
		}
		items = append(items, map[string]any{"deliveryId": fmt.Sprint(id), "eventId": eventID, "receiverId": user, "channel": "in_app", "templateCode": category, "templateVersion": 1, "status": "sent", "retryCount": 0, "createdAt": at})
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) <= size {
		return items, "", nil
	}
	return items[:size], items[size-1]["deliveryId"].(string), nil
}
