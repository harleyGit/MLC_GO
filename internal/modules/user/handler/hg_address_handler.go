package UserHandlerPackage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	userrepository "MLC_GO/internal/modules/user/repository"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
)

func hgAddressUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "未登录")
	}
	return userID, ok
}

func hgAddressID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		hgAddressFailure(w, r, UserServicePackage.ErrHGAddressInvalid)
		return 0, false
	}
	return id, true
}

func hgAddressFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, HGResponsePackage.InternalError.Code, "收货地址操作失败"
	switch {
	case errors.Is(err, sql.ErrNoRows):
		status, code, message = http.StatusNotFound, HGResponsePackage.UserNotFound.Code, "收货地址不存在"
	case errors.Is(err, UserServicePackage.ErrHGAddressInvalid), errors.Is(err, userrepository.ErrHGAddressLimit):
		status, code, message = http.StatusBadRequest, HGResponsePackage.InvalidParam.Code, err.Error()
	}
	w.WriteHeader(status)
	HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: code, Message: message})
}

func hgDecodeAddress(w http.ResponseWriter, r *http.Request) (UserDtoPackage.HGAddressInput, bool) {
	var input UserDtoPackage.HGAddressInput
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		hgAddressFailure(w, r, UserServicePackage.ErrHGAddressInvalid)
		return input, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		hgAddressFailure(w, r, UserServicePackage.ErrHGAddressInvalid)
		return input, false
	}
	return input, true
}

// HGCreateAddress 新增当前登录用户收货地址。
func (h *HGUserHandler) HGCreateAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgAddressUser(w, r)
	if !ok {
		return
	}
	input, ok := hgDecodeAddress(w, r)
	if !ok {
		return
	}
	a, err := h.hgAddressService.HGCreateAddress(r.Context(), userID, input)
	if err != nil {
		hgAddressFailure(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, a)
}

// HGListAddresses 返回当前登录用户地址列表。
func (h *HGUserHandler) HGListAddresses(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgAddressUser(w, r)
	if !ok {
		return
	}
	items, err := h.hgAddressService.HGListAddresses(r.Context(), userID)
	if err != nil {
		hgAddressFailure(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, items)
}

// HGGetAddress 按查询参数 id 返回本人地址。
func (h *HGUserHandler) HGGetAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgAddressUser(w, r)
	if !ok {
		return
	}
	id, ok := hgAddressID(w, r)
	if !ok {
		return
	}
	a, err := h.hgAddressService.HGGetAddress(r.Context(), userID, id)
	if err != nil {
		hgAddressFailure(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, a)
}

// HGUpdateAddress 按 id 全量替换本人地址字段，不变更默认状态。
func (h *HGUserHandler) HGUpdateAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgAddressUser(w, r)
	if !ok {
		return
	}
	id, ok := hgAddressID(w, r)
	if !ok {
		return
	}
	input, ok := hgDecodeAddress(w, r)
	if !ok {
		return
	}
	if err := h.hgAddressService.HGUpdateAddress(r.Context(), userID, id, input); err != nil {
		hgAddressFailure(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, true)
}

// HGDeleteAddress 删除本人地址，删除默认时自动补位。
func (h *HGUserHandler) HGDeleteAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgAddressUser(w, r)
	if !ok {
		return
	}
	id, ok := hgAddressID(w, r)
	if !ok {
		return
	}
	if err := h.hgAddressService.HGDeleteAddress(r.Context(), userID, id); err != nil {
		hgAddressFailure(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, true)
}

// HGSetDefaultAddress 设置本人默认收货地址。
func (h *HGUserHandler) HGSetDefaultAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgAddressUser(w, r)
	if !ok {
		return
	}
	id, ok := hgAddressID(w, r)
	if !ok {
		return
	}
	if err := h.hgAddressService.HGSetDefaultAddress(r.Context(), userID, id); err != nil {
		hgAddressFailure(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, true)
}
