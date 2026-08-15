package VideoCommentRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrSubmissionNotCommentable   = errors.New("submission is not commentable")
	ErrParentNotAvailable         = errors.New("parent comment is unavailable")
	ErrCommentNotAvailable        = errors.New("comment is unavailable")
	ErrCommentHasReplies          = errors.New("comment has replies")
	ErrCounterConsistency         = errors.New("comment counter consistency check failed")
	ErrReactionBackfillIncomplete = errors.New("comment reaction backfill is incomplete")
	ErrImageQuotaExceeded         = errors.New("comment image quota exceeded")
	ErrImageNotAvailable          = errors.New("comment image is unavailable")
)

// HGCreateCommand 是同步创建评论的权威写入参数；线程关系由 repository 从父评论派生。
type HGCreateCommand struct {
	CommentID       string
	SubmissionID    string
	UserID          string
	RequestID       string
	Content         string
	ParentCommentID string
	ImageURLs       []string
}

// HGListCursor 保存 latest/hot/replies keyset 所需的完整排序元组。
type HGListCursor struct {
	LikeCount  uint64
	ReplyCount uint64
	CreatedAt  time.Time
	ID         uint64
}

// HGComment 是 repository 与 service 之间的评论业务模型。
type HGComment struct {
	ID              uint64
	CommentID       string
	SubmissionID    string
	UserID          string
	UserName        string
	AvatarURL       string
	Content         string
	RootCommentID   string
	ParentCommentID string
	ReplyToUserID   string
	ReplyToUserName string
	LikeCount       uint64
	DislikeCount    uint64
	ReplyCount      uint64
	Reaction        string
	ImageURLs       []string
	CreatedAt       time.Time
}

// HGListResult 同时返回有界评论页和预聚合总数；总数读取固定 32 个分片，不扫描评论热表。
type HGListResult struct {
	Comments   []HGComment
	TotalCount uint64
}

// HGRepliesResult 复用评论页结构，其中 TotalCount 来自根评论维护的 reply_count。
type HGRepliesResult = HGListResult

// HGReactionResult 是最终态反应事务提交后的权威用户状态和计数。
type HGReactionResult struct {
	CommentID    string
	Reaction     string
	LikeCount    uint64
	DislikeCount uint64
}

// HGImageAsset 是对象上传前持久化的 reservation，保存所有者、确定性 storage key、公开 URL、字节数和 MIME 类型。
// 该记录与用户配额在同一事务写入，因此进程在 PUT 前后崩溃时，维护 worker 仍能定位对象并最终归还容量。
type HGImageAsset struct {
	ImageID, UserID, StorageKey, ImageURL, ContentType string
	SizeBytes                                          int64
}

// HGImageCleanupAsset 是已进入 deleting 状态、允许执行外部存储删除的最小资产投影。
// CleanupToken 是本轮 claim 的 fencing token；完成或失败恢复都必须匹配它，陈旧 worker 不得修改新一轮 claim。
type HGImageCleanupAsset struct {
	ImageID, UserID, StorageKey, CleanupToken string
	SizeBytes                                 int64
}

// HGReactionProjectionResult 返回本批已选中条目数和 revision CAS miss 数；即使后续条目失败，已观察到的 CAS miss 仍可上报。
type HGReactionProjectionResult struct {
	Projected int
	CASMisses int
}

// HGReplyProjectionResult 返回本批回复计数投影数量和 revision CAS miss 数。
type HGReplyProjectionResult = HGReactionProjectionResult

// HGImageCleanupClaim 返回成功持有新 fencing token 的资产，以及其中实际从过期 deleting 租约恢复的数量。
type HGImageCleanupClaim struct {
	Assets               []HGImageCleanupAsset
	ExpiredLeaseReclaims int
}

// Repository 负责视频评论的同步 MySQL 权威读写。
type Repository struct{ db *sql.DB }

