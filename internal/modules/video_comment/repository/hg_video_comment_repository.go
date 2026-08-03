package VideoCommentRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

var ErrSubmissionNotCommentable = errors.New("submission is not commentable")

// HGCreateCommand 是同步创建顶级评论的权威写入参数。
type HGCreateCommand struct {
	CommentID    string
	SubmissionID string
	UserID       string
	RequestID    string
	Content      string
}

// HGListCursor 保存 latest/hot keyset 所需的完整排序元组。
type HGListCursor struct {
	LikeCount  uint64
	ReplyCount uint64
	CreatedAt  time.Time
	ID         uint64
}

// HGComment 是 repository 与 service 之间的评论业务模型。
type HGComment struct {
	ID           uint64
	CommentID    string
	SubmissionID string
	UserID       string
	UserName     string
	AvatarURL    string
	Content      string
	LikeCount    uint64
	ReplyCount   uint64
	CreatedAt    time.Time
}

// Repository 负责视频评论的同步 MySQL 权威读写。
type Repository struct{ db *sql.DB }

// NewRepository 创建基于共享 MySQL 连接池的评论仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Create 在一个短事务内完成稿件资格点查、幂等写入和结果读取，不执行外部 I/O。
func (r *Repository) Create(ctx context.Context, command HGCreateCommand) (HGComment, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGComment{}, fmt.Errorf("begin comment transaction: %w", err)
	}
	defer tx.Rollback()

	var submissionID string
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCommentableSubmissionSQL, command.SubmissionID).Scan(&submissionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGComment{}, ErrSubmissionNotCommentable
		}
		return HGComment{}, fmt.Errorf("verify commentable submission: %w", err)
	}

	_, err = tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoCommentSQL, command.CommentID, command.SubmissionID, command.UserID, command.RequestID, command.Content)
	var comment HGComment
	if hgIsDuplicateKey(err) {
		comment, err = hgScanComment(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentByRequestIDSQL, command.UserID, command.RequestID))
	} else if err == nil {
		comment, err = hgScanComment(tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoCommentByCommentIDSQL, command.CommentID))
	}
	if err != nil {
		return HGComment{}, fmt.Errorf("save video comment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return HGComment{}, fmt.Errorf("commit comment transaction: %w", err)
	}
	return comment, nil
}

// List 使用与排序模式匹配的复合 keyset 查询顶级评论。
func (r *Repository) List(ctx context.Context, submissionID string, sort string, cursor HGListCursor, limit int) ([]HGComment, error) {
	var rows *sql.Rows
	var err error
	if sort == "hot" {
		if cursor.ID == 0 {
			rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsHotFirstSQL, submissionID, limit)
		} else {
			rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsHotByCursorSQL, submissionID, cursor.LikeCount, cursor.ReplyCount, cursor.CreatedAt, cursor.ID, limit)
		}
	} else if cursor.ID == 0 {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsLatestFirstSQL, submissionID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.ListVideoCommentsLatestByCursorSQL, submissionID, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list video comments: %w", err)
	}
	defer rows.Close()

	comments := make([]HGComment, 0, limit)
	for rows.Next() {
		comment, scanErr := hgScanComment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan video comment: %w", scanErr)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate video comments: %w", err)
	}
	return comments, nil
}

// Delete 按 comment_id 唯一键和作者 user_id 执行范围受限的软删除。
func (r *Repository) Delete(ctx context.Context, userID string, commentID string) (bool, error) {
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.SoftDeleteVideoCommentSQL, commentID, userID)
	if err != nil {
		return false, fmt.Errorf("soft delete video comment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read soft delete result: %w", err)
	}
	return affected == 1, nil
}

type hgScanner interface{ Scan(...any) error }

func hgScanComment(scanner hgScanner) (HGComment, error) {
	var comment HGComment
	err := scanner.Scan(&comment.ID, &comment.CommentID, &comment.SubmissionID, &comment.UserID, &comment.UserName, &comment.AvatarURL, &comment.Content, &comment.LikeCount, &comment.ReplyCount, &comment.CreatedAt)
	return comment, err
}

func hgIsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
