package VideoDanmakuRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/go-sql-driver/mysql"
)

var ErrVideoNotDanmakuEnabled = errors.New("video is unavailable or danmaku is disabled")
var ErrIdempotencyConflict = errors.New("danmaku idempotency request conflicts with existing command")

// HGCreateCommand 是服务校验后的权威弹幕写入命令。
type HGCreateCommand struct {
	DanmakuID, VideoID, UserID, RequestID, Content, Mode, Color string
	ProgressMS                                                  uint32
	FontSize                                                    uint8
}

// HGCursor 保存时间线 keyset 排序元组。
type HGCursor struct {
	ProgressMS uint32
	ID         uint64
}

// HGDanmaku 是 repository 与 service 之间的弹幕业务模型。
type HGDanmaku struct {
	ID                                                             uint64
	DanmakuID, SubmissionID, VideoID, UserID, Content, Mode, Color string
	ProgressMS                                                     uint32
	FontSize                                                       uint8
	CreatedAt                                                      time.Time
}

// HGListResult 返回有界结果及固定 64 分片聚合总数。
type HGListResult struct {
	Danmaku    []HGDanmaku
	TotalCount uint64
}

// Repository 负责视频弹幕 MySQL 权威读写。
type Repository struct{ db *sql.DB }

// NewRepository 创建复用共享连接池的弹幕仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// ResolveVideo 验证视频已发布、公开且未关闭弹幕。
func (r *Repository) ResolveVideo(ctx context.Context, videoID string) (string, error) {
	var resolved, submissionID string
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectDanmakuVideoTargetSQL, videoID).Scan(&resolved, &submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrVideoNotDanmakuEnabled
		}
		return "", fmt.Errorf("resolve danmaku video: %w", err)
	}
	return submissionID, nil
}

// Create 在短事务内验证视频、幂等写入并更新 64 分片计数；提交后数据才允许广播。
func (r *Repository) Create(ctx context.Context, command HGCreateCommand) (HGDanmaku, bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGDanmaku{}, false, fmt.Errorf("begin danmaku transaction: %w", err)
	}
	defer tx.Rollback()
	var videoID, submissionID string
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.SelectDanmakuVideoTargetSQL, command.VideoID).Scan(&videoID, &submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGDanmaku{}, false, ErrVideoNotDanmakuEnabled
		}
		return HGDanmaku{}, false, fmt.Errorf("verify danmaku video: %w", err)
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoDanmakuSQL, command.DanmakuID, submissionID, videoID, command.UserID, command.RequestID, command.ProgressMS, command.Content, command.Mode, command.Color, command.FontSize)
	var item HGDanmaku
	created := false
	if hgDuplicate(err) {
		item, err = hgScan(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoDanmakuByRequestIDSQL, command.UserID, command.RequestID))
		if err == nil && (item.VideoID != command.VideoID || item.ProgressMS != command.ProgressMS || item.Content != command.Content || item.Mode != command.Mode || item.Color != command.Color || item.FontSize != command.FontSize) {
			return HGDanmaku{}, false, ErrIdempotencyConflict
		}
	} else if err == nil {
		created = true
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.IncrementVideoDanmakuStatShardSQL, videoID, crc32.ChecksumIEEE([]byte(command.DanmakuID))%64)
		if err == nil {
			item, err = hgScan(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoDanmakuByIDSQL, command.DanmakuID))
		}
	}
	if err != nil {
		return HGDanmaku{}, false, fmt.Errorf("save video danmaku: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return HGDanmaku{}, false, fmt.Errorf("commit video danmaku: %w", err)
	}
	return item, created, nil
}

// List 使用视频时间窗和复合游标读取；总数只聚合最多 64 个计数分片。
func (r *Repository) List(ctx context.Context, videoID string, fromMS, toMS uint32, cursor HGCursor, limit int) (HGListResult, error) {
	if _, err := r.ResolveVideo(ctx, videoID); err != nil {
		return HGListResult{}, err
	}
	var rows *sql.Rows
	var err error
	if cursor.ID == 0 {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoDanmakuFirstSQL, videoID, fromMS, toMS, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoDanmakuByCursorSQL, videoID, fromMS, toMS, cursor.ProgressMS, cursor.ID, limit)
	}
	if err != nil {
		return HGListResult{}, fmt.Errorf("list video danmaku: %w", err)
	}
	defer rows.Close()
	items := make([]HGDanmaku, 0, limit)
	for rows.Next() {
		item, scanErr := hgScan(rows)
		if scanErr != nil {
			return HGListResult{}, scanErr
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return HGListResult{}, err
	}
	var total uint64
	if err = r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoDanmakuTotalSQL, videoID).Scan(&total); err != nil {
		return HGListResult{}, fmt.Errorf("read danmaku total: %w", err)
	}
	return HGListResult{Danmaku: items, TotalCount: total}, nil
}

type hgScanner interface{ Scan(...any) error }

func hgScan(scanner hgScanner) (HGDanmaku, error) {
	var item HGDanmaku
	err := scanner.Scan(&item.ID, &item.DanmakuID, &item.SubmissionID, &item.VideoID, &item.UserID, &item.ProgressMS, &item.Content, &item.Mode, &item.Color, &item.FontSize, &item.CreatedAt)
	return item, err
}
func hgDuplicate(err error) bool {
	var target *mysql.MySQLError
	return errors.As(err, &target) && target.Number == 1062
}
