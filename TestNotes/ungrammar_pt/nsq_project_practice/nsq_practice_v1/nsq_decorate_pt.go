/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-09-09 21:15:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-09-11 17:02:52
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/nsq_project_practice/nsq_practice_v1/nsq_decorate_pt.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/* 装饰器使用 */

package nsq_practice_v1

import (
	"MLC_GO/pkg/logHG"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type APIHandler func(http.ResponseWriter, *http.Request, httprouter.Params) (interface{}, error)

func (this *NSQPracticeV1) Decorate_pt_NSQ() {

	router := httprouter.New()

	// 使用 PlainText 装饰器
	router.Handle("GET", "/ping", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		decorate(pingHandler, plainText)(w, r, ps)
	})

	router.Handle("GET", "/hello/:name", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		decorate(helloHandler, plainText)(w, r, ps)
	})

	logHG.DebugInfo("Server running on http://localhost:8080")
	// 启动 HTTP服务，若是出错，则推出
	logHG.FatalFInfo("%v", http.ListenAndServe(":8080", router))
}

// PlainText 装饰器： 把返回结果写到HTTP响应
func plainText(f APIHandler) APIHandler {

	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) (interface{}, error) {
		code := 200
		data, err := f(w, req, ps) // 调用传入的f（业务函数），得到原始的返回值 data, err
		if err != nil {
			//简化： 假设错误就是500
			code = 500
			data = err.Error()
		}

		// 根据data的类型写回响应
		switch d := data.(type) {
		case string:
			w.WriteHeader(code) // 直接写到http.ResponseWriter
			io.WriteString(w, d)
		case []byte:
			w.WriteHeader(code)
			w.Write(d)
		default:
			panic(fmt.Sprintf("unknown response type %T", data))
		}

		return nil, nil
	}
}

// Decorate 把多个装饰器包裹起来
func decorate(f APIHandler, decorators ...func(APIHandler) APIHandler) APIHandler {
	// 反向应用：最后一个 decorator 最先执行
	for i := len(decorators) - 1; i >= 0; i-- {
		f = decorators[i](f)
	}
	return f
}

// ====================== 示例业务逻辑 ======================
func pingHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) (interface{}, error) {
	return "pong", nil
}

func helloHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) (interface{}, error) {
	name := ps.ByName("name")
	return fmt.Sprintf("Hello, %s!", &name), nil
}
