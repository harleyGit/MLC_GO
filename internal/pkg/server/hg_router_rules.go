/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:27:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-06-14 22:37:40
 * @FilePath: /MLC_GO/server/hg_router.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGServerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
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
			Path:    "/detail",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
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
			Path:    "/send_email_code",
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
			Path:    "/register_with_email",
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
		{
			Path:    "/click_captcha",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: false,
		},
		{
			Path:    "/verify_click_captcha",
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

// BilibiliMethodRules 声明作者空间公开读接口的方法白名单。
func BilibiliMethodRules() []HGMiddlewarePackage.HGAPIRule {
	paths := []string{"/author/profile", "/author/stats", "/author/videos", "/author/homepage"}
	rules := make([]HGMiddlewarePackage.HGAPIRule, 0, len(paths))
	for _, path := range paths {
		rules = append(rules, HGMiddlewarePackage.HGAPIRule{Path: path, Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: false})
	}
	return rules
}

func VideoInteractionMethodRules() []HGMiddlewarePackage.HGAPIRule {
	paths := map[string]string{
		"/state": http.MethodGet, "/like": http.MethodPost, "/coin": http.MethodPost,
		"/favorite": http.MethodPost, "/share": http.MethodPost, "/follow": http.MethodPost,
	}
	rules := make([]HGMiddlewarePackage.HGAPIRule, 0, len(paths))
	for path, method := range paths {
		rules = append(rules, HGMiddlewarePackage.HGAPIRule{Path: path, Version: "v1", Methods: map[string]bool{method: true}, NeedAuth: true})
	}
	return rules
}

// VideoRecommendMethodRules 声明认证推荐流的方法白名单。
func VideoRecommendMethodRules() []HGMiddlewarePackage.HGAPIRule {
	return []HGMiddlewarePackage.HGAPIRule{{Path: "/feed", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true}}
}

// VideoCommentMethodRules 声明评论 API 的固定方法白名单和认证要求。
func VideoCommentMethodRules() []HGMiddlewarePackage.HGAPIRule {
	paths := map[string]string{"/create": http.MethodPost, "/list": http.MethodGet, "/replies": http.MethodGet, "/reaction": http.MethodPost, "/image": http.MethodPost, "/delete": http.MethodPost}
	rules := make([]HGMiddlewarePackage.HGAPIRule, 0, len(paths))
	for path, method := range paths {
		rules = append(rules, HGMiddlewarePackage.HGAPIRule{Path: path, Version: "v1", Methods: map[string]bool{method: true}, NeedAuth: true})
	}
	return rules
}

// VideoDanmakuMethodRules 仅包含标准 HTTP 端口上的接口；/ws 由 gnet 自行校验 Origin 和一次性票据。
func VideoDanmakuMethodRules() []HGMiddlewarePackage.HGAPIRule {
	paths := map[string]string{"/create": http.MethodPost, "/list": http.MethodGet, "/ticket": http.MethodPost}
	rules := make([]HGMiddlewarePackage.HGAPIRule, 0, len(paths))
	for path, method := range paths {
		rules = append(rules, HGMiddlewarePackage.HGAPIRule{Path: path, Version: "v1", Methods: map[string]bool{method: true}, NeedAuth: true})
	}
	return rules
}

func OpsMethodRules() []HGMiddlewarePackage.HGAPIRule {
	return []HGMiddlewarePackage.HGAPIRule{
		{
			Path:    "/admins/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles/update",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles/delete",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/admins/candidates",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/admins",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/users/search",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/users/roles",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/users/roles/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/menus",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/menus/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles/permissions",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodPost: true,
			},
			NeedAuth: true,
		},
		{
			Path:    "/roles/permissions/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
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
			Path:    "/files/list",
			Version: "v1",
			Methods: map[string]bool{
				http.MethodGet: true,
			},
			NeedAuth: true,
		},
		{Path: "/bilibili/tags", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/bilibili/tags/list", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/bilibili/tags/update", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/bilibili/tags/delete", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/coin/users/search", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/coin/accounts/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/coin/transactions/list", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/coin/grant", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/coin/refund", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/coin/correct", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/coin/corrections/approve", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/coin/corrections/list", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/asset-permissions/current", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/observability/asset-pipeline", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/crawler/tasks/debug", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/crawler/tasks/save", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/crawler/tasks/save-and-run", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/crawler/tasks/list", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/crawler/tasks/contents", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
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
