package HGResponsePakcage

import (
	"MLC_GO/internal/pkg/logHG"
	hgtrace "MLC_GO/internal/pkg/trace"
	utils "MLC_GO/internal/pkg/utils"
	"MLC_GO/internal/response/hgerrors"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// HGBusinessMediaType 由已能处理非 2xx JSON 的客户端显式请求。
const HGBusinessMediaType = "application/vnd.mlc.v2+json"

// HGBusinessEnvelope 是协商后的 HTTP JSON 协议，分页对象整体置于 Data。
type HGBusinessEnvelope struct {
	Code    HGErrorCode          `json:"code"`              // 200 或七位业务码，与 HTTP 状态分离。
	Error   string               `json:"error,omitempty"`   // 失败时的稳定语义键。
	Message string               `json:"message"`           // 安全提示，不用于客户端分支判断。
	Data    any                  `json:"data"`              // 成功载荷，失败恒为 null。
	TraceID string               `json:"trace_id"`          // 有效 Span 优先，否则是本次请求 ID。
	Details *HGPermissionDetails `json:"details,omitempty"` // 只接受公开的权限申请条件。
}

// HGPermissionDetails 只描述路由所需权限，不描述调用方实际绑定。
type HGPermissionDetails struct {
	Requirement string `json:"requirement"` // 服务端从公开权限声明生成的说明。
}

// HGUsesBusinessEnvelope 只对专用 Accept 值启用 v2，默认和 */* 保留旧协议。
func HGUsesBusinessEnvelope(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Accept")), HGBusinessMediaType)
}

func hgTraceID(w http.ResponseWriter, r *http.Request) string {
	if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
		return sc.TraceID().String()
	}
	if tc := hgtrace.Get(r.Context()); tc != nil && tc.TraceD != "" {
		return tc.TraceD
	}
	if id := utils.GetTID(r.Context()); id != "" {
		return id
	}
	if id := w.Header().Get("X-Request-ID"); id != "" {
		return id
	}
	id := utils.GenerateTID()
	w.Header().Set("X-Request-ID", id)
	return id
}

var hgCounters = func() map[hgerrors.Code]*atomic.Uint64 {
	m := make(map[hgerrors.Code]*atomic.Uint64)
	for _, d := range hgerrors.HGDefinitions() {
		m[d.Code] = new(atomic.Uint64)
	}
	return m
}()

// HGWritePrometheusMetrics 仅使用字典中的固定维度，抓取不访问外部服务。
func HGWritePrometheusMetrics(w io.Writer) {
	fmt.Fprintln(w, "# TYPE hg_business_errors_total counter")
	for _, d := range hgerrors.HGDefinitions() {
		fmt.Fprintf(w, "hg_business_errors_total{module=%q,error_code=\"%d\"} %d\n", d.Code.HGModule(), d.Code, hgCounters[d.Code].Load())
	}
}

var hgStatusCodes = map[int]hgerrors.Code{
	400: hgerrors.HGCommonInvalidParam, 401: hgerrors.HGAuthTokenInvalid, 403: hgerrors.HGCommonForbidden,
	404: hgerrors.HGCommonNotFound, 405: hgerrors.HGCommonMethodNotAllowed, 408: hgerrors.HGCommonTimeout,
	409: hgerrors.HGCommonConflict, 410: hgerrors.HGCommonGone, 412: hgerrors.HGCommonPreconditionFailed,
	413: hgerrors.HGCommonBodyTooLarge, 415: hgerrors.HGMediaTypeUnsupported, 422: hgerrors.HGCommonUnprocessable,
	429: hgerrors.HGCommonRateLimited, 502: hgerrors.HGCommonBadGateway, 503: hgerrors.HGCommonUnavailable, 504: hgerrors.HGCommonGatewayTimeout,
}

func hgLegacyDefinition(status int, legacy HGErrorCode) hgerrors.Definition {
	c, known := hgLegacyCodes[legacy]
	d := c.Definition()
	if status >= 400 && d.HTTPStatus != status && (known || legacy >= InternalErrorCode && legacy <= SerializationCode) {
		return hgStatusCodes[status].Definition()
	}
	return d
}

// HGFailStatus 将旧状态与旧码在提交响应头之前翻译为业务定义。status=0 表示旧端隐式 200。
func HGFailStatus(w http.ResponseWriter, r *http.Request, status int, legacy HGErrorResult) {
	hgWriteFailure(w, r, status, legacy, hgLegacyDefinition(status, legacy.Code), nil, nil)
}

// HGSafeLegacyResult 保留历史数字码，但不把底层错误文本复制到旧客户端。
func HGSafeLegacyResult(code HGErrorCode) HGErrorResult {
	return HGErrorResult{Code: code, Message: hgLegacyDefinition(0, code).Message}
}

