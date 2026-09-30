package handler

import (
	"MLC_GO/internal/modules/wallet/dto"
	"MLC_GO/internal/modules/wallet/model"
	"MLC_GO/internal/modules/wallet/service"
	hgcontext "MLC_GO/internal/pkg/hg_context"
	response "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// HGHandler 是用户钱包HTTP边界，所有用户标识均从JWT提取。
type HGHandler struct{ hgService *service.HGService }

// HGNewHandler 注入钱包服务，不执行外部IO。
func HGNewHandler(hgService *service.HGService) *HGHandler { return &HGHandler{hgService: hgService} }

// HGEnsureUser 仅接受JWT上下文身份，缺失身份时返回统一未登录响应。
func (hgHandler *HGHandler) HGEnsureUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	hgUser, hgOK := hgcontext.CurrentUserID(r)
	if !hgOK {
		w.WriteHeader(http.StatusUnauthorized)
		response.FailTokenInvalid(w, r, "请先登录")
	}
	return hgUser, hgOK
}

// HGBalance 查询当前用户的权威余额，以十进制字符串返回。
func (hgHandler *HGHandler) HGBalance(w http.ResponseWriter, r *http.Request) {
	hgUser, hgOK := hgHandler.HGEnsureUser(w, r)
	if !hgOK {
		return
	}
	hgResult, hgErr := hgHandler.hgService.HGBalance(r.Context(), hgUser)
	if hgErr != nil {
		hgWalletError(w, r, hgErr)
		return
	}
	response.SuccessResult(w, r, hgResult)
}

// HGSKUs 返回已登录用户可见的有效档位游标页，不调用运维权限接口。
func (hgHandler *HGHandler) HGSKUs(w http.ResponseWriter, r *http.Request) {
	if _, hgOK := hgHandler.HGEnsureUser(w, r); !hgOK {
		return
	}
	hgResult, hgErr := hgHandler.hgService.HGListSKUs(r.Context(), r.URL.Query().Get("cursor"), r.URL.Query().Get("pageSize"))
	if hgErr != nil {
		hgWalletError(w, r, hgErr)
		return
	}
	response.SuccessResult(w, r, hgResult)
}

// HGCreateOrder 只接收档位和幂等键，由服务端创建本人订单快照。
func (hgHandler *HGHandler) HGCreateOrder(w http.ResponseWriter, r *http.Request) {
	hgUser, hgOK := hgHandler.HGEnsureUser(w, r)
	if !hgOK {
		return
	}
	var hgRequest dto.HGCreateRequest
	if !hgDecode(w, r, &hgRequest) {
		return
	}
	hgResult, hgErr := hgHandler.hgService.HGCreateOrder(r.Context(), hgUser, hgRequest)
	if hgErr != nil {
		hgWalletError(w, r, hgErr)
		return
	}
	response.SuccessResult(w, r, hgResult)
}

// HGOrderDetail 按JWT身份查询本人订单，不存在与无权访问使用相同响应。
func (hgHandler *HGHandler) HGOrderDetail(w http.ResponseWriter, r *http.Request) {
	hgUser, hgOK := hgHandler.HGEnsureUser(w, r)
	if !hgOK {
		return
	}
	hgResult, hgErr := hgHandler.hgService.HGOrderDetail(r.Context(), hgUser, r.URL.Query().Get("orderId"))
	if hgErr != nil {
		hgWalletError(w, r, hgErr)
		return
	}
	response.SuccessResult(w, r, hgResult)
}

// HGPay 验证本人订单后报告渠道未配置，不执行支付或资产入账。
func (hgHandler *HGHandler) HGPay(w http.ResponseWriter, r *http.Request) {
	hgUser, hgOK := hgHandler.HGEnsureUser(w, r)
	if !hgOK {
		return
	}
	var hgRequest dto.HGPayRequest
	if !hgDecode(w, r, &hgRequest) {
		return
	}
	if hgErr := hgHandler.hgService.HGPay(r.Context(), hgUser, hgRequest.HGOrderID); hgErr != nil {
		hgWalletError(w, r, hgErr)
		return
	}
	response.SuccessResult[any](w, r, nil)
}

func hgDecode(w http.ResponseWriter, r *http.Request, hgTarget any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	hgDecoder := json.NewDecoder(r.Body)
	hgDecoder.DisallowUnknownFields()
	if hgErr := hgDecoder.Decode(hgTarget); hgErr != nil {
		hgWalletError(w, r, model.HGErrInvalid)
		return false
	}
	var hgExtra any
	if hgDecoder.Decode(&hgExtra) != io.EOF {
		hgWalletError(w, r, model.HGErrInvalid)
		return false
	}
	return true
}
func hgWalletError(w http.ResponseWriter, r *http.Request, hgErr error) {
	hgStatus, hgMessage := http.StatusInternalServerError, "钱包服务暂不可用"
	switch {
	case errors.Is(hgErr, model.HGErrInvalid):
		hgStatus, hgMessage = http.StatusBadRequest, model.HGErrInvalid.Error()
	case errors.Is(hgErr, model.HGErrNotFound):
		hgStatus, hgMessage = http.StatusNotFound, model.HGErrNotFound.Error()
	case errors.Is(hgErr, model.HGErrConflict):
		hgStatus, hgMessage = http.StatusConflict, model.HGErrConflict.Error()
	case errors.Is(hgErr, model.HGErrSKUUnavailable):
		hgStatus, hgMessage = http.StatusNotFound, model.HGErrSKUUnavailable.Error()
	case errors.Is(hgErr, model.HGErrUnsupported):
		hgStatus, hgMessage = http.StatusUnprocessableEntity, model.HGErrUnsupported.Error()
	case errors.Is(hgErr, model.HGErrPaymentUnavailable):
		hgStatus, hgMessage = http.StatusServiceUnavailable, model.HGErrPaymentUnavailable.Error()
	}
	hgCode := response.InvalidParam.Code
	if hgStatus >= 500 {
		hgCode = response.InternalError.Code
	}
	if hgStatus == http.StatusConflict {
		hgCode = response.ConflictCode
	}
	w.WriteHeader(hgStatus)
	response.FailResult[string](w, r, response.HGErrorResult{Code: hgCode, Message: hgMessage})
}
