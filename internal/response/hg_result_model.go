/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:01:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:44:49
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
type HGResultModel interface {
	ResponseCode() HGErrorCode
	ResponseMessage() string
}

/* 统一·JOSN 输出方法 */

func WriteJSON(
	w http.ResponseWriter,
	r *http.Request,
	result HGResultModel,
) {

	resp := HGAPIResponseModel{
		Code:      0,
		Message:   "success💯",
		Result:    result,
		TID:       UtilsPackage.GetTID(r.Context()), //TODO: tid直接写这里，就不用写tid中间件了
		Timestamp: time.Now().UnixMilli(),
	}

	// 若是 result 实现了接口，自动使用它的 code / message
	if r, ok := interface{}(result).(HGResultModel); ok {
		resp.Code = r.ResponseCode()
		resp.Message = r.ResponseMessage()
	}

	// json.MarshlIndent会在每个字段之间自动换行，并缩进2格
	jsonBytes, err := json.MarshalIndent(resp, "", " ") // "" = 前缀，"  " = 每级缩进两个空格
	if err != nil {
		http.Error(w, "JSON encode failed ❌", http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}
