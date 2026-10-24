package product

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	resp "MLC_GO/internal/response"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
)

// HGMediaItem 是已上架商品待审补图的审核协议；每张图片单独审核，拒绝后回到商家未使用图片。
type HGMediaItem struct {
	ID        uint64 `json:"-"`         // 内部图片游标，不暴露给客户端。
	ProductID string `json:"productId"` // 被补图的已上架商品业务 ID。
	ImageURL  string `json:"imageURL"`  // 待审图片 URL，由商家本人上传。
	Title     string `json:"title"`     // 审核员辨识商品用标题。
	ShopName  string `json:"shopName"`  // 审核员辨识商家用店铺名。
	UserID    string `json:"-"`         // 商家所有人，只用于服务端防止自审。
}

// HGMediaPage 按图片内部 ID 倒序分页；过滤本人申请后保留原游标防止漏审。
type HGMediaPage struct {
	List       []HGMediaItem `json:"list"`       // 当前最多20张待审图片。
	NextCursor string        `json:"nextCursor"` // 下一页游标；空表示无下一页。
	HasMore    bool          `json:"hasMore"`    // 是否还有下一页。
}

// StageMedia 只锁定本人已上架商品，校验额度并一次性占用本人未使用图片；事务外已完成文件上传。
func (r *HGRepository) StageMedia(ctx context.Context, owner, productID string, urls []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, q.HGMallMediaOwnProductSQL, productID, owner).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	var published []string
	if err = json.Unmarshal(raw, &published); err != nil {
		return err
	}
	var pending int
	if err = tx.QueryRowContext(ctx, q.HGMallMediaPendingCountSQL, productID).Scan(&pending); err != nil {
		return err
	}
	if len(published)+pending+len(urls) > 5 {
		return ErrHGConflict
	}
	seen := make(map[string]bool, len(urls))
	for _, url := range urls {
		if len(url) == 0 || len(url) > 1024 || seen[url] {
			return ErrHGInvalid
		}
		seen[url] = true
		if err = hgOne(tx.ExecContext(ctx, q.HGMallMediaStageSQL, productID, url, url, owner)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PendingMedia 以图片状态索引取21条，审核者本人申请在服务层过滤。
func (r *HGRepository) PendingMedia(ctx context.Context, cursor uint64) ([]HGMediaItem, error) {
	if cursor == 0 {
		cursor = math.MaxUint64
	}
	rows, err := r.db.QueryContext(ctx, q.HGMallMediaPendingSQL, cursor, 21)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGMediaItem, 0, 21)
	for rows.Next() {
		var item HGMediaItem
		if err = rows.Scan(&item.ID, &item.ProductID, &item.ImageURL, &item.Title, &item.ShopName, &item.UserID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ReviewMedia 锁住商品后原子修改图片状态及公开目录 JSON；拒绝不修改已上架商品。
func (r *HGRepository) ReviewMedia(ctx context.Context, reviewer, url string, approve bool) error {
	var productID string
	if err := r.db.QueryRowContext(ctx, q.HGMallMediaTargetSQL, url, url).Scan(&productID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGConflict
		}
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, q.HGMallMediaReviewerProductSQL, productID, reviewer).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if !approve {
		if err = hgOne(tx.ExecContext(ctx, q.HGMallMediaRejectImageSQL, url, url, productID)); err != nil {
			return err
		}
		return tx.Commit()
	}
	var published []string
	if err = json.Unmarshal(raw, &published); err != nil {
		return err
	}
	if len(published) >= 5 {
		return ErrHGConflict
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGMallMediaApproveImageSQL, url, url, productID)); err != nil {
		return err
	}
	published = append(published, url)
	imageJSON, err := json.Marshal(published)
	if err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGMallMediaPublishSQL, imageJSON, productID)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGStageMedia 把本人已上架商品补图送入审核；商品状态和现有订单不受影响。
func (s *HGService) HGStageMedia(ctx context.Context, user, productID string, urls []string) error {
	if user == "" || !hgBusinessID(productID) || len(urls) == 0 || len(urls) > 5 {
		return ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	return s.store.StageMedia(ctx, user, productID, urls)
}

func (s *HGService) HGPendingMedia(ctx context.Context, user, cursor string) (HGMediaPage, error) {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return HGMediaPage{}, err
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGMediaPage{}, err
	}
	items, err := s.store.PendingMedia(ctx, id)
	if err != nil {
		return HGMediaPage{}, err
	}
	page := HGMediaPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = strconv.FormatUint(page.List[19].ID, 10)
	}
	visible := make([]HGMediaItem, 0, len(page.List))
	for _, item := range page.List {
		if item.UserID != user {
			visible = append(visible, item)
		}
	}
	page.List = visible
	return page, nil
}

func (s *HGService) HGReviewMedia(ctx context.Context, user, url string, approve bool) error {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return err
	}
	if url == "" || len(url) > 1024 {
		return ErrHGInvalid
	}
	return s.store.ReviewMedia(ctx, user, url, approve)
}

func (h *HGHandler) HGStageMedia(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ProductID string   `json:"productId"` // 本人已上架商品。
		ImageURLs []string `json:"imageURLs"` // 本人上传的至多5张未使用图片。
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGStageMedia(r.Context(), user, req.ProductID, req.ImageURLs); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ProductID string `json:"productId"`
	}{req.ProductID})
}

func (h *HGHandler) HGPendingMedia(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.HGPendingMedia(r.Context(), user, r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGReviewMedia(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ImageURL string `json:"imageURL"` // 待审图片 URL；必须与登记的状态和商品一致。
		Approve  *bool  `json:"approve"`  // 明确的审核决定；缺省拒绝处理。
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if req.Approve == nil {
		hgError(w, r, ErrHGInvalid)
		return
	}
	if err := h.service.HGReviewMedia(r.Context(), user, req.ImageURL, *req.Approve); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ImageURL string `json:"imageURL"`
	}{req.ImageURL})
}
