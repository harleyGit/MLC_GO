package handler

import (
	"MLC_GO/internal/modules/recharge/model"
	"MLC_GO/internal/modules/recharge/service"
	hgjwt "MLC_GO/internal/modules/user/middleware"
	hguser "MLC_GO/internal/modules/user/service"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type hgRechargeHandlerRepo struct{ hgErr error }

func (hgRepo hgRechargeHandlerRepo) HGList(context.Context, string, *model.HGCursor, int) ([]model.HGRecord, error) {
	return nil, hgRepo.hgErr
}

func hgRechargeRequest(hgUser, hgQuery string) *http.Request {
	hgRequest := httptest.NewRequest(http.MethodGet, "/records"+hgQuery, nil)
	if hgUser != "" {
		hgRequest = hgRequest.WithContext(context.WithValue(hgRequest.Context(), hgjwt.UserIDKey, &hguser.HGClaims{UserID: hgUser}))
	}
	return hgRequest
}

func TestHGRechargeHandlerErrors(t *testing.T) {
	for _, hgCase := range []struct {
		hgName, hgUser, hgQuery string
		hgRepoErr               error
		hgWant                  int
	}{
		{"401", "", "", nil, http.StatusUnauthorized},
		{"400 cursor", "owner", "?cursor=bad", nil, http.StatusBadRequest},
		{"400 page size", "owner", "?pageSize=51", nil, http.StatusBadRequest},
		{"500 repository", "owner", "", errors.New("db failure"), http.StatusInternalServerError},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgHandler := HGNewHandler(service.HGNewService(hgRechargeHandlerRepo{hgErr: hgCase.hgRepoErr}))
			hgRecorder := httptest.NewRecorder()
			hgHandler.HGRecords(hgRecorder, hgRechargeRequest(hgCase.hgUser, hgCase.hgQuery))
			if hgRecorder.Code != hgCase.hgWant {
				t.Fatalf("status=%d body=%s", hgRecorder.Code, hgRecorder.Body.String())
			}
			if strings.Contains(hgRecorder.Body.String(), "db failure") {
				t.Fatal("不得返回数据库原始错误")
			}
		})
	}
}
