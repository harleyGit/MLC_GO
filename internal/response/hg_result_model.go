/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:01:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-07 21:41:12
 * @FilePath: /MLC_GO/internal/interfaces/response/hg_result_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGResponsePakcage

import (
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"encoding/json"
	"net/http"
	"time"
)

// 所有 Result 数据如果需要自描述，可以实现这个接口
// 在中大型项目（错误码分层、领域模型）非常有用
type HGResultProtocol interface {
	ResponseCode() HGErrorCode
	ResponseMessage() string
}

/* 单个数据 */
type HGResultModel[T any] struct {
	Code      HGErrorCode `json:"code"`
	Message   string      `json:"message"`
	Result    T           `json:"result,omitempty"` //使用泛型结构数据，用泛型结构体
	TID       string      `json:"tid"`
	Timestamp int64       `json:"timestamp"`
}

/** HGPageResultModel 是一个通用的分页响应结构体，T 为结果数组中每个元素的类型
 *
 * @description: 分页结果结构体
 * @param {T} any - 结果数组中每个元素的类型
 */
type HGPageResultModel[T any] struct {
	Seid           string `json:"seid"`            // 搜索会话 ID（用于追踪本次搜索）
	Page           int    `json:"page"`            // 当前页码（第 N 页）
	Pagesize       int    `json:"pagesize"`        // 每页返回多少条结果
	NumResults     int    `json:"numResults"`      // 总共匹配到的结果数（可能为估算值）
	NumPages       int    `json:"numPages"`        // 总共页数
	SuggestKeyword string `json:"suggest_keyword"` // 搜索建议词
	RqtType        string `json:"rqt_type"`        // 请求类型，如 "search"
	IsHitWebInf    bool   `json:"is_hit_web_inf"`  // 是否命中网页信息

	Result []T `json:"result"` // 泛型结果数组，存放实际数据
}

// PageOption 是用于自定义 PageResponse 的选项函数
type HGPageOption func(*HGPageResponseConfig)

// PageResponseConfig 保存构造时的配置
type HGPageResponseConfig struct {
	Seid           string
	Page           int
	Pagesize       int
	NumResults     int
	NumPages       int
	SuggestKeyword string
	RqtType        string
	IsHitWebInf    bool
}

// 默认配置
func defaultPageConfig() *HGPageResponseConfig {
	return &HGPageResponseConfig{
		Seid:           UtilsPackage.GenerateSeid(), // 自动生成唯一 seid
		Page:           1,                           // 默认第 1 页
		Pagesize:       20,                          // 默认每页 20 条
		NumResults:     0,
		NumPages:       0,
		SuggestKeyword: "",
		RqtType:        "search",
		IsHitWebInf:    false,
	}
}

// WithSeid 自定义 Seid
func WithSeid(seid string) HGPageOption {
	return func(c *HGPageResponseConfig) {
		if seid != "" {
			c.Seid = seid
		}
	}
}

// WithPage 设置页码
func WithPage(page int) HGPageOption {
	return func(c *HGPageResponseConfig) {
		if page > 0 {
			c.Page = page
		}
	}
}

// WithPagesize 设置每页大小
func WithPagesize(size int) HGPageOption {
	return func(c *HGPageResponseConfig) {
		if size > 0 {
			c.Pagesize = size
		}
	}
}

// WithTotal 设置总结果数，并自动计算总页数（如果未显式设置）
func WithTotal(total int) HGPageOption {
	return func(c *HGPageResponseConfig) {
		if total >= 0 {
			c.NumResults = total
			// 自动计算 NumPages（向上取整）
			if c.Pagesize > 0 {
				pages := (total + c.Pagesize - 1) / c.Pagesize
				if c.NumPages == 0 { // 仅当未手动设置时才自动计算
					c.NumPages = pages
				}
			}
		}
	}
}

// WithNumPages 显式设置总页数
func WithNumPages(pages int) HGPageOption {
	return func(c *HGPageResponseConfig) {
		if pages >= 0 {
			c.NumPages = pages
		}
	}
}

// WithSuggestKeyword 设置建议词
func WithSuggestKeyword(keyword string) HGPageOption {
	return func(c *HGPageResponseConfig) {
		c.SuggestKeyword = keyword
	}
}