// NewRepository 创建基于共享 MySQL 连接池的评论仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// ReserveImageAsset 在外部 I/O 前原子预占用户容量并登记确定性对象键，使崩溃后仍可回收。
// 同一用户会在 quota 主键行上自然串行，不同用户互不竞争；条件 UPDATE 保证并发上传不能突破 capacityBytes。
// 资产插入失败会回滚同事务内的容量增量，不存在“有配额占用但无可恢复 reservation”的提交状态。
func (r *Repository) ReserveImageAsset(ctx context.Context, asset HGImageAsset, capacityBytes int64) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin image quota transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.EnsureVideoCommentImageQuotaSQL, asset.UserID); err != nil {
		return fmt.Errorf("ensure image quota: %w", err)
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.ReserveVideoCommentImageQuotaSQL, asset.SizeBytes, asset.UserID, asset.SizeBytes, capacityBytes)
	if err != nil {
		return fmt.Errorf("reserve image quota: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read image quota result: %w", err)
	}
	if affected == 0 {
		return ErrImageQuotaExceeded
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoCommentImageAssetSQL, asset.ImageID, asset.UserID, asset.StorageKey, asset.ImageURL, asset.SizeBytes, asset.ContentType); err != nil {
		return fmt.Errorf("insert image reservation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit image quota: %w", err)
	}
	return nil
}

// ScheduleImageCleanup 将失败或状态不确定的上传交给持久化清理流程，不在请求线程直接释放配额。
// PUT 超时可能代表对象已写入，因此只能持久化 delete_pending 并执行幂等 DELETE，不能假设失败后对象不存在。
func (r *Repository) ScheduleImageCleanup(ctx context.Context, imageID, userID string) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.ScheduleVideoCommentImageCleanupSQL, imageID, userID)
	if err != nil {
		return fmt.Errorf("schedule image cleanup: %w", err)
	}
	return nil
}

// Create 在短事务内完成资格/父链校验、幂等插入、分片计数和可选根回复计数。
func (r *Repository) Create(ctx context.Context, command HGCreateCommand) (HGComment, error) {
	if len(command.ImageURLs) > 0 {
		if existing, existingErr := hgScanComment(r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentByRequestIDSQL, command.UserID, command.UserID, command.RequestID)); existingErr == nil {
			return existing, nil
		} else if !errors.Is(existingErr, sql.ErrNoRows) {
			return HGComment{}, fmt.Errorf("read idempotent comment: %w", existingErr)
		}
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGComment{}, fmt.Errorf("begin comment transaction: %w", err)
	}
	defer tx.Rollback()

	var submissionID string
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCommentableSubmissionSQL, command.SubmissionID, command.SubmissionID).Scan(&submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGComment{}, ErrSubmissionNotCommentable
		}
		return HGComment{}, fmt.Errorf("verify commentable submission: %w", err)
	}
	var rootCommentID, parentCommentID, replyToUserID, replyToUserName any
	if command.ParentCommentID != "" {
		var parentID, parentUserID, parentUserName string
		var parentRootID sql.NullString
		if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentParentForUpdateSQL, command.ParentCommentID, submissionID).Scan(&parentID, &parentRootID, &parentUserID, &parentUserName); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return HGComment{}, ErrParentNotAvailable
			}
			return HGComment{}, fmt.Errorf("select parent comment: %w", err)
		}
		parentCommentID, replyToUserID, replyToUserName = parentID, parentUserID, parentUserName
		if parentRootID.Valid {
			// All nested reply writers lock direct parent then root, preventing parent/root lock-order inversion.
			var visibleRootID string
			if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentRootForUpdateSQL, parentRootID.String, submissionID).Scan(&visibleRootID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return HGComment{}, ErrParentNotAvailable
				}
				return HGComment{}, fmt.Errorf("select root comment: %w", err)
			}
			rootCommentID = visibleRootID
		} else {
			// A direct root reply already holds the root row lock through the parent lookup.
			rootCommentID = parentID
		}
	}
	if command.ImageURLs == nil {
		command.ImageURLs = []string{}
	}
	imageIDs := make([]string, 0, len(command.ImageURLs))
	for _, imageURL := range command.ImageURLs {
		var imageID string
		if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentImageForAttachSQL, imageURL, imageURL, command.UserID).Scan(&imageID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return HGComment{}, ErrImageNotAvailable
			}
			return HGComment{}, fmt.Errorf("lock comment image: %w", err)
		}
		imageIDs = append(imageIDs, imageID)
	}
	imageJSON, err := json.Marshal(command.ImageURLs)
	if err != nil {
		return HGComment{}, fmt.Errorf("marshal comment images: %w", err)
	}
	insertResult, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoCommentSQL, command.CommentID, submissionID, command.UserID, command.RequestID, rootCommentID, parentCommentID, replyToUserID, replyToUserName, command.Content, string(imageJSON))
	var comment HGComment
	if hgIsDuplicateKey(err) {
		comment, err = hgScanComment(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentByRequestIDSQL, command.UserID, command.UserID, command.RequestID))
	} else if err == nil {
		if err = hgRequireAffected(insertResult, 1); err == nil {
			for _, imageID := range imageIDs {
				var imageResult sql.Result
				imageResult, err = tx.ExecContext(ctx, SQLQueriesPackage.AttachVideoCommentImageSQL, command.CommentID, imageID, command.UserID)
				if err == nil {
					err = hgRequireAffected(imageResult, 1)
				}
				if err != nil {
					break
				}
			}
		}
		if err == nil {
			var shardResult sql.Result
			shardResult, err = tx.ExecContext(ctx, SQLQueriesPackage.IncrementVideoCommentStatShardSQL, submissionID, hgCommentShard(command.CommentID))
			if err == nil {
				err = hgRequirePositiveAffected(shardResult)
			}
		}
		if err == nil && command.ParentCommentID != "" {
			replyShard := hgReplyShard(command.CommentID)
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.EnsureVideoCommentReplyShardSQL, rootCommentID)
			if err == nil {
				_, err = tx.ExecContext(ctx, SQLQueriesPackage.IncrementVideoCommentReplyShardSQL, rootCommentID, replyShard)
			}
			if err == nil {
				_, err = tx.ExecContext(ctx, SQLQueriesPackage.MarkVideoCommentReplyDirtySQL, rootCommentID, replyShard)
			}
		}
		if err == nil {
			comment, err = hgScanComment(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentByCommentIDSQL, command.UserID, command.CommentID))
		}
	}
	if err != nil {
		return HGComment{}, fmt.Errorf("save video comment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return HGComment{}, fmt.Errorf("commit comment transaction: %w", err)
	}
	return comment, nil
}

