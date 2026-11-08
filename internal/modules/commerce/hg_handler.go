package commerce

import (
	user "MLC_GO/internal/pkg/hg_context"
	resp "MLC_GO/internal/response"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HGHandler 实现生产 CNY 商品发布及 debug 受控订单；生产/预发的交易写入 fail-closed。
type HGHandler struct {
	service *HGService    // 发布、审核与归属校验入口。
	media   *HGMediaStore // 独立生产 CNY 图片资产管理入口。
	orders  *HGOrderStore // 与商品、统一订单共用的 MySQL 池；nil 表示交易入口不可用。
}

// HGNewHandler 装配商品和媒体入口，不访问外部服务。
func HGNewHandler(service *HGService, media *HGMediaStore) *HGHandler {
	return &HGHandler{service: service, media: media}
}

// HGWithOrderStore 仅装配共库交易存储，不放宽环境或支付权限。
func (h *HGHandler) HGWithOrderStore(store *HGOrderStore) *HGHandler { h.orders = store; return h }

func hgAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := user.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		resp.FailTokenInvalid(w, r, "登录状态无效")
	}
	return id, ok
}

func hgDecode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		hgError(w, r, ErrHGInvalid)
		return false
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		hgError(w, r, ErrHGInvalid)
		return false
	}
	return true
}

func hgError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := http.StatusInternalServerError, "人民币商品服务暂不可用"
	switch {
	case errors.Is(err, ErrHGInvalid):
		status, message = http.StatusBadRequest, "商品参数无效"
	case errors.Is(err, ErrHGForbidden):
		status, message = http.StatusForbidden, resp.HGPermissionError("需具备相应操作权限或满足本人商品/订单归属条件", "").Message
	case errors.Is(err, ErrHGMerchantRequired):
		status, message = http.StatusForbidden, resp.HGPermissionError("需要已批准的商家身份（mall_merchant.status=1）；平台类目请由具备 mall:merchant:review 权限的审核员使用 GET /api/v1/commerce/admin/categories", "").Message
	case errors.Is(err, ErrHGConflict):
		status, message = http.StatusConflict, "商品状态、规格或版本已变化"
	case errors.Is(err, ErrHGStock):
		status, message = http.StatusConflict, "库存不足或商品不可购买"
	case errors.Is(err, ErrHGPaymentUnavailable):
		status, message = http.StatusServiceUnavailable, "交易渠道未验收，当前操作未开放"
	case errors.Is(err, sql.ErrNoRows):
		status, message = http.StatusNotFound, "商品或类目不存在"
	}
	code := resp.InvalidParam.Code
	if status == http.StatusForbidden {
		code = resp.Forbidden.Code
	}
	if status >= 500 {
		code = resp.InternalError.Code
	}
	w.WriteHeader(status)
	resp.FailResult[string](w, r, resp.HGErrorResult{Code: code, Message: message})
}

func hgContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

func hgVersion(text string) (uint64, error) {
	if text == "" || len(text) > 20 || text[0] == '0' {
		return 0, ErrHGInvalid
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, ErrHGInvalid
		}
	}
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil || v == 0 {
		return 0, ErrHGInvalid
	}
	return v, nil
}

// HGCategories 获取当前商家可经营的普通叶子类目；未申请资质时不可发布受监管商品。
func (h *HGHandler) HGCategories(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	items, err := h.service.HGCategories(ctx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, items)
}

// HGAdminCategories 供具备商城审核权限的管理员读取平台类目，不要求商家入驻。
func (h *HGHandler) HGAdminCategories(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	items, err := h.service.HGAdminCategories(ctx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, items)
}

// HGCreateCategory 由具有商城审核权的管理员维护平台类目，禁止客户端伪造资质。
func (h *HGHandler) HGCreateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGCategoryRequest
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	item, err := h.service.HGCreateCategory(ctx, id, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, item)
}

// HGCreateDraft 绑定商家幂等键；同键不同正文不创建第二个商品。
func (h *HGHandler) HGCreateDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if !hgID(key) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	var req HGDraftInput
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	item, err := h.service.HGCreateDraft(ctx, id, key, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, item)
}

// HGEditDraft 仅允许本人按版本修改草稿或生成新的待审内容版本。
func (h *HGHandler) HGEditDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	productID := r.URL.Query().Get("productId")
	var req HGDraftInput
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	item, err := h.service.HGEditDraft(ctx, id, productID, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, item)
}

