package OpsHandlerPackage

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
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
		return operational.GrantCoin(r.Context(), hgAssetOperator(r, operatorID), req)
	})
}

// RefundCoin 退款必须引用原 debit transaction。
func (h *Handler) RefundCoin(w http.ResponseWriter, r *http.Request) {
	var req OpsDtoPackage.HGCoinRefundRequest
	if !hgDecodeOpsJSON(w, r, &req) {
		return
	}
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.RefundCoin(r.Context(), hgAssetOperator(r, operatorID), req)
	})
}

// CorrectCoin 通过不可变 correction 流水执行有界修正。
func (h *Handler) CorrectCoin(w http.ResponseWriter, r *http.Request) {
	var req OpsDtoPackage.HGCoinCorrectionRequest
	if !hgDecodeOpsJSON(w, r, &req) {
		return
	}
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.CorrectCoin(r.Context(), hgAssetOperator(r, operatorID), req)
	})
}

// ApproveCoinCorrection approves and applies a pending correction using the current JWT operator.
func (h *Handler) ApproveCoinCorrection(w http.ResponseWriter, r *http.Request) {
	var req OpsDtoPackage.HGCoinCorrectionApproveRequest
	if !hgDecodeOpsJSON(w, r, &req) {
		return
	}
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.ApproveCoinCorrection(r.Context(), hgAssetOperator(r, operatorID), req.CorrectionID)
	})
}

// ListCoinCorrections returns bounded correction workflow state using a primary-key cursor.
func (h *Handler) ListCoinCorrections(w http.ResponseWriter, r *http.Request) {
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		return operational.ListCoinCorrections(r.Context(), operatorID, r.URL.Query().Get("cursor"), pageSize)
	})
}

// GetCurrentAssetPermissions exposes current database-backed asset permissions for later UI gating.
func (h *Handler) GetCurrentAssetPermissions(w http.ResponseWriter, r *http.Request) {
	h.hgWithOperator(w, r, func(operatorID string, operational *OpsServicePackage.HGOperationalService) (any, error) {
		return operational.GetCurrentAssetPermissions(r.Context(), operatorID)
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
	case errors.Is(err, OpsServicePackage.ErrHGOperationsRateLimited):
		status, code, message = http.StatusTooManyRequests, HGResponsePakcage.InvalidParam.Code, "资产写操作请求过于频繁"
	case errors.Is(err, OpsServicePackage.ErrHGOperationsRateLimitUnavailable):
		status, code, message = http.StatusServiceUnavailable, HGResponsePakcage.InternalError.Code, "资产写操作限流服务不可用"
	case errors.Is(err, OpsServicePackage.ErrHGOperationsInvalidApprover):
		status, code, message = http.StatusConflict, HGResponsePakcage.InvalidParam.Code, "审批人不能与申请人相同"
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

func hgAssetOperator(r *http.Request, operatorID string) OpsServicePackage.HGAssetOperator {
	return OpsServicePackage.HGAssetOperator{ID: operatorID, SourceIP: hgRemoteIP(r), TID: UtilsPackage.GetTID(r.Context())}
}

func hgRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