// List 先把 submission_id/video_id 解析为可评论的权威 submission_id，再执行顶级评论 keyset 和分片总数查询。
// 解析与两个读取均为独立只读语句，不需要事务；可见性策略与 Create 保持一致。
func (r *Repository) List(ctx context.Context, userID, targetID, sort string, cursor HGListCursor, limit int) (HGListResult, error) {
	var submissionID string
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectCommentableSubmissionSQL, targetID, targetID).Scan(&submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGListResult{}, ErrSubmissionNotCommentable
		}
		return HGListResult{}, fmt.Errorf("resolve video comment target: %w", err)
	}
	var rows *sql.Rows
	var err error
	if sort == "hot" {
		if cursor.ID == 0 {
			rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsHotFirstSQL, userID, submissionID, limit)
		} else {
			rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsHotByCursorSQL, userID, submissionID, cursor.LikeCount, cursor.ReplyCount, cursor.CreatedAt, cursor.ID, limit)
		}
	} else if cursor.ID == 0 {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsLatestFirstSQL, userID, submissionID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsLatestByCursorSQL, userID, submissionID, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return HGListResult{}, fmt.Errorf("list video comments: %w", err)
	}
	comments, err := hgScanComments(rows, limit)
	if err != nil {
		return HGListResult{}, err
	}
	var totalCount uint64
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentTotalCountSQL, submissionID).Scan(&totalCount); err != nil {
		return HGListResult{}, fmt.Errorf("read video comment total: %w", err)
	}
	return HGListResult{Comments: comments, TotalCount: totalCount}, nil
}

