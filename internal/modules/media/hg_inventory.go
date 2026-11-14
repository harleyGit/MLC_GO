package media

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// HGInventoryItem 是管理端元数据盘点协议；旧来源只报告记录，不授予对象读取、引用或迁移权限。
type HGInventoryItem struct {
	Source  string `json:"source"`    // new 或四种 legacy_* 来源；每一来源独立按主键倒序分页。
	ID      string `json:"id"`        // 来源内主键（新资源 UUID、旧 IM 十六进制 ID 或旧表数字 ID），不可用于跨来源操作。
	Kind    string `json:"kind"`      // image/video/audio/file；由 MIME 或旧来源分类，不代表审核通过。
	Purpose string `json:"purpose"`   // 新媒体业务用途；旧来源为对应的 legacy_* 标记。
	MIME    string `json:"mime_type"` // 声明 MIME；旧头像/商品没有此字段时为空，不能据此预览。
	Size    uint64 `json:"file_size"` // 已登记字节数；旧头像/商品未知时为零，不表示空文件。
	Status  string `json:"status"`    // 新媒体状态或 legacy: 原来源状态；旧状态绝不等同新媒体 ready。
}

// HGInventoryPage 按固定来源顺序逐来源主键倒序游标分页；next_cursor 为空表示已遍历已知来源。
type HGInventoryPage struct {
	Items      []HGInventoryItem `json:"items"`       // 最多 50 条；仅含只读非敏感元数据。
	NextCursor string            `json:"next_cursor"` // 含类别、来源序号与上次主键；换类别必须重新从首页查询。
}

var hgInventorySources = [...]string{"new", "legacy_mall", "legacy_video", "legacy_avatar", "legacy_im"}

func hgInventoryKindValid(kind string) bool {
	return kind == "" || kind == "image" || kind == "video" || kind == "audio" || kind == "file"
}

func hgInventorySourceEnabled(source int, kind string) bool {
	switch source {
	case 1, 3:
		return kind == "" || kind == "image"
	case 2:
		return kind == "" || kind == "video"
	default:
		return true
	}
}

