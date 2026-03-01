/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:27:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 19:21:19
 * @FilePath: /MLC_GO/server/hg_router.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGServerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	"net/http"
)

func UserMethodRules() []HGMiddlewarePackage.HGAPIRule {
	return []HGMiddlewarePackage.HGAPIRule{
		{
			Path:    "/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: false,
			// Permissions: []string{
			// 	"user:list",
			// },
		},
		{
			Path:    "/update",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPut: true,
			},
			NeedAuth: false,
			Permissions: []string{
				"user:update",
			},
		},
		{
			Path:    "/info",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: false,
			Permissions: []string{
				"user:view",
			},
		},
	}
}

func PublicAPIRules() []HGMiddlewarePackage.HGAPIRule {
	return []HGMiddlewarePackage.HGAPIRule{
		{
			Path:    "/send_code",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: false,
		},
		{
			Path:    "/register",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: false,
		},
		{
			Path:    "/login",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: false,
		},
	}
}

func MethdRules() []HGMiddlewarePackage.HGAPIRule {

	return []HGMiddlewarePackage.HGAPIRule{
		{
			Path: "/v1/user/update",
			Methods: map[string]bool{
				http.MethodPut: true,
			},
		},
		{
			Path: "/v1/user/info",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
		},
	}
}