// ListReplies 按 (created_at,id) 正序 keyset 读取回复，totalCount 来自根评论 reply_count。
func (r *Repository) ListReplies(ctx context.Context, userID, rootCommentID string, cursor HGListCursor, limit int) (HGRepliesResult, error) {
	var submissionID string
	var totalCount uint64
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentRootListMetadataSQL, rootCommentID).Scan(&submissionID, &totalCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGRepliesResult{}, ErrCommentNotAvailable
		}
		return HGRepliesResult{}, fmt.Errorf("read root reply metadata: %w", err)
	}
	var rows *sql.Rows
	var err error
	if cursor.ID == 0 {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentRepliesFirstSQL, userID, submissionID, rootCommentID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentRepliesByCursorSQL, userID, submissionID, rootCommentID, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return HGRepliesResult{}, fmt.Errorf("list video comment replies: %w", err)
	}
	comments, err := hgScanComments(rows, limit)
	if err != nil {
		return HGRepliesResult{}, err
	}
	if err := r.hgHydrateReplyToUserNames(ctx, comments); err != nil {
		return HGRepliesResult{}, err
	}
	return HGRepliesResult{Comments: comments, TotalCount: totalCount}, nil
}

// SetReaction 锁定当前用户关系行，只更新其固定 32 分片之一，并返回分片聚合后的权威计数。
// checkpoint completed=false 时拒绝在线写入，防止历史回填聚合与运行期 delta 双写同时修改 shard；列表和 hot 仍读取异步投影列。
func (r *Repository) SetReaction(ctx context.Context, userID, commentID, reaction string) (HGReactionResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGReactionResult{}, fmt.Errorf("begin reaction transaction: %w", err)
	}
	defer tx.Rollback()
	var backfillReady bool
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentReactionBackfillReadySQL).Scan(&backfillReady); err != nil {
		return HGReactionResult{}, fmt.Errorf("read reaction backfill state: %w", err)
	}
	if !backfillReady {
		return HGReactionResult{}, ErrReactionBackfillIncomplete
	}
	var visibleCommentID string
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentReactionTargetSQL, commentID).Scan(&visibleCommentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGReactionResult{}, ErrCommentNotAvailable
		}
		return HGReactionResult{}, fmt.Errorf("lock reaction target: %w", err)
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.EnsureVideoCommentReactionSQL, commentID, userID); err != nil {
		return HGReactionResult{}, fmt.Errorf("ensure comment reaction: %w", err)
	}
	oldReaction := "none"
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentReactionForUpdateSQL, commentID, userID).Scan(&oldReaction); err != nil {
		return HGReactionResult{}, fmt.Errorf("lock comment reaction: %w", err)
	}
	if oldReaction != reaction {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpsertVideoCommentReactionSQL, commentID, userID, reaction)
		if err == nil {
			likeDelta, dislikeDelta := hgReactionDeltas(oldReaction, reaction)
			_, err = tx.ExecContext(
				ctx,
				SQLQueriesPackage.UpdateVideoCommentReactionShardSQL,
				commentID,
				hgReactionShard(userID),
				max(likeDelta, 0),
				max(dislikeDelta, 0),
				likeDelta,
				dislikeDelta,
			)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.MarkVideoCommentReactionDirtySQL, commentID)
		}
		if err != nil {
			return HGReactionResult{}, fmt.Errorf("update comment reaction: %w", err)
		}
	}
	var likeCount, dislikeCount uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentReactionShardTotalsSQL, commentID).Scan(&likeCount, &dislikeCount); err != nil {
		return HGReactionResult{}, fmt.Errorf("read comment reaction totals: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return HGReactionResult{}, fmt.Errorf("commit reaction transaction: %w", err)
	}
	return HGReactionResult{CommentID: commentID, Reaction: reaction, LikeCount: likeCount, DislikeCount: dislikeCount}, nil
}

