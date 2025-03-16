/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 17:15:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-03 17:25:21
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/pkg/util/grpc.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package util

import (
	"net/http"
	"strings"

	"google.golang.org/grpc"
)

func GrpcHandleFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	if otherHandler == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			grpcServer.ServeHTTP(w, r)
		})
	}

	// GrpcHandlerFunc函数是用于判断请求是来源于Rpc客户端还是Restful Api的请求，根据不同的请求注册不同的ServeHTTP服务
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.ProtoMajor == 2也代表着请求必须基于HTTP/2
		if  r.ProtoMajor ==2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}