package media

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HGService 管理媒体隔离上传会话；不会将未经审核的对象升级为可引用资源。
type HGService struct {
	db     *sql.DB       // MySQL 权威会话与元数据，事务不可跨 S3 网络 I/O。
	store  HGObjectStore // SeaweedFS 隔离桶访问，必须复用持久客户端。
	bucket string        // 与 store 一致的受信隔离桶名。
}

// HGNewService 构造未审核上传服务；bucket 必须与对象存储实例配置相同。
func HGNewService(db *sql.DB, store HGObjectStore, bucket string) (*HGService, error) {
	if db == nil || store == nil || !hgBucketName.MatchString(bucket) {
		return nil, HGErrInvalid
	}
	return &HGService{db: db, store: store, bucket: bucket}, nil
}

// HGUploadSession 是直传初始化响应；URL 只允许在隔离桶内短时使用，不表示业务资源可用。
type HGUploadSession struct {
	UploadID        string            `json:"upload_id"`                  // 会话 UUID，由服务端生成。
	AssetID         string            `json:"asset_id"`                   // 未审核资源 UUID，不允许直接用于业务引用。
	Status          string            `json:"status"`                     // pending/processing，仅 processing 表示对象大小已核对。
	Method          string            `json:"method,omitempty"`           // PUT，仅 pending 会话签发。
	UploadURL       string            `json:"upload_url,omitempty"`       // 10 分钟有效的隔离桶预签名 URL，不能记录日志。
	RequiredHeaders map[string]string `json:"required_headers,omitempty"` // PUT 必须发送的 MIME 头。
	ExpiresIn       int               `json:"expires_in,omitempty"`       // 上传授权的有效秒数，非会话保留时间。
}

type hgSavedSession struct {
	id      string    // 会话 UUID。
	asset   string    // 资源 UUID。
	purpose string    // 业务用途。
	mime    string    // 申请 MIME。
	size    int64     // 申请字节数。
	sha     string    // 申请摘要。
	key     string    // 隔离区随机对象键。
	status  string    // pending 或 processing。
	expires time.Time // 会话过期时间，数据库时区 UTC。
}