// Delete 在同一短事务内完成作者限定软删除、分片总数和根回复数扣减。
// 有可见回复的根评论禁止删除，避免回复树失去可查询入口。
func (r *Repository) Delete(ctx context.Context, userID, commentID string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, fmt.Errorf("begin delete comment transaction: %w", err)
	}
	defer tx.Rollback()
	var submissionID string
	var rootCommentID sql.NullString
	var replyCount uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentDeleteTargetForUpdateSQL, commentID, userID).Scan(&submissionID, &rootCommentID, &replyCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock delete comment: %w", err)
	}
	if !rootCommentID.Valid && replyCount > 0 {
		return false, ErrCommentHasReplies
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.SoftDeleteVideoCommentSQL, commentID, userID)
	if err != nil {
		return false, fmt.Errorf("soft delete video comment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return false, fmt.Errorf("read soft delete result: %w", err)
	}
	var shardResult sql.Result
	if shardResult, err = tx.ExecContext(ctx, SQLQueriesPackage.DecrementVideoCommentStatShardSQL, submissionID, hgCommentShard(commentID)); err == nil {
		err = hgRequireAffected(shardResult, 1)
	}
	if err == nil && rootCommentID.Valid {
		replyShard := hgReplyShard(commentID)
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.EnsureVideoCommentReplyShardSQL, rootCommentID.String)
		if err == nil {
			var replyResult sql.Result
			replyResult, err = tx.ExecContext(ctx, SQLQueriesPackage.DecrementVideoCommentReplyShardSQL, rootCommentID.String, replyShard)
			if err == nil {
				err = hgRequireAffected(replyResult, 1)
			}
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.MarkVideoCommentReplyDirtySQL, rootCommentID.String, replyShard)
		}
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.MarkVideoCommentImagesDeletePendingSQL, commentID)
	}
	if err != nil {
		return false, fmt.Errorf("decrement comment counters: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit delete comment transaction: %w", err)
	}
	return true, nil
}

// ProjectReactionCounts 将一个有界 dirty 批次写入 video_comments 的列表/hot 反范式投影。
// revision 是 CAS 版本：投影期间若有新 reaction，条件 DELETE 不命中，dirty 行保留；随后刷新排队时间把热点项移到队尾，避免阻塞后续评论。
func (r *Repository) ProjectReactionCounts(ctx context.Context, limit int) (HGReactionProjectionResult, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentReactionDirtySQL, limit)
	if err != nil {
		return HGReactionProjectionResult{}, fmt.Errorf("list reaction dirty: %w", err)
	}
	type dirty struct {
		id       string
		revision uint64
	}
	var entries []dirty
	for rows.Next() {
		var entry dirty
		if err := rows.Scan(&entry.id, &entry.revision); err != nil {
			rows.Close()
			return HGReactionProjectionResult{}, err
		}
		entries = append(entries, entry)
	}
	rows.Close()
	// Projected 表示本轮已选中的 dirty 数，用于维护任务判断是否继续下一批；CASMisses 随已执行条目递增。
	result := HGReactionProjectionResult{Projected: len(entries)}
	for _, entry := range entries {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.ProjectVideoCommentReactionCountsSQL, entry.id); err == nil {
			var deleteResult sql.Result
			deleteResult, err = tx.ExecContext(ctx, SQLQueriesPackage.DeleteVideoCommentReactionDirtySQL, entry.id, entry.revision)
			if err == nil {
				var affected int64
				affected, err = deleteResult.RowsAffected()
				if err == nil && affected == 0 {
					result.CASMisses++
					// revision 已变化说明并发 reaction 发生在本轮投影窗口内；保留信号并公平重排，下一轮会再次聚合最新 32 分片。
					_, err = tx.ExecContext(ctx, SQLQueriesPackage.RequeueVideoCommentReactionDirtySQL, entry.id, entry.revision)
				}
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return result, fmt.Errorf("project reaction %s: %w", entry.id, err)
		}
		if err = tx.Commit(); err != nil {
			return result, err
		}
	}
	return result, nil
}

