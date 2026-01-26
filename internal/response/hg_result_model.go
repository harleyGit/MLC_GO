/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:01:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 21:27:14
 * @FilePath: /MLC_GO/internal/interfaces/response/hg_result_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGResponsePakcage

import (
	HGBaseModelPackage "MLC_GO/internal/models"
	"encoding/json"
	"net/http"
	"time"
)

// 所有 Result 数据如果需要自描述，可以实现这个接口
// 在中大型项目（错误码分层、领域模型）非常有用
type HGResultModel interface {
	ResponseCode() int
	ResponseMessage() string
}



/* 统一·JOSN 输出方法 */

func WriteJSON(
	w http.ResponseWriter,
	tid string,
	result interface{},
) {

	resp := HGBaseModelPackage.HGBaseResponseModel {
		Code:0,
		Message: "success💯",
		Result: result,
		TID: tid,
		Timestamp: time.Now().UnixMilli(),
	}

	// 若是 result 实现了接口，自动使用它的 code / message
	if r, ok := result.(HGResultModel); ok {
		resp.Code = r.ResponseCode()
		resp.Message = r.ResponseMessage()
	}

	jsonBytes, err  := json.MarshalIndent(resp, "", " ") // "" = 前缀，"  " = 每级缩进两个空格
	if err != nil {
		http.Error(w, "JSON encode failed ❌", http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}