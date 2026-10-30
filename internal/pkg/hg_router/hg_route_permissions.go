package HGRouterPackage

import (
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	HGResponsePackage "MLC_GO/internal/response"
	"context"
	"net/http"
	"slices"
	"time"
)

// HGRouteAuthorizer 在 JWT 验证后查询 IAM 全局权限；生产注入现有 IAM 管理员适配器。
// 不接受 X-Role、请求体 userID 或客户端权限列表作为授权依据。
type HGRouteAuthorizer interface {
	HasAssetPermission(context.Context, string, string) (bool, error)
}

func hgFirstAuthorizer(hgItems []HGRouteAuthorizer) HGRouteAuthorizer {
	if len(hgItems) > 0 {
		return hgItems[0]
	}
	return nil
}

// hgRouteGuardRules 从标准 HTTP 路由生成版本、方法和认证白名单；独立入口自行认证。
// Guard 位于 JWT 之前，只执行协议检查；权限由绑定到具体方法的 handler 包装器检查。
func hgRouteGuardRules(hgSpecs []RouteSpec) []HGMiddlewarePackage.APIRule {
	hgRules := make([]HGMiddlewarePackage.APIRule, 0, len(hgSpecs))
	hgIndexes := make(map[string]int, len(hgSpecs))
	for _, hgSpec := range hgSpecs {
		hgValidateRouteBinding(hgSpec)
		if hgSpec.Binding == HGRouteBindingExternal {
			continue
		}
		hgValidateRoutePermissions(hgSpec)
		if hgSpec.Version == "" || hgSpec.SubPath == "" || hgSpec.Method == "" {
			panic("HTTP 路由缺少版本、路径或方法: " + hgSpec.FullPath)
		}
		hgKey := hgSpec.Version + " " + hgSpec.SubPath
		if hgIndex, hgFound := hgIndexes[hgKey]; hgFound {
			hgRule := &hgRules[hgIndex]
			if hgRule.NeedAuth != hgSpec.NeedAuth || hgRule.Methods[hgSpec.Method] {
				panic("同路径路由的认证声明冲突或方法重复: " + hgSpec.FullPath)
			}
			hgRule.Methods[hgSpec.Method] = true
			continue
		}
		hgIndexes[hgKey] = len(hgRules)
		hgRules = append(hgRules, HGMiddlewarePackage.APIRule{
			Path: hgSpec.SubPath, Version: hgSpec.Version,
			Methods: map[string]bool{hgSpec.Method: true}, NeedAuth: hgSpec.NeedAuth,
		})
	}
	return hgRules
}

func hgValidateRoutePermissions(hgSpec RouteSpec) {
	if len(hgSpec.Permissions) == 0 {
		if hgSpec.PermissionMatch != HGPermissionMatchNone {
			panic("空权限列表不能声明匹配模式: " + hgSpec.FullPath)
		}
		return
	}
	if !hgSpec.NeedAuth || len(hgSpec.Permissions) > 16 || (hgSpec.PermissionMatch != HGPermissionMatchAll && hgSpec.PermissionMatch != HGPermissionMatchAny) {
		panic("路由权限必须认证且使用有界 all/any 声明: " + hgSpec.FullPath)
	}
	for hgIndex, hgPermission := range hgSpec.Permissions {
		if !hgPermission.HGKnown() || slices.Contains(hgSpec.Permissions[:hgIndex], hgPermission) {
			panic("路由权限编码无效或重复: " + hgSpec.FullPath)
		}
	}
}

func hgPermissionHandler(hgSpec RouteSpec, hgAuthorizer HGRouteAuthorizer) http.Handler {
	hgValidateRoutePermissions(hgSpec)
	hgPermissions := slices.Clone(hgSpec.Permissions)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hgSpec.NeedAuth && len(hgPermissions) == 0 {
			hgSpec.Handler(w, r)
			return
		}
		hgUserID, hgOK := HGContextPackage.CurrentUserID(r)
		if !hgOK {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePackage.FailTokenInvalid(w, r, "登录状态无效")
			return
		}
		if len(hgPermissions) == 0 {
			hgSpec.Handler(w, r)
			return
		}
		if hgAuthorizer == nil {
			hgPermissionError(w, r, http.StatusServiceUnavailable)
			return
		}
		// 查询总预算包含所有复合权限，继承请求取消；不为每个权限重新延长超时。
		hgCtx, hgCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer hgCancel()
		for _, hgPermission := range hgPermissions {
			hgAllowed, hgErr := hgAuthorizer.HasAssetPermission(hgCtx, hgUserID, string(hgPermission))
			if hgErr != nil || hgCtx.Err() != nil {
				hgPermissionError(w, r, http.StatusServiceUnavailable)
				return
			}
			if hgSpec.PermissionMatch == HGPermissionMatchAny && hgAllowed {
				hgSpec.Handler(w, r)
				return
			}
			if hgSpec.PermissionMatch == HGPermissionMatchAll && !hgAllowed {
				hgPermissionError(w, r, http.StatusForbidden)
				return
			}
		}
		if hgSpec.PermissionMatch == HGPermissionMatchAny {
			hgPermissionError(w, r, http.StatusForbidden)
			return
		}
		hgSpec.Handler(w, r)
	})
}

func hgPermissionError(w http.ResponseWriter, r *http.Request, hgStatus int) {
	hgResult := HGResponsePackage.HGErrorResult{Code: HGResponsePackage.Forbidden.Code, Message: "无权执行此操作"}
	if hgStatus == http.StatusServiceUnavailable {
		hgResult = HGResponsePackage.HGErrorResult{Code: HGResponsePackage.InternalError.Code, Message: "权限服务暂不可用"}
	}
	w.WriteHeader(hgStatus)
	HGResponsePackage.FailResult[string](w, r, hgResult)
}