// WithRqtType 设置请求类型
func WithRqtType(rqtType string) HGPageOption {
	return func(c *HGPageResponseConfig) {
		if rqtType != "" {
			c.RqtType = rqtType
		}
	}
}

// WithIsHitWebInf 设置是否命中网页信息
func WithIsHitWebInf(hit bool) HGPageOption {
	return func(c *HGPageResponseConfig) {
		c.IsHitWebInf = hit
	}
}


// NewPageResponse 创建一个新的 PageResponse[T] 实例
// items: 实际数据列表
// opts: 可选配置（模拟“默认参数”）
func NewPageResponse[T any](items []T, opts ...HGPageOption) HGPageResultModel[T] {
	config := defaultPageConfig()

	// 应用用户传入的选项
	for _, opt := range opts {
		opt(config)
	}

	// 如果 NumPages 仍为 0 且有 Pagesize 和 NumResults，则重新计算
	if config.NumPages == 0 && config.Pagesize > 0 && config.NumResults > 0 {
		config.NumPages = (config.NumResults + config.Pagesize - 1) / config.Pagesize
	}

	return HGPageResultModel[T]{
		Seid:           config.Seid,
		Page:           config.Page,
		Pagesize:       config.Pagesize,
		NumResults:     config.NumResults,
		NumPages:       config.NumPages,
		SuggestKeyword: config.SuggestKeyword,
		RqtType:        config.RqtType,
		IsHitWebInf:    config.IsHitWebInf,
		Result:         items,
	}
}

func SuccessResult[T any](w http.ResponseWriter, r *http.Request, data T) {

	result := HGResultModel[T]{
		Code:      OKCode,
		Message:   "success💯",
		Result:    data,
		TID:       UtilsPackage.GetTID(r.Context()),
		Timestamp: time.Now().UnixMilli(),
	}

	writeResult(result, w)
}

func SuccessPageResult[T any](w http.ResponseWriter, r *http.Request, data T) {

	result := HGResultModel[T]{
		Code:      OKCode,
		Message:   "success💯",
		Result:    data,
		TID:       UtilsPackage.GetTID(r.Context()),
		Timestamp: time.Now().UnixMilli(),
	}

	writeResult(result, w)
}

func FailResult[T any](w http.ResponseWriter, r *http.Request, code HGErrorCode, msg string) {

	var zero T
	resp := HGResultModel[T]{Code: code,
		Message:   msg,
		Result:    zero,
		TID:       UtilsPackage.GetTID(r.Context()),
		Timestamp: time.Now().UnixMilli(),
	}

	writeResult(resp, w)
}

/* 统一·JOSN 输出方法 */
// Deprecated: 使用 SuccessResult 方法，因为该方法无法定制化
func WriteJSON(
	w http.ResponseWriter,
	r *http.Request,
	result HGResultProtocol,
) {

	resp := HGAPIResponseModel[HGResultProtocol]{
		Code:      0,
		Message:   "success💯",
		Result:    result,
		TID:       UtilsPackage.GetTID(r.Context()), //TODO: tid直接写这里，就不用写tid中间件了
		Timestamp: time.Now().UnixMilli(),
	}

	// 若是 result 实现了接口，自动使用它的 code / message
	if r, ok := interface{}(result).(HGResultProtocol); ok {
		resp.Code = r.ResponseCode()
		resp.Message = r.ResponseMessage()
	}

	writeResult(resp, w)
	// json.MarshlIndent会在每个字段之间自动换行，并缩进2格
	// jsonBytes, err := json.MarshalIndent(resp, "", " ") // "" = 前缀，"  " = 每级缩进两个空格
	// if err != nil {
	// 	http.Error(w, "JSON encode failed ❌", http.StatusInternalServerError)
	// 	return
	// }

	// w.Write(jsonBytes)
}

func writeResult(resp interface{}, w http.ResponseWriter) {

	// json.NewEncoder(w).Encode(userMap) //不缩进，自动输出 JSON
	// json.MarshlIndent会在每个字段之间自动换行，并缩进2格
	jsonBytes, err := json.MarshalIndent(resp, "", " ") // "" = 前缀，"  " = 每级缩进两个空格
	if err != nil {
		http.Error(w, "JSON encode failed ❌", http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}


