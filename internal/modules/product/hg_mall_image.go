package product

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	resp "MLC_GO/internal/response"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// HGImageStore 按本人商家身份管理有界、一次性可绑定的商品图片。
type HGImageStore struct {
	db       *sql.DB
	uploader *HGUploadPackage.Uploader
}

func HGNewImageStore(db *sql.DB, uploader *HGUploadPackage.Uploader) *HGImageStore {
	return &HGImageStore{db: db, uploader: uploader}
}

// HGUploadImage 在上传前登记，失败删除占位和对象；对象 I/O 永远不在商品事务中。
func (s *HGImageStore) HGUploadImage(ctx context.Context, user, ext string, reader io.Reader, size int64) (string, error) {
	if user == "" || size <= 0 || size > 5<<20 || !map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true}[ext] {
		return "", ErrHGInvalid
	}
	target, err := s.uploader.PrepareUploadTarget("mall_product", ext)
	if err != nil {
		return "", ErrHGInvalid
	}
	// 锁定本店记录并在同一短事务内占用配额，并发上传同一店铺时不会越过20张上限。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var merchantID string
	if err := tx.QueryRowContext(ctx, q.HGMallMerchantApprovedForUpdateSQL, user).Scan(&merchantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrHGForbidden
		}
		return "", err
	}
	result, err := tx.ExecContext(ctx, q.HGMallImageReserveSQL, target.FileURL, target.FilePath, user, user)
	if err != nil {
		return "", err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if n != 1 {
		return "", ErrHGConflict
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, q.HGMallImageRemoveSQL, target.FileURL, target.FileURL, user)
	}()
	if err = s.uploader.UploadFromReaderToKeyContext(ctx, reader, size, target.FilePath, ext); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.uploader.DeleteFileContext(cleanup, target.FilePath)
		return "", ErrHGInvalid
	}
	if err = hgOne(s.db.ExecContext(ctx, q.HGMallImageReadySQL, target.FileURL, target.FileURL, user)); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.uploader.DeleteFileContext(cleanup, target.FilePath)
		return "", err
	}
	return target.FileURL, nil
}

// HGImage 将原始图片二进制写入配置化存储，大小/扩展名/魔数由处理链双重约束。
func (h *HGHandler) HGImage(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ext := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("ext")))
	if r.ContentLength <= 0 || r.ContentLength > 5<<20 || h.images == nil {
		hgError(w, r, ErrHGInvalid)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	url, err := h.images.HGUploadImage(ctx, user, ext, r.Body, r.ContentLength)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ImageURL string `json:"imageURL"`
	}{url})
}

// HGDeleteUnusedImage 仅允许本人删除尚未绑定商品的图片，避免部分上传成功后永久占用限额。
func (h *HGHandler) HGDeleteUnusedImage(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var request struct {
		ImageURL string `json:"imageURL"`
	}
	if !hgDecode(w, r, &request) {
		return
	}
	if h.images == nil || request.ImageURL == "" || len(request.ImageURL) > 1024 {
		hgError(w, r, ErrHGInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), HGTimeLimit)
	defer cancel()
	if err := h.images.HGDeleteUnusedImage(ctx, user, request.ImageURL); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, json.RawMessage(`{}`))
}

func (s *HGImageStore) HGDeleteUnusedImage(ctx context.Context, user, url string) error {
	if err := hgOne(s.db.ExecContext(ctx, q.HGMallImageClaimDeleteSQL, url, url, user)); err != nil && !errors.Is(err, ErrHGConflict) {
		return err
	}
	var key string
	if err := s.db.QueryRowContext(ctx, q.HGMallImageKeySQL, url, url, user).Scan(&key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGConflict
		}
		return err
	}
	if err := s.uploader.DeleteFileContext(ctx, key); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, q.HGMallImageUndoDeleteSQL, url, url, user)
		return err
	}
	return hgOne(s.db.ExecContext(ctx, q.HGMallImageFinishDeleteSQL, url, url, user))
}

// HGUnusedImages 返回本人尚未发布的有界图片列表，供重新进入页面时释放上传额度。
func (h *HGHandler) HGUnusedImages(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), HGTimeLimit)
	defer cancel()
	rows, err := h.images.db.QueryContext(ctx, q.HGMallImageUnusedListSQL, user)
	if err != nil {
		hgError(w, r, err)
		return
	}
	defer rows.Close()
	urls := make([]string, 0, 20)
	for rows.Next() {
		var url string
		if err = rows.Scan(&url); err != nil {
			hgError(w, r, err)
			return
		}
		urls = append(urls, url)
	}
	if err = rows.Err(); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ImageURLs []string `json:"imageURLs"`
	}{urls})
}