// HGFailError 使用包装后的领域原因，保留旧客户端的状态和安全文案。
func HGFailError(w http.ResponseWriter, r *http.Request, status int, legacy HGErrorResult, err error) {
	hgWriteFailure(w, r, status, legacy, hgerrors.Resolve(err), nil, nil)
}

func hgWriteFailure(w http.ResponseWriter, r *http.Request, status int, legacy HGErrorResult, d hgerrors.Definition, details *HGPermissionDetails, legacyData any) {
	id := hgTraceID(w, r)
	hgCounters[d.Code].Add(1)
	logFn := logHG.DebugFInfo
	if d.HTTPStatus >= 500 {
		logFn = logHG.ErrFInfo
	}
	logFn(`{"event":"业务请求失败","error_code":%d,"error_key":%q,"trace_id":%q,"http_status":%d}`, d.Code, d.Key, id, d.HTTPStatus)
	if HGUsesBusinessEnvelope(r) {
		hgWriteEnvelope(w, d.HTTPStatus, HGBusinessMediaType, HGBusinessEnvelope{Code: HGErrorCode(d.Code), Error: d.Key, Message: d.Message, TraceID: id, Details: details})
		return
	}
	hgWriteEnvelope(w, status, "application/json", HGResultModel[any]{Code: legacy.Code, Message: legacy.Message, Result: legacyData, TID: utils.GetTID(r.Context()), Timestamp: time.Now().UnixMilli()})
}

// HGFailPermission 仅接受由 HGPermissionError 生成的公开说明。
func HGFailPermission(w http.ResponseWriter, r *http.Request, legacy HGErrorResult) {
	hgWriteFailure(w, r, 403, legacy, hgerrors.HGIAMPermissionDenied.Definition(), &HGPermissionDetails{Requirement: legacy.Message}, nil)
}

// HGWriteCompatibleResult 接入历史上允许失败携带 result 的独立业务服务。新版失败 data 恒为 null。
func HGWriteCompatibleResult(w http.ResponseWriter, r *http.Request, status int, legacy HGResultModel[any], err error) {
	if err != nil {
		hgWriteFailure(w, r, status, HGErrorResult{legacy.Code, legacy.Message}, hgerrors.Resolve(err), nil, legacy.Result)
		return
	}
	if HGUsesBusinessEnvelope(r) {
		HGSuccessStatus(w, r, status, legacy.Result)
		return
	}
	legacy.Timestamp = time.Now().UnixMilli()
	legacy.TID = utils.GetTID(r.Context())
	hgWriteEnvelope(w, status, "application/json", legacy)
}

// HGSuccessStatus 支持显式 202；调用方把任务状态放在 data.status。
func HGSuccessStatus[T any](w http.ResponseWriter, r *http.Request, status int, data T) {
	if HGUsesBusinessEnvelope(r) {
		hgWriteEnvelope(w, status, HGBusinessMediaType, HGBusinessEnvelope{Code: OKCode, Message: HGSuccess.Message, Data: data, TraceID: hgTraceID(w, r)})
		return
	}
	hgWriteEnvelope(w, status, "application/json", HGResultModel[T]{Code: OKCode, Message: HGSuccess.Message, Result: data, TID: utils.GetTID(r.Context()), Timestamp: time.Now().UnixMilli()})
}

func hgWriteEnvelope(w http.ResponseWriter, status int, media string, body any) {
	// 主 HTTP 链的 writer 提供提交状态；经包装时沿 Unwrap 查找，防止重复错误体。
	for current := w; current != nil; {
		if committed, ok := current.(interface{ HGResponseCommitted() bool }); ok && committed.HGResponseCommitted() {
			logHG.ErrFInfo("业务响应已经开始，拒绝追加第二个响应体")
			return
		}
		wrapped, ok := current.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		current = wrapped.Unwrap()
	}
	// 编码成功后才提交状态，避免写入半个 JSON 后再发送错误。
	var b []byte
	var err error
	if media == HGBusinessMediaType {
		b, err = json.Marshal(body)
	} else {
		// 旧调用方及协议测试依赖既有缩进输出，不在此次灰度中附带改变。
		b, err = json.MarshalIndent(body, "", " ")
	}
	if err != nil {
		d := hgerrors.HGCommonInternalError.Definition()
		status = d.HTTPStatus
		if v, ok := body.(HGBusinessEnvelope); ok {
			b, _ = json.Marshal(HGBusinessEnvelope{Code: HGErrorCode(d.Code), Error: d.Key, Message: d.Message, TraceID: v.TraceID})
		} else {
			b, _ = json.Marshal(HGResultModel[any]{Code: InternalErrorCode, Message: d.Message, Timestamp: time.Now().UnixMilli()})
		}
	}
	w.Header().Add("Vary", "Accept")
	w.Header().Set("Content-Type", media+"; charset=utf-8")
	if status != 0 {
		w.WriteHeader(status)
	}
	if _, err = w.Write(b); err != nil {
		logHG.ErrFInfo("业务响应写入失败，客户端可能已断开：error_type=%T", err)
	}
}
