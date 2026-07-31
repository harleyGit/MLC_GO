/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-06-13 19:43:11
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-25 18:40:46
 * @FilePath: /MLC_GO/internal/pkg/hg_router/hg_route_ops.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGRouterPackage

import (
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	"net/http"
)

// opsRoutes 返回 ops 模块完整路由定义。
// 运维模块接口增长较快，单独放在本文件，避免主路由组文件堆积过多 NewRouteSpec 配置。
func opsRoutes(opsHandler *OpsHandlerPackage.Handler) []RouteSpec {
	if opsHandler == nil {
		return []RouteSpec{
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/list", true, "获取角色列表", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/update", true, "更新角色", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/delete", true, "删除角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/list", true, "获取管理员列表", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/candidates", true, "搜索添加管理员候选用户", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/admins", true, "添加管理员", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/search", true, "搜索管理员用户", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles/list", true, "获取用户角色", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus/list", true, "获取菜单列表", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions/list", true, "获取角色权限", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files/list", true, "获取文件列表", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags", true, "创建Bilibili动画标签", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/bilibili/tags/list", true, "获取Bilibili动画标签", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/update", true, "更新Bilibili动画标签", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/delete", true, "删除Bilibili动画标签", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/accounts/detail", true, "查询硬币权威余额", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/transactions/list", true, "查询硬币资产流水", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/grant", true, "人工赠币", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/refund", true, "原扣款退款", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/correct", true, "硬币资产修正", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/corrections/approve", true, "审批并应用硬币资产修正", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/corrections/list", true, "查询硬币资产修正", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/asset-permissions/current", true, "查询当前资产权限", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/observability/asset-pipeline", true, "查询资产链路状态", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", opsHandler.CreateRole),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/list", true, "获取角色列表", opsHandler.GetRoleList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/update", true, "更新角色", opsHandler.UpdateRole),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/delete", true, "删除角色", opsHandler.DeleteRole),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/list", true, "获取管理员列表", opsHandler.GetAdminUserList),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/candidates", true, "搜索添加管理员候选用户", opsHandler.SearchAdminCandidates),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/admins", true, "添加管理员", opsHandler.AddAdmin),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/search", true, "搜索管理员用户", opsHandler.SearchAdminUsers),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", opsHandler.AssignUserRoles),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles/list", true, "获取用户角色", opsHandler.GetUserRoles),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", opsHandler.CreateMenu),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus/list", true, "获取菜单列表", opsHandler.GetMenuList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", opsHandler.AssignRolePermissions),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions/list", true, "获取角色权限", opsHandler.GetRolePermissions),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", opsHandler.UploadFile),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files/list", true, "获取文件列表", opsHandler.GetFileList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags", true, "创建Bilibili动画标签", opsHandler.CreateBilibiliTag),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/bilibili/tags/list", true, "获取Bilibili动画标签", opsHandler.GetBilibiliTagList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/update", true, "更新Bilibili动画标签", opsHandler.UpdateBilibiliTag),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/delete", true, "删除Bilibili动画标签", opsHandler.DeleteBilibiliTag),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/accounts/detail", true, "查询硬币权威余额", opsHandler.GetCoinAccount),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/transactions/list", true, "查询硬币资产流水", opsHandler.GetCoinTransactions),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/grant", true, "人工赠币", opsHandler.GrantCoin),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/refund", true, "原扣款退款", opsHandler.RefundCoin),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/correct", true, "硬币资产修正", opsHandler.CorrectCoin),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/corrections/approve", true, "审批并应用硬币资产修正", opsHandler.ApproveCoinCorrection),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/corrections/list", true, "查询硬币资产修正", opsHandler.ListCoinCorrections),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/asset-permissions/current", true, "查询当前资产权限", opsHandler.GetCurrentAssetPermissions),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/observability/asset-pipeline", true, "查询资产链路状态", opsHandler.GetAssetPipelineStatus),
	}
}
