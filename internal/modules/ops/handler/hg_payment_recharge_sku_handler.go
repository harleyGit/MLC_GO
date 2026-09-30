package OpsHandlerPackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// HGCreateRechargeSKU accepts one bounded catalog definition from an authenticated operator.
func (h *Handler) HGCreateRechargeSKU(w http.ResponseWriter, r *http.Request) {
	operator, ok := hgRechargeSKUOperator(w, r)
	if !ok {
		return
	}
	var req OpsDtoPackage.HGPaymentRechargeSKUCreateRequest
	if !hgDecodeRechargeSKU(w, r, &req) {
		return
	}
	id, err := h.service.HGCreateRechargeSKU(r.Context(), operator, req)
	if err != nil {
		hgWriteRechargeSKUError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, struct {
		SKUID   string `json:"skuId"`
		Version uint32 `json:"version"`
	}{id, 1})
}

// HGUpdateRechargeSKU requires the last observed positive version.
func (h *Handler) HGUpdateRechargeSKU(w http.ResponseWriter, r *http.Request) {
	operator, ok := hgRechargeSKUOperator(w, r)
	if !ok {
		return
	}
	var req OpsDtoPackage.HGPaymentRechargeSKUUpdateRequest
	if !hgDecodeRechargeSKU(w, r, &req) {
		return
	}
	if err := h.service.HGUpdateRechargeSKU(r.Context(), operator, req); err != nil {
		hgWriteRechargeSKUError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, struct {
		SKUID   string `json:"skuId"`
		Version uint32 `json:"version"`
	}{req.SKUID, *req.Version + 1})
}

// HGDeleteRechargeSKU soft deletes a versioned catalog row, not any order or balance.
func (h *Handler) HGDeleteRechargeSKU(w http.ResponseWriter, r *http.Request) {
	operator, ok := hgRechargeSKUOperator(w, r)
	if !ok {
		return
	}
	var req OpsDtoPackage.HGPaymentRechargeSKUDeleteRequest
	if !hgDecodeRechargeSKU(w, r, &req) {
		return
	}
	if err := h.service.HGDeleteRechargeSKU(r.Context(), operator, req); err != nil {
		hgWriteRechargeSKUError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult[any](w, r, nil)
}

// HGListRechargeSKUs exposes only business IDs and a decimal pagination cursor.
func (h *Handler) HGListRechargeSKUs(w http.ResponseWriter, r *http.Request) {
	operator, ok := hgRechargeSKUOperator(w, r)
	if !ok {
		return
	}
	resp, err := h.service.HGListRechargeSKUs(r.Context(), operator, r.URL.Query().Get("cursor"), r.URL.Query().Get("pageSize"))
	if err != nil {
		hgWriteRechargeSKUError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, resp)
}

func hgRechargeSKUOperator(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
	}
	return id, ok
}

func hgDecodeRechargeSKU(w http.ResponseWriter, r *http.Request, target any) bool {
	// 1. 限制请求体最大读取字节数：16<<10 = 16*1024 = 16KB
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	// 2. 创建JSON解码器，从r.Body（请求body流）读取数据
	decoder := json.NewDecoder(r.Body)
	// 3. 禁止JSON里存在结构体未定义的字段
	decoder.DisallowUnknownFields()
	// 4. 把JSON body反序列化到 target 指向的变量（必须传指针）
	err := decoder.Decode(target)
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = OpsServicePackage.ErrHGRechargeSKUInvalid
		}
	}
	if err != nil {
		hgWriteRechargeSKUError(w, r, OpsServicePackage.ErrHGRechargeSKUInvalid)
		return false
	}
	return true
}

func hgWriteRechargeSKUError(w http.ResponseWriter, r *http.Request, err error) {
	// 左边两个变量，一次性分别赋值右边两个值
	status, message := http.StatusInternalServerError, "Recharge SKU service unavailable"
	switch {
	// erros.Is() 函数用于判断 err 是否与指定的错误类型匹配，支持链式错误检查。
	case errors.Is(err, OpsServicePackage.ErrHGOperationsForbidden):
		status, message = http.StatusForbidden, "Permission denied"
	case errors.Is(err, OpsServicePackage.ErrHGRechargeSKUInvalid):
		status, message = http.StatusBadRequest, "Invalid recharge SKU parameters"
	case errors.Is(err, OpsRepositoryPackage.ErrHGRechargeSKUConflict):
		status, message = http.StatusConflict, "SKU code already exists or SKU version has changed; refresh and retry"
	}
	code := HGResponsePakcage.InvalidParam.Code
	if status >= 500 {
		code = HGResponsePakcage.InternalError.Code
	}
	w.WriteHeader(status)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: message})
}
