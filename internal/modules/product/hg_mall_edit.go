package product

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	resp "MLC_GO/internal/response"
	"context"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

// HGEditRequest 是本人已上架商品文字编辑请求；不改变价格、库存、SKU 或已产生的订单快照。
type HGEditRequest struct {
	ProductID   string `json:"productId"`   // 本人已上架的商品业务 ID。
	Title       string `json:"title"`       // 1-128 字符，管理员审核后才公开。
	Summary     string `json:"summary"`     // 1-512 字符纯文本。
	Description string `json:"description"` // 0-4096 字符纯文本，空表示不展示额外介绍。
}

// HGEditItem 是已上架商品待审文字变更；申请人只用于防止管理员自审。
type HGEditItem struct {
	ID uint64 `json:"-"` // 内部审核游标。
	HGEditRequest
	ShopName string `json:"shopName"` // 店铺展示名。
	UserID   string `json:"-"`        // 所有人业务 ID，不对审核前端开放。
}

// HGEditPage 按申请 ID 倒序游标分页，不统计大表总量。
type HGEditPage struct {
	List       []HGEditItem `json:"list"`       // 当前最多20件待审编辑。
	NextCursor string       `json:"nextCursor"` // 下一页位置，空表示没有下一页。
	HasMore    bool         `json:"hasMore"`    // 是否存在下一页。
}

// StageEdit 在商品行锁保护下只写待审数据，重复提交待审版本返回冲突。
func (r *HGRepository) StageEdit(ctx context.Context, owner string, req HGEditRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var productID string
	if err = tx.QueryRowContext(ctx, q.HGMallEditOwnSQL, req.ProductID, owner).Scan(&productID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	var status int8
	err = tx.QueryRowContext(ctx, q.HGMallEditStatusSQL, req.ProductID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, q.HGMallEditInsertSQL, req.ProductID, owner, req.Title, req.Summary, req.Description); err != nil {
			return hgCatalogDBError(err)
		}
	} else if err != nil {
		return err
	} else if status == 0 {
		return ErrHGConflict
	} else if err = hgOne(tx.ExecContext(ctx, q.HGMallEditResubmitSQL, owner, req.Title, req.Summary, req.Description, req.ProductID)); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingEdits 依状态索引有界读取管理员工作台待审文字变更。
func (r *HGRepository) PendingEdits(ctx context.Context, cursor uint64) ([]HGEditItem, error) {
	if cursor == 0 {
		cursor = math.MaxUint64
	}
	rows, err := r.db.QueryContext(ctx, q.HGMallEditPendingSQL, cursor, 21)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGEditItem, 0, 21)
	for rows.Next() {
		var item HGEditItem
		if err = rows.Scan(&item.ID, &item.ProductID, &item.Title, &item.Summary, &item.Description, &item.ShopName, &item.UserID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ReviewEdit 先锁商品再锁审核记录；批准时同时更新公开文字和审核状态，拒绝时目录不变。
func (r *HGRepository) ReviewEdit(ctx context.Context, reviewer, productID string, approve bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lockedID string
	if err = tx.QueryRowContext(ctx, q.HGMallEditReviewerProductSQL, productID, reviewer).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	var title, summary, description string
	if err = tx.QueryRowContext(ctx, q.HGMallEditPendingOneSQL, productID).Scan(&title, &summary, &description); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGConflict
		}
		return err
	}
	status := int8(2)
	if approve {
		status = 1
		if _, err = tx.ExecContext(ctx, q.HGMallEditPublishSQL, title, summary, description, productID); err != nil {
			return err
		}
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGMallEditDecisionSQL, status, reviewer, productID)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGStageEdit 仅向本人已上架商品提交纯文本审核，不允许 HTML 或无界文字。
func (s *HGService) HGStageEdit(ctx context.Context, user string, req HGEditRequest) error {
	if user == "" || !hgBusinessID(req.ProductID) || !hgText(req.Title, 128) || !hgText(req.Summary, 512) || !utf8.ValidString(req.Description) || utf8.RuneCountInString(req.Description) > 4096 || strings.ContainsAny(req.Description, "<>\x00") {
		return ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	return s.store.StageEdit(ctx, user, req)
}

func (s *HGService) HGPendingEdits(ctx context.Context, user, cursor string) (HGEditPage, error) {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return HGEditPage{}, err
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGEditPage{}, err
	}
	items, err := s.store.PendingEdits(ctx, id)
	if err != nil {
		return HGEditPage{}, err
	}
	page := HGEditPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = strconv.FormatUint(page.List[19].ID, 10)
	}
	visible := make([]HGEditItem, 0, len(page.List))
	for _, item := range page.List {
		if item.UserID != user {
			visible = append(visible, item)
		}
	}
	page.List = visible
	return page, nil
}

func (s *HGService) HGReviewEdit(ctx context.Context, user, productID string, approve bool) error {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return err
	}
	if !hgBusinessID(productID) {
		return ErrHGInvalid
	}
	return s.store.ReviewEdit(ctx, user, productID, approve)
}

func (h *HGHandler) HGStageEdit(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGEditRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGStageEdit(r.Context(), user, req); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ProductID string `json:"productId"`
	}{req.ProductID})
}

func (h *HGHandler) HGPendingEdits(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.HGPendingEdits(r.Context(), user, r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGReviewEdit(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ProductID string `json:"productId"` // 待审商品 ID。
		Approve   *bool  `json:"approve"`   // 必填，管理员明确批准或拒绝。
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if req.Approve == nil {
		hgError(w, r, ErrHGInvalid)
		return
	}
	if err := h.service.HGReviewEdit(r.Context(), user, req.ProductID, *req.Approve); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ProductID string `json:"productId"`
	}{req.ProductID})
}
