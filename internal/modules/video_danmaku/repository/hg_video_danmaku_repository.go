package VideoDanmakuRepositoryPackage

import (
	"MLC_GO/internal/events"
	VideoDanmakuEventsPackage "MLC_GO/internal/events/video_danmaku"
	"MLC_GO/internal/outbox"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
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
type Repository struct {
	db          *sql.DB
	outboxTopic string
}

// NewRepository 创建复用共享连接池的弹幕仓储。
func NewRepository(db *sql.DB) *Repository { return NewRepositoryWithTopic(db, "") }

// NewRepositoryWithTopic 创建带事务 Outbox 的弹幕仓储；topic 为空时只保留兼容 MySQL 路径。
func NewRepositoryWithTopic(db *sql.DB, topic string) *Repository {
	return &Repository{db: db, outboxTopic: topic}
}

// ResolveVideo 验证视频是否支持弹幕，主要完成三件事：
//  1. 根据 videoID 选择查询 SQL。
//  2. 查询视频是否存在并允许发送弹幕。
//  3. 返回对应的 submissionID【视频所属投稿或内容记录 ID】。
//     @param ctx
//     @param videoID 具体视频 ID
//     @return string
//     @return error
func (r *Repository) ResolveVideo(ctx context.Context, videoID string) (string, error) {
	var resolved, submissionID string
	// 注意：实际允许发送弹幕的条件取决于 SQL 的 WHERE 条件。
	hgSQL, hgArgs := hgDanmakuTarget(videoID)
	if err := r.db.QueryRowContext(ctx, hgSQL, hgArgs...).Scan(&resolved, &submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrVideoNotDanmakuEnabled
		}
		return "", fmt.Errorf("resolve danmaku video: %w", err)
	}
	return submissionID, nil
}