func hgInventoryCursor(kind, raw string) (int, string, error) {
	if raw == "" {
		return 0, "", nil
	}
	if len(raw) > 90 {
		return 0, "", HGErrInvalid
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 3 || parts[0] != kind {
		return 0, "", HGErrInvalid
	}
	source, err := strconv.Atoi(parts[1])
	if err != nil || source < 0 || source >= len(hgInventorySources) || !hgInventorySourceEnabled(source, kind) {
		return 0, "", HGErrInvalid
	}
	id := parts[2]
	switch source {
	case 0:
		if uuid.Validate(id) != nil || strings.ToLower(id) != id {
			return 0, "", HGErrInvalid
		}
	case 4:
		decoded, decodeErr := hex.DecodeString(id)
		if decodeErr != nil || len(decoded) != 16 || strings.ToUpper(id) != id {
			return 0, "", HGErrInvalid
		}
	default:
		n, parseErr := strconv.ParseUint(id, 10, 64)
		if parseErr != nil || n == 0 || strconv.FormatUint(n, 10) != id {
			return 0, "", HGErrInvalid
		}
	}
	return source, id, nil
}

// HGInventory 只查询已知的新媒体及四种旧媒体元数据；权限由路由上的 media:inventory:read 与活动管理员校验。
// 不关联旧业务主表，也不从旧 URL/对象键推测可见性；失败时不返回部分来源的结果。
func (s *HGService) HGInventory(ctx context.Context, kind, cursor string, limit int) (HGInventoryPage, error) {
	page := HGInventoryPage{Items: []HGInventoryItem{}}
	if s == nil || s.db == nil || !hgInventoryKindValid(kind) || limit < 1 || limit > 50 {
		return page, HGErrInvalid
	}
	source, bound, err := hgInventoryCursor(kind, cursor)
	if err != nil {
		return page, err
	}
	for ; source < len(hgInventorySources) && len(page.Items) <= limit; source++ {
		if !hgInventorySourceEnabled(source, kind) {
			bound = ""
			continue
		}
		found, queryErr := s.hgInventorySource(ctx, source, kind, bound, limit+1-len(page.Items))
		if queryErr != nil {
			return HGInventoryPage{}, queryErr
		}
		page.Items = append(page.Items, found...)
		bound = ""
	}
	if len(page.Items) > limit {
		last := page.Items[limit-1]
		for index, name := range hgInventorySources {
			if name == last.Source {
				page.NextCursor = kind + ":" + strconv.Itoa(index) + ":" + last.ID
				break
			}
		}
		page.Items = page.Items[:limit]
	}
	return page, nil
}

func (s *HGService) hgInventorySource(ctx context.Context, source int, kind, bound string, limit int) ([]HGInventoryItem, error) {
	var query string
	args := []any{}
	// BIGINT 游标以数值类型绑定，避免列与文本参数比较时的隐式转换。
	var numericBound uint64
	if bound != "" && source >= 1 && source <= 3 {
		var err error
		numericBound, err = strconv.ParseUint(bound, 10, 64)
		if err != nil {
			return nil, HGErrInvalid
		}
	}
	if source == 0 {
		if kind == "" {
			query = SQL.HGMediaInventoryNewAllFirstSQL
			if bound != "" {
				query = SQL.HGMediaInventoryNewAllSQL
				args = append(args, bound)
			}
		} else {
			query = SQL.HGMediaInventoryNewKindFirstSQL
			args = append(args, kind)
			if bound != "" {
				query = SQL.HGMediaInventoryNewKindSQL
				args = append(args, bound)
			}
		}
	} else if source == 1 {
		query = SQL.HGMediaInventoryMallFirstSQL
		if bound != "" {
			query = SQL.HGMediaInventoryMallSQL
			args = append(args, numericBound)
		}
	} else if source == 2 {
		query = SQL.HGMediaInventoryVideoFirstSQL
		if bound != "" {
			query = SQL.HGMediaInventoryVideoSQL
			args = append(args, numericBound)
		}
	} else if source == 3 {
		query = SQL.HGMediaInventoryAvatarFirstSQL
		if bound != "" {
			query = SQL.HGMediaInventoryAvatarSQL
			args = append(args, numericBound)
		}
	} else if kind == "" {
		query = SQL.HGMediaInventoryIMAllFirstSQL
		if bound != "" {
			query = SQL.HGMediaInventoryIMAllSQL
			args = append(args, bound)
		}
	} else {
		query = SQL.HGMediaInventoryIMKindFirstSQL
		args = append(args, kind)
		if bound != "" {
			query = SQL.HGMediaInventoryIMKindSQL
			args = append(args, bound)
		}
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGInventoryItem, 0, limit)
	for rows.Next() {
		item, scanErr := hgScanInventoryRow(rows, source)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func hgScanInventoryRow(rows *sql.Rows, source int) (HGInventoryItem, error) {
	item := HGInventoryItem{Source: hgInventorySources[source]}
	switch source {
	case 0:
		if err := rows.Scan(&item.ID, &item.Purpose, &item.MIME, &item.Size, &item.Status); err != nil {
			return item, err
		}
		item.Kind = hgInventoryMIMEKind(item.MIME)
	case 1:
		var id uint64
		var status int
		if err := rows.Scan(&id, &status); err != nil {
			return item, err
		}
		item.ID, item.Kind, item.Purpose, item.Status = strconv.FormatUint(id, 10), "image", "legacy_mall", "legacy:"+strconv.Itoa(status)
	case 2:
		var id uint64
		var status string
		if err := rows.Scan(&id, &item.MIME, &item.Size, &status); err != nil {
			return item, err
		}
		item.ID, item.Kind, item.Purpose, item.Status = strconv.FormatUint(id, 10), "video", "legacy_video", "legacy:"+status
	case 3:
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return item, err
		}
		item.ID, item.Kind, item.Purpose, item.Status = strconv.FormatUint(id, 10), "image", "legacy_avatar", "legacy:unverified"
	case 4:
		var status string
		if err := rows.Scan(&item.ID, &item.Kind, &item.MIME, &item.Size, &status); err != nil {
			return item, err
		}
		item.Purpose, item.Status = "legacy_im", "legacy:"+status
	}
	return item, nil
}

func hgInventoryMIMEKind(mime string) string {
	for _, kind := range []string{"image", "video", "audio"} {
		if strings.HasPrefix(mime, kind+"/") {
			return kind
		}
	}
	return "file"
}