func hgDecisionBody(w http.ResponseWriter, r *http.Request) (string, uint64, bool) {
	var req struct {
		ProductID       string `json:"productId"`
		ExpectedVersion uint64 `json:"expectedVersion"`
	}
	if !hgDecode(w, r, &req) {
		return "", 0, false
	}
	if !hgID(req.ProductID) || req.ExpectedVersion == 0 {
		hgError(w, r, ErrHGInvalid)
		return "", 0, false
	}
	return req.ProductID, req.ExpectedVersion, true
}

// HGSubmit 把本人草稿提交审核；上架必须在审批成功后另行执行。
func (h *HGHandler) HGSubmit(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	productID, version, ok := hgDecisionBody(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.service.HGSubmit(ctx, id, productID, version); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

// HGReview 严格校验审核权和自审；拒绝必须包含原因，审核记录写入版本快照。
func (h *HGHandler) HGReview(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ProductID string `json:"productId"`
		Approve   bool   `json:"approve"`
		Reason    string `json:"reason"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.service.HGReview(ctx, id, req.ProductID, req.Approve, req.Reason); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

// HGPublish 已审批商品明确上架；审核期间旧的在售版本可继续展示。
func (h *HGHandler) HGPublish(w http.ResponseWriter, r *http.Request) { h.hgSale(w, r, true) }

// HGUnpublish 本店下架，不改变历史订单快照。
func (h *HGHandler) HGUnpublish(w http.ResponseWriter, r *http.Request) { h.hgSale(w, r, false) }

// HGArchive 永久停止本人商品公开展示，保留审核历史与图片绑定，不提供恢复入口。
func (h *HGHandler) HGArchive(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	productID, version, ok := hgDecisionBody(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.service.HGArchive(ctx, id, productID, version); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

func (h *HGHandler) hgSale(w http.ResponseWriter, r *http.Request, publish bool) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	productID, version, ok := hgDecisionBody(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.service.HGSetSale(ctx, id, productID, version, publish); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

func (h *HGHandler) hgList(w http.ResponseWriter, r *http.Request, scope string) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	page, err := h.service.HGList(ctx, id, r.URL.Query().Get("cursor"), scope)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

// HGList 仅展示在售且有已审核版本的生产人民币商品，不支持结算。
func (h *HGHandler) HGList(w http.ResponseWriter, r *http.Request) { h.hgList(w, r, "public") }

// HGMine 本店按 ID 倒序游标读取所有审核状态商品，最多20条。
func (h *HGHandler) HGMine(w http.ResponseWriter, r *http.Request) { h.hgList(w, r, "mine") }

// HGPending 仅平台审核员读取待审有界列表。
func (h *HGHandler) HGPending(w http.ResponseWriter, r *http.Request) { h.hgList(w, r, "review") }

func (h *HGHandler) hgDetail(w http.ResponseWriter, r *http.Request, scope string) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	item, err := h.service.HGDetail(ctx, id, r.URL.Query().Get("productId"), scope)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, item)
}

// HGDetail 只读取线上版本。
func (h *HGHandler) HGDetail(w http.ResponseWriter, r *http.Request) { h.hgDetail(w, r, "public") }

// HGMineDetail 本人可读取尚未审核的完整草稿。
func (h *HGHandler) HGMineDetail(w http.ResponseWriter, r *http.Request) { h.hgDetail(w, r, "mine") }

// HGPendingDetail 供审核员查看待审版本，不按公开目录透出。
func (h *HGHandler) HGPendingDetail(w http.ResponseWriter, r *http.Request) {
	h.hgDetail(w, r, "review")
}

// HGImage 上传最多5 MiB 图片；商家上传并不自动通过商品审核。
func (h *HGHandler) HGImage(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ext := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("ext")))
	if r.ContentLength <= 0 || r.ContentLength > 5<<20 {
		hgError(w, r, ErrHGInvalid)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	url, err := h.media.HGUpload(ctx, id, ext, r.Body, r.ContentLength)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ImageURL string `json:"imageURL"`
	}{url})
}

// HGUnusedImages 本人尚未绑定的图片列表，避免上传失败或离开页面后占满额度。
func (h *HGHandler) HGUnusedImages(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	urls, err := h.media.HGUnused(ctx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ImageURLs []string `json:"imageURLs"`
	}{urls})
}

// HGDeleteUnusedImage 仅允许删除本人尚未绑定商品的临时图片。
func (h *HGHandler) HGDeleteUnusedImage(w http.ResponseWriter, r *http.Request) {
	id, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ImageURL string `json:"imageURL"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.media.HGDeleteUnused(ctx, id, req.ImageURL); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}