// ProjectReplyCounts 将有界 dirty 根评论的 256 分片和投影到列表列；revision CAS 保证并发回复不会丢失重投信号。
func (r *Repository) ProjectReplyCounts(ctx context.Context, limit int) (HGReplyProjectionResult, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentReplyDirtySQL, limit)
	if err != nil {
		return HGReplyProjectionResult{}, fmt.Errorf("list reply count dirty: %w", err)
	}
	type dirty struct {
		id       string
		shardID  uint32
		revision uint64
	}
	entries := make([]dirty, 0, limit)
	for rows.Next() {
		var entry dirty
		if err := rows.Scan(&entry.id, &entry.shardID, &entry.revision); err != nil {
			rows.Close()
			return HGReplyProjectionResult{}, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Close(); err != nil {
		return HGReplyProjectionResult{}, err
	}
	result := HGReplyProjectionResult{Projected: len(entries)}
	for _, entry := range entries {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.ProjectVideoCommentReplyCountSQL, entry.id); err == nil {
			var deleteResult sql.Result
			deleteResult, err = tx.ExecContext(ctx, SQLQueriesPackage.DeleteVideoCommentReplyDirtySQL, entry.id, entry.shardID, entry.revision)
			if err == nil {
				var affected int64
				affected, err = deleteResult.RowsAffected()
				if err == nil && affected == 0 {
					result.CASMisses++
					_, err = tx.ExecContext(ctx, SQLQueriesPackage.RequeueVideoCommentReplyDirtySQL, entry.id, entry.shardID, entry.revision)
				}
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return result, fmt.Errorf("project reply count %s: %w", entry.id, err)
		}
		if err = tx.Commit(); err != nil {
			return result, err
		}
	}
	return result, nil
}

// ClaimImageCleanup 使用索引有界扫描并在同一事务内把候选资产切换为 deleting。
// 该状态迁移与评论创建的 pending 行锁互斥，防止对象在绑定成功后被并发删除。
// 查询优先级为显式删除、崩溃恢复、过期未绑定图片；每类使用独立复合索引，并共享剩余 limit，避免亿级表上的 OR/index-merge 扫描。
func (r *Repository) ClaimImageCleanup(ctx context.Context, orphanBefore time.Time, limit int, lease time.Duration) (HGImageCleanupClaim, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGImageCleanupClaim{}, err
	}
	defer tx.Rollback()
	assets := make([]HGImageCleanupAsset, 0, limit)
	queries := []struct {
		sql                 string
		args                []any
		expiredLeaseReclaim bool
	}{
		// delete_pending 已明确不可再绑定，优先释放对象和用户容量。
		{sql: SQLQueriesPackage.ListDeletePendingVideoCommentImageCleanupForUpdateSQL, args: []any{limit}},
		// deleting 租约过期表示前一 worker 可能崩溃；S3 DELETE 幂等，因此允许新 token 重试。
		{sql: SQLQueriesPackage.ListExpiredVideoCommentImageCleanupForUpdateSQL, args: []any{limit}, expiredLeaseReclaim: true},
		// pending 仅在超过 orphanBefore 后清理，为客户端上传后创建评论保留足够绑定窗口。
		{sql: SQLQueriesPackage.ListPendingVideoCommentImageCleanupForUpdateSQL, args: []any{orphanBefore, limit}},
	}
	// 仅记录候选来源；最终 reclaim 指标必须等 Mark...Deleting 成功后再累计，避免并发竞争造成虚高。
	expiredLeaseAssets := make(map[string]struct{})
	for _, query := range queries {
		remaining := limit - len(assets)
		if remaining <= 0 {
			break
		}
		query.args[len(query.args)-1] = remaining
		rows, queryErr := tx.QueryContext(ctx, query.sql, query.args...)
		if queryErr != nil {
			return HGImageCleanupClaim{}, fmt.Errorf("list image cleanup: %w", queryErr)
		}
		for rows.Next() {
			var asset HGImageCleanupAsset
			if scanErr := rows.Scan(&asset.ImageID, &asset.UserID, &asset.StorageKey, &asset.SizeBytes); scanErr != nil {
				rows.Close()
				return HGImageCleanupClaim{}, scanErr
			}
			assets = append(assets, asset)
			if query.expiredLeaseReclaim {
				expiredLeaseAssets[asset.ImageID] = struct{}{}
			}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return HGImageCleanupClaim{}, rowsErr
		}
		rows.Close()
	}
	claimed := assets[:0]
	expiredLeaseReclaims := 0
	for _, asset := range assets {
		asset.CleanupToken = UtilsPackage.GenerateBusinessID("CLM")
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.MarkVideoCommentImageDeletingSQL, asset.CleanupToken, time.Now().UTC().Add(lease), asset.ImageID)
		if err != nil {
			return HGImageCleanupClaim{}, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return HGImageCleanupClaim{}, err
		}
		if affected == 1 {
			claimed = append(claimed, asset)
			if _, ok := expiredLeaseAssets[asset.ImageID]; ok {
				expiredLeaseReclaims++
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return HGImageCleanupClaim{}, err
	}
	return HGImageCleanupClaim{Assets: claimed, ExpiredLeaseReclaims: expiredLeaseReclaims}, nil
}

// MaintenanceOldestTimes 使用四个单状态索引 LIMIT 1 查询待投影/待清理的最早可处理时间。
// pending 资产返回 created_at+orphanAge，而不是上传时间，确保年龄表示超过绑定宽限期后的真实积压；指标抓取只读内存，不直接访问 MySQL。
func (r *Repository) MaintenanceOldestTimes(ctx context.Context, orphanBefore time.Time, orphanAge time.Duration) (time.Time, time.Time, error) {
	dirtyOldest, err := hgQueryOptionalTime(ctx, r.db, SQLQueriesPackage.SelectVideoCommentReactionDirtyOldestSQL)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("read oldest reaction dirty: %w", err)
	}
	cleanupQueries := []struct {
		query string
		args  []any
	}{
		{query: SQLQueriesPackage.SelectDeletePendingVideoCommentImageCleanupOldestSQL},
		{query: SQLQueriesPackage.SelectExpiredVideoCommentImageCleanupOldestSQL},
	}
	var cleanupOldest time.Time
	for _, cleanupQuery := range cleanupQueries {
		candidate, queryErr := hgQueryOptionalTime(ctx, r.db, cleanupQuery.query, cleanupQuery.args...)
		if queryErr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("read oldest image cleanup: %w", queryErr)
		}
		if !candidate.IsZero() && (cleanupOldest.IsZero() || candidate.Before(cleanupOldest)) {
			cleanupOldest = candidate
		}
	}
	if pendingCreatedAt, queryErr := hgQueryOptionalTime(ctx, r.db, SQLQueriesPackage.SelectPendingVideoCommentImageCleanupOldestSQL, orphanBefore); queryErr != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("read oldest pending image cleanup: %w", queryErr)
	} else if eligibleAt := pendingCreatedAt.Add(orphanAge); !pendingCreatedAt.IsZero() && (cleanupOldest.IsZero() || eligibleAt.Before(cleanupOldest)) {
		cleanupOldest = eligibleAt
	}
	return dirtyOldest, cleanupOldest, nil
}

