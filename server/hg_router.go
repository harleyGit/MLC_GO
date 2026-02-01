/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:27:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 16:08:40
 * @FilePath: /MLC_GO/server/hg_router.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGServerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	"net/http"
)

func UserMethodRules() []HGMiddlewarePackage.HGMethodRule {
	return []HGMiddlewarePackage.HGMethodRule{
		{
			Path: "/update",
			Methods: map[string]bool{
				http.MethodPut: true,
			},
		},
		{
			Path: "/info",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
		},
	}
}

func PublicMethodRules() []HGMiddlewarePackage.HGMethodRule {
	return []HGMiddlewarePackage.HGMethodRule{
		{
			Path: "/send_code",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
		},
		{
			Path: "/register",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
		},
		{
			Path: "/login",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
		},
	}
}

func MethdRules() []HGMiddlewarePackage.HGMethodRule {

	return []HGMiddlewarePackage.HGMethodRule{
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
