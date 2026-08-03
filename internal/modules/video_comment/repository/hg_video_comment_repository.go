package VideoCommentRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
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
	ErrSubmissionNotCommentable = errors.New("submission is not commentable")
	ErrParentNotAvailable       = errors.New("parent comment is unavailable")
	ErrCommentNotAvailable      = errors.New("comment is unavailable")
	ErrCommentHasReplies        = errors.New("comment has replies")
	ErrCounterConsistency       = errors.New("comment counter consistency check failed")
	ErrImageQuotaExceeded       = errors.New("comment image quota exceeded")
	ErrImageNotAvailable        = errors.New("comment image is unavailable")
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

// HGImageAsset is the durable ownership record used to bind or clean an uploaded object.
type HGImageAsset struct {
	ImageID, UserID, StorageKey, ImageURL, ContentType string
	SizeBytes                                          int64
}

// HGImageCleanupAsset 是已进入 deleting 状态、允许执行外部存储删除的最小资产投影。
type HGImageCleanupAsset struct {
	ImageID, UserID, StorageKey string
	SizeBytes                   int64
}

// Repository 负责视频评论的同步 MySQL 权威读写。
type Repository struct{ db *sql.DB }

// NewRepository 创建基于共享 MySQL 连接池的评论仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// ReserveImageQuota atomically reserves authoritative per-user storage before external I/O.
func (r *Repository) ReserveImageQuota(ctx context.Context, userID string, sizeBytes, capacityBytes int64) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin image quota transaction: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoCommentImageQuotaSQL, userID, sizeBytes, capacityBytes, capacityBytes)
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
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit image quota: %w", err)
	}
	return nil
}

// ReleaseImageQuota 在上传失败或对象清理完成后归还用户权威容量，单位为字节。
func (r *Repository) ReleaseImageQuota(ctx context.Context, userID string, sizeBytes int64) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.ReleaseVideoCommentImageQuotaSQL, sizeBytes, sizeBytes, userID)
	if err != nil {
		return fmt.Errorf("release image quota: %w", err)
	}
	return nil
}

// CreateImageAsset 记录 pending 图片的所有者和存储 key，使孤儿对象可被可靠清理。
func (r *Repository) CreateImageAsset(ctx context.Context, asset HGImageAsset) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertVideoCommentImageAssetSQL, asset.ImageID, asset.UserID, asset.StorageKey, asset.ImageURL, asset.SizeBytes, asset.ContentType)
	if err != nil {
		return fmt.Errorf("create image asset: %w", err)
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
	var rootCommentID, parentCommentID, replyToUserID any
	if command.ParentCommentID != "" {
		var parentID, parentUserID string
		var parentRootID sql.NullString
		if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentParentForUpdateSQL, command.ParentCommentID, submissionID).Scan(&parentID, &parentRootID, &parentUserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return HGComment{}, ErrParentNotAvailable
			}
			return HGComment{}, fmt.Errorf("select parent comment: %w", err)
		}
		parentCommentID, replyToUserID = parentID, parentUserID
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
	insertResult, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoCommentSQL, command.CommentID, submissionID, command.UserID, command.RequestID, rootCommentID, parentCommentID, replyToUserID, command.Content, string(imageJSON))
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
			var replyResult sql.Result
			replyResult, err = tx.ExecContext(ctx, SQLQueriesPackage.IncrementVideoCommentReplyCountSQL, rootCommentID)
			if err == nil {
				err = hgRequireAffected(replyResult, 1)
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
	return HGRepliesResult{Comments: comments, TotalCount: totalCount}, nil
}

// SetReaction 锁定评论计数行和当前用户关系行，只应用旧状态到最终状态的差值。
// 当前实现优先保证同步权威结果；超热点评论后续应将计数迁移到分片或异步投影，避免单行锁竞争。
func (r *Repository) SetReaction(ctx context.Context, userID, commentID, reaction string) (HGReactionResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGReactionResult{}, fmt.Errorf("begin reaction transaction: %w", err)
	}
	defer tx.Rollback()
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
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpdateVideoCommentReactionShardSQL, commentID, hgReactionShard(userID), likeDelta, dislikeDelta)
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
		var replyResult sql.Result
		replyResult, err = tx.ExecContext(ctx, SQLQueriesPackage.DecrementVideoCommentReplyCountSQL, rootCommentID.String)
		if err == nil {
			err = hgRequireAffected(replyResult, 1)
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

// ProjectReactionCounts applies one bounded dirty batch to the denormalized list and hot-sort projection.
func (r *Repository) ProjectReactionCounts(ctx context.Context, limit int) (int, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentReactionDirtySQL, limit)
	if err != nil {
		return 0, fmt.Errorf("list reaction dirty: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.ProjectVideoCommentReactionCountsSQL, id); err == nil {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.DeleteVideoCommentReactionDirtySQL, id)
		}
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("project reaction %s: %w", id, err)
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// ClaimImageCleanup 使用索引有界扫描并在同一事务内把候选资产切换为 deleting。
// 该状态迁移与评论创建的 pending 行锁互斥，防止对象在绑定成功后被并发删除。
func (r *Repository) ClaimImageCleanup(ctx context.Context, orphanBefore time.Time, limit int) ([]HGImageCleanupAsset, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentImageCleanupForUpdateSQL, orphanBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("list image cleanup: %w", err)
	}
	assets := make([]HGImageCleanupAsset, 0, limit)
	for rows.Next() {
		var asset HGImageCleanupAsset
		if err := rows.Scan(&asset.ImageID, &asset.UserID, &asset.StorageKey, &asset.SizeBytes); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	claimed := assets[:0]
	for _, asset := range assets {
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.MarkVideoCommentImageDeletingSQL, asset.ImageID)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected == 1 {
			claimed = append(claimed, asset)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return claimed, nil
}

// CompleteImageCleanup 在对象删除成功后原子删除资产记录并归还用户容量。
func (r *Repository) CompleteImageCleanup(ctx context.Context, asset HGImageCleanupAsset) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var result sql.Result
	if result, err = tx.ExecContext(ctx, SQLQueriesPackage.DeleteVideoCommentImageAssetSQL, asset.ImageID); err == nil {
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
func (r *Repository) ReleaseImageCleanup(ctx context.Context, imageID string) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.ReleaseVideoCommentImageCleanupSQL, imageID)
	return err
}

type hgScanner interface{ Scan(...any) error }

func hgScanComment(scanner hgScanner) (HGComment, error) {
	var comment HGComment
	var rootID, parentID, replyToUserID sql.NullString
	var imageJSON string
	err := scanner.Scan(&comment.ID, &comment.CommentID, &comment.SubmissionID, &comment.UserID, &comment.UserName, &comment.AvatarURL, &comment.Content, &rootID, &parentID, &replyToUserID, &comment.LikeCount, &comment.DislikeCount, &comment.ReplyCount, &comment.Reaction, &imageJSON, &comment.CreatedAt)
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
