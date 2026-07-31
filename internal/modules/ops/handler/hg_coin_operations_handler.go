package OpsHandlerPackage

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// GetCoinAccount 查询指定用户的 MySQL 权威余额。
func (h *Handler) GetCoinAccount(w http.ResponseWriter, r *http.Request) {
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.GetCoinAccount(r.Context(), operatorID, r.URL.Query().Get("userId"))
	})
}

// GetCoinTransactions 使用不透明复合游标读取有界资产流水。
func (h *Handler) GetCoinTransactions(w http.ResponseWriter, r *http.Request) {
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		return operational.GetCoinTransactions(r.Context(), operatorID, r.URL.Query().Get("userId"), r.URL.Query().Get("cursor"), pageSize)
	})
}

// GrantCoin 执行有界人工赠币，requestId 在客户端重试期间必须保持稳定。
func (h *Handler) GrantCoin(w http.ResponseWriter, r *http.Request) {
	var req OpsDtoPackage.HGCoinGrantRequest
	if !hgDecodeOpsJSON(w, r, &req) {
		return
	}
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.GrantCoin(r.Context(), operatorID, req)
	})
}

// RefundCoin 退款必须引用原 debit transaction。
func (h *Handler) RefundCoin(w http.ResponseWriter, r *http.Request) {
	var req OpsDtoPackage.HGCoinRefundRequest
	if !hgDecodeOpsJSON(w, r, &req) {
		return
	}
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.RefundCoin(r.Context(), operatorID, req)
	})
}

// CorrectCoin 通过不可变 correction 流水执行有界修正。
func (h *Handler) CorrectCoin(w http.ResponseWriter, r *http.Request) {
	var req OpsDtoPackage.HGCoinCorrectionRequest
	if !hgDecodeOpsJSON(w, r, &req) {
		return
	}
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.CorrectCoin(r.Context(), operatorID, req)
	})
}

// GetAssetPipelineStatus 返回 Coin、Interaction reproject 和 Kafka 的低成本状态快照。
func (h *Handler) GetAssetPipelineStatus(w http.ResponseWriter, r *http.Request) {
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.GetAssetPipelineStatus(r.Context(), operatorID)
	})
}

func (h *Handler) hgWithOperator(w http.ResponseWriter, r *http.Request, call func(string, *OpsServicePackage.HGOperationalService) (any, error)) {
	operatorID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if h == nil || h.service == nil || h.service.Operational() == nil {
		hgWriteCoinOperationsError(w, r, errors.New("operations service unavailable"))
		return
	}
	result, err := call(operatorID, h.service.Operational())
	if err != nil {
		hgWriteCoinOperationsError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, result)
}

func hgDecodeOpsJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := decoder.Decode(target); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求参数错误"})
		return false
	}
	return true
}

func hgWriteCoinOperationsError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := HGResponsePakcage.InternalError.Code
	message := "运维操作失败"
	switch {
	case errors.Is(err, OpsServicePackage.ErrHGOperationsForbidden):
		status, code, message = http.StatusForbidden, HGResponsePakcage.Forbidden.Code, "无运维操作权限"
	case errors.Is(err, OpsServicePackage.ErrHGOperationsInvalid), errors.Is(err, CoinServicePackage.ErrHGInvalidIdentity), errors.Is(err, CoinServicePackage.ErrHGInvalidAmount), errors.Is(err, CoinServicePackage.ErrHGInvalidReference), errors.Is(err, CoinServicePackage.ErrHGInvalidReason):
		status, code, message = http.StatusBadRequest, HGResponsePakcage.InvalidParam.Code, "运维请求参数错误"
	case errors.Is(err, CoinRepositoryPackage.ErrHGIdempotencyConflict):
		status, code, message = http.StatusConflict, HGResponsePakcage.InvalidParam.Code, "requestId 已被不同资产命令使用"
	case errors.Is(err, CoinRepositoryPackage.ErrHGInsufficientBalance):
		status, code, message = http.StatusConflict, HGResponsePakcage.InvalidParam.Code, "用户硬币余额不足"
	case errors.Is(err, CoinRepositoryPackage.ErrHGRefundExceedsDebit):
		status, code, message = http.StatusConflict, HGResponsePakcage.InvalidParam.Code, "退款超过原扣款可退金额"
	}
	w.WriteHeader(status)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: message})
}