// HGInit 创建幂等上传会话：同一调用人及重试键必须有相同业务语义；活动会话数和字节数有界。
func (s *HGService) HGInit(ctx context.Context, owner string, req HGUploadRequest) (HGUploadSession, error) {
	policy, ext, err := hgValidateRequest(req)
	if err != nil || owner == "" || len(owner) > 64 {
		return HGUploadSession{}, HGErrInvalid
	}
	// 先签名随机键并不写存储；真正进入事务后才持久化该键。
	asset := uuid.NewString()
	id := uuid.NewString()
	key := fmt.Sprintf("quarantine/%s/%s/%s.%s", policy.biz, policy.kind, asset, ext)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGUploadSession{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, SQL.HGMediaQuotaCreateSQL, owner); err != nil {
		return HGUploadSession{}, err
	}
	var locked string
	if err = tx.QueryRowContext(ctx, SQL.HGMediaQuotaLockSQL, owner).Scan(&locked); err != nil {
		return HGUploadSession{}, err
	}
	var existing hgSavedSession
	err = tx.QueryRowContext(ctx, SQL.HGMediaFindIdempotencySQL, owner, req.IdempotencyKey).Scan(&existing.id, &existing.asset, &existing.purpose, &existing.mime, &existing.size, &existing.sha, &existing.key, &existing.status, &existing.expires)
	if err == nil {
		if existing.purpose != req.Purpose || existing.mime != req.MIME || existing.size != req.Size || existing.sha != req.SHA256 {
			return HGUploadSession{}, HGErrConflict
		}
		if err = tx.Commit(); err != nil {
			return HGUploadSession{}, err
		}
		return s.hgSessionResponse(ctx, existing)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGUploadSession{}, err
	}
	rows, err := tx.QueryContext(ctx, SQL.HGMediaReservationSQL, owner)
	if err != nil {
		return HGUploadSession{}, err
	}
	var count, sum int64
	for rows.Next() {
		var n int64
		if err = rows.Scan(&n); err != nil {
			break
		}
		count++
		sum += n
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return HGUploadSession{}, err
	}
	if count >= 20 || sum > (1<<30)-req.Size {
		return HGUploadSession{}, HGErrConflict
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaInsertAssetSQL, asset, owner, req.Purpose, s.bucket, key, req.MIME, req.Size, req.SHA256); err != nil {
		return HGUploadSession{}, err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaInsertSessionSQL, id, asset, owner, req.IdempotencyKey, req.Purpose, req.MIME, req.Size, req.SHA256, key); err != nil {
		return HGUploadSession{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGUploadSession{}, err
	}
	return s.hgSessionResponse(ctx, hgSavedSession{id: id, asset: asset, key: key, mime: req.MIME, status: "pending", expires: time.Now().Add(24 * time.Hour)})
}

func (s *HGService) hgSessionResponse(ctx context.Context, session hgSavedSession) (HGUploadSession, error) {
	result := HGUploadSession{UploadID: session.id, AssetID: session.asset, Status: session.status}
	if session.status != "pending" || !time.Now().Before(session.expires) {
		return result, nil
	}
	url, err := s.store.HGSignUpload(ctx, session.key, session.mime, session.size)
	if err != nil {
		return HGUploadSession{}, err
	}
	result.Method, result.UploadURL, result.ExpiresIn = "PUT", url, 600
	result.RequiredHeaders = map[string]string{"Content-Type": session.mime}
	return result, nil
}

// HGComplete 核对本人会话及 S3 真实长度；不做内容审核，不允许将资源标记 ready。
func (s *HGService) HGComplete(ctx context.Context, owner, uploadID string) (HGUploadSession, error) {
	if owner == "" || len(owner) > 64 || uuid.Validate(uploadID) != nil {
		return HGUploadSession{}, HGErrInvalid
	}
	// 不在事务中执行外部 Head；并发 PUT 仍可能发生，所以只能转 processing，不能转 ready。
	var asset, key, status string
	var size int64
	var expires time.Time
	err := s.db.QueryRowContext(ctx, SQL.HGMediaFindOwnedSessionSQL, uploadID, owner).Scan(&asset, &key, &size, &status, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return HGUploadSession{}, HGErrNotFound
	}
	if err != nil {
		return HGUploadSession{}, err
	}
	result := HGUploadSession{UploadID: uploadID, AssetID: asset, Status: status}
	if status == "processing" {
		return result, nil
	}
	if status != "pending" || !time.Now().Before(expires) {
		return HGUploadSession{}, HGErrConflict
	}
	actual, err := s.store.HGHead(ctx, key)
	if err != nil {
		return HGUploadSession{}, err
	}
	if actual != size {
		return HGUploadSession{}, HGErrConflict
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGUploadSession{}, err
	}
	defer tx.Rollback()
	var lockedAsset, lockedOwner, purpose, mime, lockedKey, lockedStatus string
	var lockedSize int64
	if err = tx.QueryRowContext(ctx, SQL.HGMediaLockSessionSQL, uploadID).Scan(&lockedAsset, &lockedOwner, &purpose, &mime, &lockedSize, &lockedKey, &lockedStatus, &expires); errors.Is(err, sql.ErrNoRows) {
		return HGUploadSession{}, HGErrNotFound
	} else if err != nil {
		return HGUploadSession{}, err
	}
	if lockedOwner != owner || lockedAsset != asset || lockedKey != key || lockedSize != size {
		return HGUploadSession{}, HGErrNotFound
	}
	if lockedStatus == "processing" {
		return result, nil
	}
	if lockedStatus != "pending" || !time.Now().Before(expires) {
		return HGUploadSession{}, HGErrConflict
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaProcessSessionSQL, uploadID); err != nil {
		return HGUploadSession{}, err
	}
	res, err := tx.ExecContext(ctx, SQL.HGMediaProcessAssetSQL, asset)
	if err != nil {
		return HGUploadSession{}, err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return HGUploadSession{}, HGErrConflict
	}
	if err = tx.Commit(); err != nil {
		return HGUploadSession{}, err
	}
	result.Status = "processing"
	return result, nil
}

// HGAssetStatus 仅返回本人隔离资源的状态和声明元数据，不返回桶、对象键或未审核下载地址。
func (s *HGService) HGAssetStatus(ctx context.Context, owner, asset string) (map[string]any, error) {
	if owner == "" || len(owner) > 64 || uuid.Validate(asset) != nil {
		return nil, HGErrInvalid
	}
	var purpose, mime, status string
	var size int64
	err := s.db.QueryRowContext(ctx, SQL.HGMediaOwnerAssetSQL, asset, owner).Scan(&purpose, &mime, &size, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, HGErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"asset_id": asset, "purpose": purpose, "mime_type": mime, "file_size": size, "status": status}, nil
}
