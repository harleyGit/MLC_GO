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

// Repository 负责视频评论的同步 MySQL 权威读写。
type Repository struct{ db *sql.DB }

// NewRepository 创建基于共享 MySQL 连接池的评论仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Create 在短事务内完成资格/父链校验、幂等插入、分片计数和可选根回复计数。
func (r *Repository) Create(ctx context.Context, command HGCreateCommand) (HGComment, error) {
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
	var likeCount, dislikeCount uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentReactionTargetForUpdateSQL, commentID).Scan(&likeCount, &dislikeCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGReactionResult{}, ErrCommentNotAvailable
		}
		return HGReactionResult{}, fmt.Errorf("lock reaction target: %w", err)
	}
	oldReaction := "none"
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentReactionForUpdateSQL, commentID, userID).Scan(&oldReaction); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return HGReactionResult{}, fmt.Errorf("lock comment reaction: %w", err)
	}
	if oldReaction != reaction {
		likeCount, dislikeCount = hgApplyReactionDelta(likeCount, dislikeCount, oldReaction, reaction)
		if reaction == "none" {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.DeleteVideoCommentReactionSQL, commentID, userID)
		} else {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpsertVideoCommentReactionSQL, commentID, userID, reaction)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpdateVideoCommentReactionCountsSQL, likeCount, dislikeCount, commentID)
		}
		if err != nil {
			return HGReactionResult{}, fmt.Errorf("update comment reaction: %w", err)
		}
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
	if err != nil {
		return false, fmt.Errorf("decrement comment counters: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit delete comment transaction: %w", err)
	}
	return true, nil
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

func hgApplyReactionDelta(likeCount, dislikeCount uint64, oldReaction, newReaction string) (uint64, uint64) {
	if oldReaction == "like" && likeCount > 0 {
		likeCount--
	} else if oldReaction == "dislike" && dislikeCount > 0 {
		dislikeCount--
	}
	if newReaction == "like" {
		likeCount++
	} else if newReaction == "dislike" {
		dislikeCount++
	}
	return likeCount, dislikeCount
}

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
