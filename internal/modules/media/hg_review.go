package media

import (
	Transport "MLC_GO/internal/modules/im/transport"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var hgSealedKey = regexp.MustCompile(`^sealed/[a-f0-9]{32}$`)
var hgQuarantineKey = regexp.MustCompile(`^quarantine/[a-z]+/[a-z]+/[0-9a-f-]{36}\.[a-z0-9]+$`)

// HGReviewer 接受现有 IM 审核服务的同步契约：扫描、探测真实类型/摘要、不可变封存；pending 不等于通过。
type HGReviewer interface {
	HGReview(context.Context, string, string, string, string, string, int64) (Transport.HGReviewResult, error)
}

// HGEnableReview 在模块注册/后台启动前注入受信审核服务及只供 worker 使用的 S3 能力。
func (s *HGService) HGEnableReview(reviewer HGReviewer, sealed HGSealedStore) error {
	if reviewer == nil || sealed == nil {
		return HGErrInvalid
	}
	s.reviewer, s.sealed = reviewer, sealed
	return nil
}

type hgReviewJob struct {
	upload   string // 会话主键，保证重试命中相同审核任务。
	asset    string // UUID；审核服务使用去掉连字符后的 32 位资源标识。
	purpose  string // 业务类型，对审核返回的宽高与时长做交叉校验。
	key      string // 隔离对象键；来自数据库且须符合服务端生成规则。
	mime     string // 上传申请 MIME；须与审核器探测的 MIME 相同。
	size     int64  // 上传申请字节数；须与封存对象一致。
	sha      string // 上传申请小写 SHA256；须由审核器重新计算确认。
	attempts int    // 本次领取前已尝试次数；最多 12 次。
	lease    []byte // 随机租约令牌；仅当前 worker 可回写结果。
}

func (s *HGService) hgClaimReview(ctx context.Context) (hgReviewJob, error) {
	var job hgReviewJob
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return job, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, SQL.HGMediaReviewClaimSQL).Scan(&job.upload, &job.asset, &job.purpose, &job.key, &job.mime, &job.size, &job.sha, &job.attempts)
	if err != nil {
		return job, err
	}
	job.lease = make([]byte, 16)
	if _, err = rand.Read(job.lease); err != nil {
		return job, err
	}
	res, err := tx.ExecContext(ctx, SQL.HGMediaReviewLeaseSQL, job.lease, job.upload)
	if err != nil {
		return job, err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return job, HGErrConflict
	}
	return job, tx.Commit()
}

func hgReviewValid(job hgReviewJob, result Transport.HGReviewResult) bool {
	if !hgValidReviewRequest(job) {
		return false
	}
	if result.Status != "ready" || result.SealedKey != "sealed/"+strings.ReplaceAll(job.asset, "-", "") || result.SHA256 != job.sha || result.Size != job.size || result.MIME != job.mime || !hgSealedKey.MatchString(result.SealedKey) {
		return false
	}
	if result.Width < 0 || result.Width > 16384 || result.Height < 0 || result.Height > 16384 || result.DurationMS < 0 || result.DurationMS > 4*60*60*1000 {
		return false
	}
	switch {
	case strings.HasPrefix(job.mime, "image/"):
		return result.Width > 0 && result.Height > 0 && result.DurationMS == 0
	case strings.HasPrefix(job.mime, "video/"):
		return result.Width > 0 && result.Height > 0 && result.DurationMS > 0
	case strings.HasPrefix(job.mime, "audio/"):
		return result.Width == 0 && result.Height == 0 && result.DurationMS > 0
	default:
		return result.Width == 0 && result.Height == 0 && result.DurationMS == 0
	}
}

func hgValidReviewRequest(job hgReviewJob) bool {
	policy, ok := hgPolicies[job.purpose]
	ext, allowed := policy.mimes[job.mime]
	if !ok || !allowed || uuid.Validate(job.asset) != nil || job.size < 1 || job.size > policy.max || job.key != fmt.Sprintf("quarantine/%s/%s/%s.%s", policy.biz, policy.kind, job.asset, ext) || len(job.sha) != 64 {
		return false
	}
	for _, c := range job.sha {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// HGReviewOnce 领取一件到期审核任务；外部调用发生在提交租约之后，失败重试不改变已有可用资源。
func (s *HGService) HGReviewOnce(ctx context.Context) error {
	if s.reviewer == nil || s.sealed == nil {
		return errors.New("可信媒体审核未配置")
	}
	job, err := s.hgClaimReview(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("领取媒体审核任务失败: %w", err)
	}
	state := "processing"
	var result Transport.HGReviewResult
	var reviewErr error
	if !hgValidReviewRequest(job) || job.attempts >= 12 {
		state = "failed"
	} else {
		result, reviewErr = s.reviewer.HGReview(ctx, strings.ReplaceAll(job.asset, "-", ""), s.bucket, job.key, job.sha, job.mime, job.size)
		if reviewErr == nil {
			switch result.Status {
			case "ready":
				if !hgReviewValid(job, result) {
					state = "failed"
				} else {
					var actual int64
					actual, reviewErr = s.sealed.HGHeadSealed(ctx, result.SealedKey)
					if reviewErr == nil && actual == job.size {
						state = "ready"
					} else if reviewErr == nil {
						state = "failed"
					}
				}
			case "rejected":
				state = "failed"
			case "pending":
			default:
				state = "failed"
			}
		}
		if state == "processing" && job.attempts+1 >= 12 {
			state = "failed"
		}
	}
	if state != "ready" {
		result = Transport.HGReviewResult{}
	}
	if err := s.hgFinishReview(ctx, job, state, result); err != nil {
		return fmt.Errorf("提交媒体审核结果失败: %w", err)
	}
	return reviewErr
}

func (s *HGService) hgFinishReview(ctx context.Context, job hgReviewJob, state string, result Transport.HGReviewResult) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	var lease []byte
	if err = tx.QueryRowContext(ctx, SQL.HGMediaReviewLockSQL, job.upload).Scan(&current, &lease); err != nil {
		return err
	}
	if current != "processing" || !bytes.Equal(lease, job.lease) {
		return HGErrConflict
	}
	backoff := min(30*(job.attempts+1), 300)
	res, err := tx.ExecContext(ctx, SQL.HGMediaReviewFinishSQL, state, backoff, job.upload, job.lease)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return HGErrConflict
	}
	if state != "processing" {
		var key any
		var width, height, duration any
		if state == "ready" {
			key, width, height, duration = result.SealedKey, result.Width, result.Height, result.DurationMS
		}
		res, err = tx.ExecContext(ctx, SQL.HGMediaReviewAssetSQL, state, key, width, height, duration, job.asset)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		if err != nil || n != 1 {
			return HGErrConflict
		}
	}
	return tx.Commit()
}
