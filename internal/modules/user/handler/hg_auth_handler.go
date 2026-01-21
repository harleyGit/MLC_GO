/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:55:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 21:09:17
 * @FilePath: /MLC_GO/internal/modules/user/handler/hg_auth_handler.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserHandlerPackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"encoding/json"
	"net/http"
)

type HGAuthHandler struct {
	svc *UserServicePackage.HGAuthService
}

func NewAuthHandler(svc *UserServicePackage.HGAuthService) *HGAuthHandler {
	return &HGAuthHandler{svc: svc}
}

func (h *HGAuthHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	var d UserDtoPackage.SendCodeDTO
	json.NewDecoder(r.Body).Decode(&d)
	h.svc.SendCode(r.Context(), &d)
	w.WriteHeader(http.StatusOK)
}

func (h *HGAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var d UserDtoPackage.LoginDTO
	json.NewDecoder(r.Body).Decode(&d)

	res, err := h.svc.Login(r.Context(), &d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(res)
}