func hgQueryOptionalTime(ctx context.Context, db *sql.DB, query string, args ...any) (time.Time, error) {
	var value time.Time
	err := db.QueryRowContext(ctx, query, args...).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	return value, err
}

// CompleteImageCleanup 在对象删除成功后原子删除资产记录并归还用户容量。
// 只有当前 CleanupToken 成功删除一条资产记录时才减少 quota，保证重复 DELETE、租约重领和陈旧 worker 不会双重释放。
func (r *Repository) CompleteImageCleanup(ctx context.Context, asset HGImageCleanupAsset) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var result sql.Result
	if result, err = tx.ExecContext(ctx, SQLQueriesPackage.DeleteVideoCommentImageAssetSQL, asset.ImageID, asset.CleanupToken); err == nil {
		var affected int64
		affected, err = result.RowsAffected()
		if err == nil && affected == 1 {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.ReleaseVideoCommentImageQuotaSQL, asset.SizeBytes, asset.SizeBytes, asset.UserID)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ReleaseImageCleanup 在对象删除失败时恢复 pending/delete_pending，允许后续轮次重试。
func (r *Repository) ReleaseImageCleanup(ctx context.Context, asset HGImageCleanupAsset) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.ReleaseVideoCommentImageCleanupSQL, asset.ImageID, asset.CleanupToken)
	return err
}

