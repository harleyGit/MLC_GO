/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-23 13:19:57
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-23 14:32:36
 * @FilePath: /MLC_GO/TestNotes/TestNetProgram/TestHttp/test_http.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"flag"
	"net/http"
)

func main() {
	// http服务端请求
	testHttpServerV0()
}

// http服务端请求
func testHttpServerV0() {
	host := flag.String("host", "127.0.0.1", "listen host")	// 域名
	port := flag.String("port", "3000", "listen port")	// 端口

	http.HandleFunc("/hello", Hello)

	err := http.ListenAndServe(*host+":"+*port, nil) //处理HTTP请求
	if err != nil {
		panic(err) 
	}
}
func Hello(w http.ResponseWriter, req *http.Request){
	w.Write([]byte("你好， 客户端！"))
}