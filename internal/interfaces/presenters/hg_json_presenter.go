/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-14 16:09:56
* @LastEditors: GangHuang harleysor@qq.com
* @LastEditTime: 2026-01-14 16:09:58
* @FilePath: /MLC_GO/internal/interfaces/presenters/hg_json_presenter.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 功能：响应格式化
 */
package PresentersPackage

import (
	"encoding/json"
	"net/http"
)

// 辅助函数：将响应写为 JSON 格式
// v any：Go 1.18+ 引入的泛型语法，any 是 interface{} 的别名，表示可以传入任意类型的值（比如 struct、map、slice 等
func WriteJSON(w http.ResponseWriter, v any)  {
	w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w) 创建一个将数据编码为 JSON 并直接写入 w（即 HTTP 响应流）的编码器。
	// .Encode(v) 将变量 v 序列化为 JSON，并写入响应
	json.NewEncoder(w).Encode(v)
}