/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-23 13:23:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-23 13:23:04
 * @FilePath: /MLC_GO/TestNotes/TestNetProgram/TestHttp/TestHttpClient/test_http_client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	// http客户端发送请求
	testHttpClientV0()
}

// http客户端发送请求
func testHttpClientV0() {
	response, _ := http.Get("http://localhost:3000/hello")// 发送HTTP请求， 请求一个网页
	defer response.Body.Close() // 关闭请求
	body, _ := ioutil.ReadAll(response.Body) // 接收数据
	fmt.Println(string(body))	
}