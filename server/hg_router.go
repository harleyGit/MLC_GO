/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:27:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-04 10:12:32
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
			Path:    "/info",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
			Permissions: []string{
				"user:view",
			},
		},
		{
			Path:    "/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: false,
		},
		{
			Path:    "/update",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPut: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/account",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/security",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPut: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/avatar",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true, // 上传头像
				http.MethodGet:  true, // 获取头像
			},
			NeedAuth: true,
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
		{
			Path:    "/refresh",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: false,
		},
		{
			Path:    "/send_reset_code",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: false,
		},
		{
			Path:    "/reset_password",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: false,
		},
	}
}

func VideoUploadMethodRules() []HGMiddlewarePackage.HGAPIRule {
	return []HGMiddlewarePackage.HGAPIRule{
		{
			Path:    "/upload",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/draft",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/submit",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/cover",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
	}
}

func OpsMethodRules() []HGMiddlewarePackage.HGAPIRule {
	return []HGMiddlewarePackage.HGAPIRule{
		{
			Path:    "/roles",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
				http.MethodGet:  true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/users/roles",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
				http.MethodGet:  true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/menus",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
				http.MethodGet:  true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles/permissions",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
				http.MethodGet:  true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/files/upload",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/files",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
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