// Create 创建弹幕；这段 Go 代码实现的是一个视频弹幕创建系统，支持普通视频和外部平台视频（例如 B 站），并使用 MySQL 事务、幂等控制、64 分片计数器和 Outbox 事件机制，保证弹幕创建过程的数据一致性。
/* 执行流程：
用户发送弹幕
      |
      v
Create(command)
      |
      v
开启 MySQL 事务
      |
      v
根据 VideoID 查询视频
      |
      +-- external:xxx → 外部视频
      |
      +-- 普通ID       → 本地视频
      |
      v
检查视频是否允许发送弹幕
      |
      v
INSERT 弹幕数据
      |
      +-- 唯一键冲突 → 查询已有弹幕
      |                 |
      |                 +-- 参数不同 → 幂等冲突
      |                 +-- 参数相同 → 返回已有弹幕
      |
      +-- 插入成功
              |
              v
       更新弹幕分片计数
              |
              v
       查询完整弹幕数据
              |
              v
       写入 Outbox 事件
              |
              v
          COMMIT
              |
              v
       返回弹幕 + created
*/
func (r *Repository) Create(ctx context.Context, command HGCreateCommand) (HGDanmaku, bool, error) {

	// 使用 ReadCommitted（读已提交）事务隔离级别。
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGDanmaku{}, false, fmt.Errorf("begin danmaku transaction: %w", err)
	}
	defer tx.Rollback()
	var videoID, submissionID string
	hgSQL, hgArgs := hgDanmakuTarget(command.VideoID)
	// 查询成功后，得到数据库确认的视频 ID 和投稿 ID。
	if err = tx.QueryRowContext(ctx, hgSQL, hgArgs...).Scan(&videoID, &submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGDanmaku{}, false, ErrVideoNotDanmakuEnabled
		}
		return HGDanmaku{}, false, fmt.Errorf("verify danmaku video: %w", err)
	}
	// 插入弹幕
	result, err := tx.ExecContext(ctx,
		SQLQueriesPackage.InsertVideoDanmakuSQL,
		command.DanmakuID,  //弹幕业务 ID
		submissionID,       //
		videoID,            //视频 ID
		command.UserID,     //发送用户
		command.RequestID,  //幂等请求 ID
		command.ProgressMS, //视频播放进度，毫秒;比如：ProgressMS = 15000 表示弹幕在视频播放到第 15 秒时出现。
		command.Content,    //弹幕内容
		command.Mode,       //弹幕模式
		command.Color,      //弹幕颜色
		command.FontSize,   //字体大小
	)
	var item HGDanmaku
	created := false
	// 幂等处理
	if hgDuplicate(err) {
		// 如果插入时发生唯一键冲突，就根据 UserID + RequestID 查询已有弹幕
		item, err = hgScan(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoDanmakuByRequestIDSQL, command.UserID, command.RequestID))
		if err == nil && (item.VideoID != command.VideoID || item.ProgressMS != command.ProgressMS || item.Content != command.Content || item.Mode != command.Mode || item.Color != command.Color || item.FontSize != command.FontSize) {
			return HGDanmaku{}, false, ErrIdempotencyConflict
		}
	} else if err == nil {
		created = true
		var insertedID int64
		// 用于获取刚插入记录的自增主键。
		insertedID, err = result.LastInsertId()
		if err == nil {
			// 64 分片弹幕计数器，这一段是高并发计数的关键。
			// crc32.ChecksumIEEE() 它是 Go 标准库中的 CRC32 校验和计算函数。将弹幕 ID 转换成一个 uint32 数值。对 64 取模，得到 0～63 之间的分片编号。这样不同弹幕可以较均匀地分布到 64 个计数分片中。
			/* 为什么使用 64 个分片？
			假设一个热门视频有 64000 条弹幕。

			理想情况下，64 个分片平均每个存储约 1000 条弹幕的计数。
			64 个分片并不是 64 张表，而通常是同一张统计表中的 64 条计数记录（每个视频最多 64 个分片）。

			这样可以减少热门视频在单条计数记录上的行锁竞争。
			*/
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.IncrementVideoDanmakuStatShardSQL, videoID, crc32.ChecksumIEEE([]byte(command.DanmakuID))%64)
		}
		if err == nil {
			// 通过自增主键查询完整弹幕对象。
			item, err = hgScan(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoDanmakuByPrimaryIDSQL, insertedID))
		}
		// 这里使用的是 Transactional Outbox（事务发件箱）模式
		/*流程是：
		MySQL 事务开始
		      |
		      +-- INSERT 弹幕
		      |
		      +-- UPDATE 弹幕计数
		      |
		      +-- INSERT Outbox 事件
		      |
		      v
		COMMIT 一起提交
		      |
		      v
		Outbox 后台投递程序
		      |
		      v
		Kafka Topic
		      |
		      v
		其他消费者处理
		*/
		if err == nil && r.outboxTopic != "" {
			event := VideoDanmakuEventsPackage.CreatedEvent{
				EventMeta: events.NewEventMeta(ctx), DanmakuID: item.DanmakuID, SubmissionID: item.SubmissionID,
				VideoID: item.VideoID, UserID: item.UserID, RequestID: command.RequestID, Content: item.Content,
				ProgressMS: item.ProgressMS, Mode: item.Mode, Color: item.Color, FontSize: item.FontSize,
				CreatedAt: item.CreatedAt.UTC().UnixMilli(),
			}

			//关键点： SaveTx() 将事件写入当前数据库事务，并不意味着此时已经发送到 Kafka。
			// 后续通常由独立的 Outbox 投递程序将事件发布到 Kafka，供统计、审核、通知等模块消费。
			// 如果事务失败，弹幕和 Outbox 事件一起回滚。
			err = outbox.NewRepository(r.db, r.outboxTopic).SaveTx(ctx, tx, event)
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

// hgDanmakuTarget 区分本地视频和外部视频
// 为外部视频按已入库CID解析目标，原生视频保留现有权限查询。
//
//	@param hgID
//	@return string  需要执行的 SQL
//	@return []any  SQL 占位符对应的参数
func hgDanmakuTarget(hgID string) (string, []any) {
	if strings.HasPrefix(hgID, "external:") {
		return SQLQueriesPackage.HGExternalDanmakuTargetSQL, []any{hgID, hgID}
	}
	return SQLQueriesPackage.SelectDanmakuVideoTargetSQL, []any{hgID}
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
