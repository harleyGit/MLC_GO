/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:01:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-31 22:47:47
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

type HGResultModel[T any] struct {
	Code      HGErrorCode `json:"code"`
	Message   string      `json:"message"`
	Result    T           `json:"result,omitempty"` //使用泛型结构数据，用泛型结构体
	TID       string      `json:"tid"`
	Timestamp int64       `json:"timestamp"`
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
