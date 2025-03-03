/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 16:52:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-03 23:31:58
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import "MLC_GO/TestNotes/PracticeGRPCExample/cmd"

// 终端输入： cd ./TestNotes/PracticeGRPCExample
// go run main.go server
// go run main.go server --port=8000 --cert-pem=test-pem --cert-key=test-key --cert-name=test-name
// 在 日志文件中查找
// 接着：cd ./TestNotes/PracticeGRPCExample/client
//	go run client.go
//  curl -X POST -k https://localhost:50052/hello_world -d '{"referer": "restful_api"}'
// 出现： {"message":"restful_api(‼️🍎 作者这个地方有问题，注意了)"}%  表示正确
func main() {
	cmd.Execute()
}