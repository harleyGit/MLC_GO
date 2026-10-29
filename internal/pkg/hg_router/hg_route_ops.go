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

// hgIAMCutoverGone 阻止旧 ops 角色表继续写入或展示过期快照，提示客户端迁移到 IAM。
func hgIAMCutoverGone(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "角色管理已迁移至 /api/v1/iam", http.StatusGone)
}

// opsRoutes 返回 ops 模块完整路由定义。
// 运维模块接口增长较快，单独放在本文件，避免主路由组文件堆积过多 NewRouteSpec 配置。
// /asset-permissions/current 由 JWT 确定当前人，只读数据库活动管理员及启用角色、权限的关联，
// 返回 asset.* 与 payment.recharge_sku.read、payment.recharge_sku.write，供 UI gating 控制入口展示。
// 它不查询余额；展示结果不能替代各业务接口自己的服务端授权，目录分支与实际绑定均需认证。
func opsRoutes(opsHandler *OpsHandlerPackage.Handler) []RouteSpec {
	if opsHandler == nil {
		return []RouteSpec{
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/payment/recharge/skus", true, "创建充值档位", nil, "payment:recharge_sku:write"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/payment/recharge/skus/list", true, "查询充值档位列表", nil, "payment:recharge_sku:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/payment/recharge/skus/update", true, "更新充值档位", nil, "payment:recharge_sku:write"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/payment/recharge/skus/delete", true, "删除充值档位", nil, "payment:recharge_sku:write"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/list", true, "获取角色列表", nil, "iam:role:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/update", true, "更新角色", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/delete", true, "删除角色", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/list", true, "获取管理员列表", nil, "iam:role:read"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/candidates", true, "搜索添加管理员候选用户", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/admins", true, "添加管理员", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/search", true, "搜索管理员用户", nil, "iam:role:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles/list", true, "获取用户角色", nil, "iam:role:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", nil, "ops:menu:write"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus/list", true, "获取菜单列表", nil, "ops:menu:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", nil, "iam:role:grant"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions/list", true, "获取角色权限", nil, "iam:role:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", nil, "ops:file:write"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files/list", true, "获取文件列表", nil, "ops:file:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags", true, "创建Bilibili动画标签", nil, "ops:bilibili_tag:write"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/bilibili/tags/list", true, "获取Bilibili动画标签", nil, "ops:bilibili_tag:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/update", true, "更新Bilibili动画标签", nil, "ops:bilibili_tag:write"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/delete", true, "删除Bilibili动画标签", nil, "ops:bilibili_tag:write"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/users/search", true, "精确搜索硬币操作目标用户", nil, "asset:coin_balance:read", "asset:coin_transaction:read", "asset:coin:grant", "asset:coin:refund", "asset:coin_correction:request").HGWithAnyPermission(),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/accounts/detail", true, "查询硬币权威余额", nil, "asset:coin_balance:read"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/transactions/list", true, "查询硬币资产流水", nil, "asset:coin_transaction:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/grant", true, "人工赠币", nil, "asset:coin:grant"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/refund", true, "原扣款退款", nil, "asset:coin:refund"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/correct", true, "硬币资产修正", nil, "asset:coin_correction:request"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/corrections/approve", true, "审批并应用硬币资产修正", nil, "asset:coin_correction:approve", "asset:coin_correction:apply"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/corrections/list", true, "查询硬币资产修正", nil, "asset:coin_correction:request"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/asset-permissions/current", true, "查询当前资产权限", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/observability/asset-pipeline", true, "查询资产链路状态", nil, "asset:pipeline:read"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/crawler/tasks/debug", true, "测试采集请求并识别字段", nil, "crawler:task:run"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/crawler/tasks/save", true, "保存采集任务", nil, "crawler:task:write"),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/crawler/tasks/save-and-run", true, "保存并运行采集任务", nil, "crawler:task:write", "crawler:task:run"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/crawler/tasks/list", true, "获取持久化采集任务", nil, "crawler:task:read"),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/crawler/tasks/contents", true, "获取采集任务关联内容", nil, "crawler:task:read"),
		}
	}

	return []RouteSpec{
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/payment/recharge/skus", true, "创建充值档位", opsHandler.HGCreateRechargeSKU, "payment:recharge_sku:write"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/payment/recharge/skus/list", true, "查询充值档位列表", opsHandler.HGListRechargeSKUs, "payment:recharge_sku:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/payment/recharge/skus/update", true, "更新充值档位", opsHandler.HGUpdateRechargeSKU, "payment:recharge_sku:write"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/payment/recharge/skus/delete", true, "删除充值档位", opsHandler.HGDeleteRechargeSKU, "payment:recharge_sku:write"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", hgIAMCutoverGone, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/list", true, "获取角色列表", hgIAMCutoverGone, "iam:role:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/update", true, "更新角色", hgIAMCutoverGone, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/delete", true, "删除角色", hgIAMCutoverGone, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/list", true, "获取管理员列表", opsHandler.GetAdminUserList, "iam:role:read"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/admins/candidates", true, "搜索添加管理员候选用户", opsHandler.SearchAdminCandidates, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/admins", true, "添加管理员", opsHandler.AddAdmin, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/search", true, "搜索管理员用户", opsHandler.SearchAdminUsers, "iam:role:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", hgIAMCutoverGone, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles/list", true, "获取用户角色", hgIAMCutoverGone, "iam:role:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", opsHandler.CreateMenu, "ops:menu:write"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus/list", true, "获取菜单列表", opsHandler.GetMenuList, "ops:menu:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", hgIAMCutoverGone, "iam:role:grant"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions/list", true, "获取角色权限", hgIAMCutoverGone, "iam:role:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", opsHandler.UploadFile, "ops:file:write"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files/list", true, "获取文件列表", opsHandler.GetFileList, "ops:file:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags", true, "创建Bilibili动画标签", opsHandler.CreateBilibiliTag, "ops:bilibili_tag:write"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/bilibili/tags/list", true, "获取Bilibili动画标签", opsHandler.GetBilibiliTagList, "ops:bilibili_tag:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/update", true, "更新Bilibili动画标签", opsHandler.UpdateBilibiliTag, "ops:bilibili_tag:write"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/bilibili/tags/delete", true, "删除Bilibili动画标签", opsHandler.DeleteBilibiliTag, "ops:bilibili_tag:write"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/users/search", true, "精确搜索硬币操作目标用户", opsHandler.SearchCoinUser, "asset:coin_balance:read", "asset:coin_transaction:read", "asset:coin:grant", "asset:coin:refund", "asset:coin_correction:request").HGWithAnyPermission(),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/accounts/detail", true, "查询硬币权威余额", opsHandler.GetCoinAccount, "asset:coin_balance:read"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/transactions/list", true, "查询硬币资产流水", opsHandler.GetCoinTransactions, "asset:coin_transaction:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/grant", true, "人工赠币", opsHandler.GrantCoin, "asset:coin:grant"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/refund", true, "原扣款退款", opsHandler.RefundCoin, "asset:coin:refund"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/correct", true, "硬币资产修正", opsHandler.CorrectCoin, "asset:coin_correction:request"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/coin/corrections/approve", true, "审批并应用硬币资产修正", opsHandler.ApproveCoinCorrection, "asset:coin_correction:approve", "asset:coin_correction:apply"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/coin/corrections/list", true, "查询硬币资产修正", opsHandler.ListCoinCorrections, "asset:coin_correction:request"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/asset-permissions/current", true, "查询当前资产权限", opsHandler.GetCurrentAssetPermissions),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/observability/asset-pipeline", true, "查询资产链路状态", opsHandler.GetAssetPipelineStatus, "asset:pipeline:read"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/crawler/tasks/debug", true, "测试采集请求并识别字段", opsHandler.DebugCrawlerTask, "crawler:task:run"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/crawler/tasks/save", true, "保存采集任务", opsHandler.SaveCrawlerTask, "crawler:task:write"),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/crawler/tasks/save-and-run", true, "保存并运行采集任务", opsHandler.SaveAndRunCrawlerTask, "crawler:task:write", "crawler:task:run"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/crawler/tasks/list", true, "获取持久化采集任务", opsHandler.ListCrawlerTasks, "crawler:task:read"),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/crawler/tasks/contents", true, "获取采集任务关联内容", opsHandler.ListCrawlerTaskContents, "crawler:task:read"),
	}
}
