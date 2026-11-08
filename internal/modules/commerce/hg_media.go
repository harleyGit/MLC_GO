package commerce

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	upload "MLC_GO/internal/pkg/upload"
	"context"
	"database/sql"
	"errors"
	"io"
	"time"
)

// HGMediaStore 使用独立 commerce_media 管理上传、归属、绑定与未用图片删除。
type HGMediaStore struct {
	db       *sql.DB          // 与商品创建事务共用的 MySQL 连接池。
	uploader *upload.Uploader // 沿用已配置的对象存储及内容魔数验证。
}

// HGNewMediaStore 只装配媒体依赖，不创建资源或执行迁移。
func HGNewMediaStore(db *sql.DB, uploader *upload.Uploader) *HGMediaStore {
	return &HGMediaStore{db: db, uploader: uploader}
}

// HGUpload 在短事务占用本人额度，对象写入位于事务之外，失败时尽力清理占位及对象。
func (s *HGMediaStore) HGUpload(ctx context.Context, user, ext string, reader io.Reader, size int64) (string, error) {
	if user == "" || size <= 0 || size > 5<<20 || !map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true}[ext] {
		return "", ErrHGInvalid
	}
	target, err := s.uploader.PrepareUploadTarget("commerce_product", ext)
	if err != nil {
		return "", ErrHGInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var merchant string
	if err = tx.QueryRowContext(ctx, q.HGCommerceMerchantLockSQL, user).Scan(&merchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrHGForbidden
		}
		return "", err
	}
	var count int
	if err = tx.QueryRowContext(ctx, q.HGCommerceMediaCountSQL, user).Scan(&count); err != nil {
		return "", err
	}
	if count >= 20 {
		return "", ErrHGConflict
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceMediaReserveSQL, user, target.FileURL, target.FilePath); err != nil {
		return "", hgDBError(err)
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, q.HGCommerceMediaRemovePendingSQL, target.FileURL, target.FileURL, user)
	}()
	if err = s.uploader.UploadFromReaderToKeyContext(ctx, reader, size, target.FilePath, ext); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.uploader.DeleteFileContext(cleanup, target.FilePath)
		return "", ErrHGInvalid
	}
	if err = hgExactlyOne(s.db.ExecContext(ctx, q.HGCommerceMediaReadySQL, target.FileURL, target.FileURL, user)); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.uploader.DeleteFileContext(cleanup, target.FilePath)
		return "", err
	}
	return target.FileURL, nil
}

// HGUploadCategory 审核员的类目图片与商家商品图片分别计额和绑定；对象上传在短事务之外。
// 调用方须先完成 IAM 审核权限检查；孤儿待绑定图片每管理员最多 20 张。
func (s *HGMediaStore) HGUploadCategory(ctx context.Context, user, ext string, reader io.Reader, size int64) (string, error) {
	if user == "" || size <= 0 || size > 5<<20 || !map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true}[ext] {
		return "", ErrHGInvalid
	}
	target, err := s.uploader.PrepareUploadTarget("commerce_category", ext)
	if err != nil {
		return "", ErrHGInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, q.HGCommerceCategoryMediaQuotaSeedSQL, user); err != nil {
		return "", err
	}
	var lockedUser string
	if err = tx.QueryRowContext(ctx, q.HGCommerceCategoryMediaQuotaLockSQL, user).Scan(&lockedUser); err != nil {
		return "", err
	}
	var count int
	if err = tx.QueryRowContext(ctx, q.HGCommerceCategoryMediaCountSQL, user).Scan(&count); err != nil {
		return "", err
	}
	if count >= 20 {
		return "", ErrHGConflict
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceCategoryMediaReserveSQL, user, target.FileURL, target.FilePath); err != nil {
		return "", hgDBError(err)
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, q.HGCommerceCategoryMediaRemovePendingSQL, target.FileURL, target.FileURL, user)
	}()
	if err = s.uploader.UploadFromReaderToKeyContext(ctx, reader, size, target.FilePath, ext); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.uploader.DeleteFileContext(cleanup, target.FilePath)
		return "", ErrHGInvalid
	}
	if err = hgExactlyOne(s.db.ExecContext(ctx, q.HGCommerceCategoryMediaReadySQL, target.FileURL, target.FileURL, user)); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.uploader.DeleteFileContext(cleanup, target.FilePath)
		return "", err
	}
	return target.FileURL, nil
}

// HGUnusedCategoryImages 返回本人未绑定的类目图片，供失败续编或释放上传配额。
func (s *HGMediaStore) HGUnusedCategoryImages(ctx context.Context, user string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, q.HGCommerceCategoryMediaUnusedSQL, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	urls := make([]string, 0, 20)
	for rows.Next() {
		var url string
		if err = rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, rows.Err()
}

// HGDeleteUnusedCategoryImage 仅允许上传者删除未绑定的类目图片，存储失败恢复可重试状态。
func (s *HGMediaStore) HGDeleteUnusedCategoryImage(ctx context.Context, user, url string) error {
	if user == "" || url == "" || len(url) > 1024 {
		return ErrHGInvalid
	}
	if err := hgExactlyOne(s.db.ExecContext(ctx, q.HGCommerceCategoryMediaClaimDeleteSQL, url, url, user)); err != nil && !errors.Is(err, ErrHGConflict) {
		return err
	}
	var key string
	if err := s.db.QueryRowContext(ctx, q.HGCommerceCategoryMediaKeySQL, url, url, user).Scan(&key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGConflict
		}
		return err
	}
	if err := s.uploader.DeleteFileContext(ctx, key); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, q.HGCommerceCategoryMediaUndoDeleteSQL, url, url, user)
		return err
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return hgExactlyOne(s.db.ExecContext(cleanup, q.HGCommerceCategoryMediaFinishDeleteSQL, url, url, user))
}

// HGUnused 返回当前用户最多20张尚未绑定商品的图片，防止浏览器离开后丢失清理入口。
func (s *HGMediaStore) HGUnused(ctx context.Context, user string) ([]string, error) {
	if user == "" {
		return nil, ErrHGForbidden
	}
	rows, err := s.db.QueryContext(ctx, q.HGCommerceMediaUnusedSQL, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	urls := make([]string, 0, 20)
	for rows.Next() {
		var url string
		if err = rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, rows.Err()
}

// HGDeleteUnused 按图片 URL 哈希及本人身份抢占删除权；已绑定图片不会被清理。
func (s *HGMediaStore) HGDeleteUnused(ctx context.Context, user, url string) error {
	if user == "" || url == "" || len(url) > 1024 {
		return ErrHGInvalid
	}
	// 已进入删除中的图片可安全重试：重复删对象由存储层按不存在视为成功。
	if err := hgExactlyOne(s.db.ExecContext(ctx, q.HGCommerceMediaClaimDeleteSQL, url, url, user)); err != nil && !errors.Is(err, ErrHGConflict) {
		return err
	}
	var key string
	if err := s.db.QueryRowContext(ctx, q.HGCommerceMediaKeySQL, url, url, user).Scan(&key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGConflict
		}
		return err
	}
	if err := s.uploader.DeleteFileContext(ctx, key); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, q.HGCommerceMediaUndoDeleteSQL, url, url, user)
		return err
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return hgExactlyOne(s.db.ExecContext(cleanup, q.HGCommerceMediaFinishDeleteSQL, url, url, user))
}
