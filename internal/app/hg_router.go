/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-28 21:00:13
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 21:12:38
 * @FilePath: /MLC_GO/internal/app/hg_router.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGAppPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	"net/http"
)

type HGRouter struct {
	Method string
	Path   string
	Auth   bool
	Span   string
	Handle http.HandlerFunc
}

/* 很模块路由注册 */
func Register(mux *http.ServeMux, routes []HGRouter) {

	for _, r := range routes {
		var h http.Handler = r.Handle
		h = HGMiddlewarePackage.TraceMiddleware(r.Span)(h)
		// if r.Auth {
		// 	h = HGMiddlewarePackage.AuthMiddlewareGoup(h)
		// }
		// h = HGMiddlewarePackage.LoggerMiddleware(h)
		h = HGMiddlewarePackage.TIDMiddleware(h)

		mux.Handle(r.Path, h)
	}
}

// TODO：事例使用，如下：一定要用，很赞

/*
module/user/handler.go


package user

import (
	"net/http"

	"hg-server/internal/app"
	"hg-server/internal/response"
)

func Routes() []app.Route {
	return []app.Route{
		{
			Method: "GET",
			Path:   "/user/profile",
			Auth:   true,
			Span:   "user.profile",
			Handle: Profile,
		},
	}
}

func Profile(w http.ResponseWriter, r *http.Request) {
	response.Success(w, r, map[string]string{
		"user": "harley",
	})
}



// =====================
module/auth/handler.go

package auth

import (
	"net/http"

	"hg-server/internal/app"
	"hg-server/internal/response"
)

func Routes() []app.Route {
	return []app.Route{
		{
			Method: "POST",
			Path:   "/auth/login",
			Auth:   false,
			Span:   "auth.login",
			Handle: Login,
		},
	}
}

func Login(w http.ResponseWriter, r *http.Request) {
	response.Success(w, r, map[string]string{
		"token": "mock-token",
	})
}


// =========================
main.go（最终形态）

package main

import (
	"net/http"

	"hg-server/internal/app"
	"hg-server/internal/module/auth"
	"hg-server/internal/module/user"
)

func main() {
	mux := http.NewServeMux()

	app.Register(mux, auth.Routes())
	app.Register(mux, user.Routes())

	http.ListenAndServe(":8080", mux)
}

*/