type hgScanner interface{ Scan(...any) error }

func hgScanComment(scanner hgScanner) (HGComment, error) {
	var comment HGComment
	var rootID, parentID, replyToUserID sql.NullString
	var imageJSON string
	err := scanner.Scan(&comment.ID, &comment.CommentID, &comment.SubmissionID, &comment.UserID, &comment.UserName, &comment.AvatarURL, &comment.Content, &rootID, &parentID, &replyToUserID, &comment.ReplyToUserName, &comment.LikeCount, &comment.DislikeCount, &comment.ReplyCount, &comment.Reaction, &imageJSON, &comment.CreatedAt)
	if err != nil {
		return HGComment{}, err
	}
	comment.RootCommentID, comment.ParentCommentID, comment.ReplyToUserID = rootID.String, parentID.String, replyToUserID.String
	if err := json.Unmarshal([]byte(imageJSON), &comment.ImageURLs); err != nil {
		return HGComment{}, err
	}
	if comment.ImageURLs == nil {
		comment.ImageURLs = []string{}
	}
	return comment, nil
}

func hgScanComments(rows *sql.Rows, limit int) ([]HGComment, error) {
	defer rows.Close()
	comments := make([]HGComment, 0, limit)
	for rows.Next() {
		comment, err := hgScanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan video comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate video comments: %w", err)
	}
	return comments, nil
}

// hgHydrateReplyToUserNames 仅兼容迁移前的空快照；每页最多执行一次有界批量主键查询，不回写评论热表。
func (r *Repository) hgHydrateReplyToUserNames(ctx context.Context, comments []HGComment) error {
	userIDs := make([]string, 0, len(comments))
	seen := make(map[string]struct{}, len(comments))
	for _, comment := range comments {
		if comment.ReplyToUserID == "" || comment.ReplyToUserName != "" {
			continue
		}
		if _, exists := seen[comment.ReplyToUserID]; exists {
			continue
		}
		seen[comment.ReplyToUserID] = struct{}{}
		userIDs = append(userIDs, comment.ReplyToUserID)
	}
	if len(userIDs) == 0 {
		return nil
	}
	payload, err := json.Marshal(userIDs)
	if err != nil {
		return fmt.Errorf("marshal reply user ids: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectVideoCommentReplyUserNamesSQL, string(payload))
	if err != nil {
		return fmt.Errorf("list reply user names: %w", err)
	}
	defer rows.Close()
	names := make(map[string]string, len(userIDs))
	for rows.Next() {
		var userID, userName string
		if err := rows.Scan(&userID, &userName); err != nil {
			return fmt.Errorf("scan reply user name: %w", err)
		}
		names[userID] = userName
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate reply user names: %w", err)
	}
	for index := range comments {
		if comments[index].ReplyToUserName == "" {
			comments[index].ReplyToUserName = names[comments[index].ReplyToUserID]
		}
	}
	return nil
}

func hgReactionDeltas(oldReaction, newReaction string) (int64, int64) {
	var like, dislike int64
	if oldReaction == "like" {
		like--
	} else if oldReaction == "dislike" {
		dislike--
	}
	if newReaction == "like" {
		like++
	} else if newReaction == "dislike" {
		dislike++
	}
	return like, dislike
}

func hgReactionShard(userID string) uint32 { return crc32.ChecksumIEEE([]byte(userID)) % 32 }

func hgCommentShard(commentID string) uint32 { return crc32.ChecksumIEEE([]byte(commentID)) % 32 }

func hgReplyShard(commentID string) uint32 { return crc32.ChecksumIEEE([]byte(commentID)) % 256 }

func hgRequireAffected(result sql.Result, expected int64) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != expected {
		return fmt.Errorf("%w: affected=%d expected=%d", ErrCounterConsistency, affected, expected)
	}
	return nil
}

func hgRequirePositiveAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected < 1 {
		return fmt.Errorf("%w: affected=%d", ErrCounterConsistency, affected)
	}
	return nil
}

func hgIsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
