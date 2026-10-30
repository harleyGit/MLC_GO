// Package permission 集中定义 IAM 权限协议；权限枚举只描述能力，不授予任何用户或角色。
package permission

import "slices"

// HGPermission 是路由可声明的 IAM 权限枚举，字符串值与版本化迁移及 JSON 协议保持一致。
// 谁拥有权限由 MySQL IAM 表及受控授权流程决定，不能根据枚举存在与否直接放行。
type HGPermission string

const (
	HGIAMRoleRead                HGPermission = "iam:role:read"                 // 读取角色、绑定、权限目录和待审批列表；不允许变更授权。
	HGIAMRoleGrant               HGPermission = "iam:role:grant"                // 管理角色及发起授权变更；扩权仍须 MFA、双人审批与可委派权限校验。
	HGIAMApprovalApprove         HGPermission = "iam:approval:approve"          // 裁决 IAM 扩权申请；需同时拥有角色管理权，且不能审批本人申请。
	HGVideoSubmissionRead        HGPermission = "video:submission:read"         // 读取视频投稿；资源可见范围仍由业务层限制。
	HGVideoSubmissionCreate      HGPermission = "video:submission:create"       // 创建本人视频投稿；首次创作者身份由现有业务流程建立。
	HGVideoSubmissionUpdate      HGPermission = "video:submission:update"       // 修改本人草稿或允许编辑的投稿；继续校验归属和稿件状态。
	HGMallProductCreate          HGPermission = "mall:product:create"           // 创建商品草稿；要求商家审核通过并校验商品归属。
	HGMallProductUpdate          HGPermission = "mall:product:update"           // 编辑、归档或补充本人商品；继续执行业务状态及审核流程。
	HGMallProductPublish         HGPermission = "mall:product:publish"          // 商品发布能力；枚举声明不替代业务审核与上架条件。
	HGMallMerchantReview         HGPermission = "mall:merchant:review"          // 现有商城统一审核入口权限，覆盖商家、商品、补图和编辑审核。
	HGMallProductReview          HGPermission = "mall:product:review"           // 独立商品审核权限码；当前统一审核入口仍使用 HGMallMerchantReview。
	HGMallOrderRead              HGPermission = "mall:order:read"               // 读取本人店铺订单；不能跨店读取订单或买家隐私。
	HGMallOrderShip              HGPermission = "mall:order:ship"               // 为本人店铺订单填写物流并发货；要求订单状态允许发货。
	HGCrawlerTaskRead            HGPermission = "crawler:task:read"             // 读取持久化采集任务及关联内容。
	HGCrawlerTaskWrite           HGPermission = "crawler:task:write"            // 创建或保存采集任务；不单独授予运行能力。
	HGCrawlerTaskRun             HGPermission = "crawler:task:run"              // 调试采集请求或运行任务；保存并运行还需 HGCrawlerTaskWrite。
	HGCrawlerTaskExecute         HGPermission = "crawler:task:execute"          // 保留早期采集执行权限码；不自动等价于当前 HGCrawlerTaskRun。
	HGOpsMetricsRead             HGPermission = "ops:metrics:read"              // 读取运维指标；不等价于资产链路查询或部署执行权限。
	HGOpsRBACManage              HGPermission = "ops:rbac:manage"               // 旧 RBAC 兼容权限码；不能恢复已返回 410 的旧角色管理入口。
	HGOpsMenuRead                HGPermission = "ops:menu:read"                 // 读取运维菜单配置。
	HGOpsMenuWrite               HGPermission = "ops:menu:write"                // 创建或管理运维菜单配置。
	HGOpsFileRead                HGPermission = "ops:file:read"                 // 读取运维文件列表；不授予上传能力。
	HGOpsFileWrite               HGPermission = "ops:file:write"                // 通过运维接口上传文件；文件类型和大小校验仍由业务层执行。
	HGOpsBilibiliTagRead         HGPermission = "ops:bilibili_tag:read"         // 读取 Bilibili 动画标签管理配置。
	HGOpsBilibiliTagWrite        HGPermission = "ops:bilibili_tag:write"        // 创建、更新或删除 Bilibili 动画标签配置。
	HGUserDirectoryRead          HGPermission = "user:directory:read"           // 查询平台用户目录；查询本人资料不需要该管理权限。
	HGAssetCoinBalanceRead       HGPermission = "asset:coin_balance:read"       // 查询目标用户的硬币权威余额。
	HGAssetCoinTransactionRead   HGPermission = "asset:coin_transaction:read"   // 查询目标用户的硬币资产流水。
	HGAssetCoinGrant             HGPermission = "asset:coin:grant"              // 人工赠币；继续执行限流、幂等和审计约束。
	HGAssetCoinRefund            HGPermission = "asset:coin:refund"             // 按原扣款记录退款；不允许无关联交易退款。
	HGAssetCoinCorrectionRequest HGPermission = "asset:coin_correction:request" // 申请及查询硬币资产修正；不能自行审批或应用修正。
	HGAssetCoinCorrectionApprove HGPermission = "asset:coin_correction:approve" // 审批硬币资产修正；审批并应用还需 HGAssetCoinCorrectionApply。
	HGAssetCoinCorrectionApply   HGPermission = "asset:coin_correction:apply"   // 应用已批准的硬币修正；不能替代双人审批。
	HGAssetPipelineRead          HGPermission = "asset:pipeline:read"           // 查询资产投影链路及检查点状态。
	HGPaymentRechargeSKURead     HGPermission = "payment:recharge_sku:read"     // 读取管理端充值档位，包括管理所需状态信息。
	HGPaymentRechargeSKUWrite    HGPermission = "payment:recharge_sku:write"    // 创建、更新或删除充值档位；不授予支付或余额修改能力。
	HGNotificationChannelRead    HGPermission = "notification:channel:read"     // 查看站内信与未接供应商渠道的真实可用状态。
	HGNotificationDeliveryRead   HGPermission = "notification:delivery:read"    // 查看站内通知投递记录，不包括私信内容。
	HGNotificationChannelSend    HGPermission = "notification:channel:send"     // 仅向本人发送有界站内试通知，不是运营群发。
)

// hgPermissions 是编译期权限字典；新增权限须同步受控迁移，不在路由注册时写库。
var hgPermissions = [...]HGPermission{
	HGIAMRoleRead,
	HGIAMRoleGrant,
	HGIAMApprovalApprove,
	HGVideoSubmissionRead,
	HGVideoSubmissionCreate,
	HGVideoSubmissionUpdate,
	HGMallProductCreate,
	HGMallProductUpdate,
	HGMallProductPublish,
	HGMallMerchantReview,
	HGMallProductReview,
	HGMallOrderRead,
	HGMallOrderShip,
	HGCrawlerTaskRead,
	HGCrawlerTaskWrite,
	HGCrawlerTaskRun,
	HGCrawlerTaskExecute,
	HGOpsMetricsRead,
	HGOpsRBACManage,
	HGOpsMenuRead,
	HGOpsMenuWrite,
	HGOpsFileRead,
	HGOpsFileWrite,
	HGOpsBilibiliTagRead,
	HGOpsBilibiliTagWrite,
	HGUserDirectoryRead,
	HGAssetCoinBalanceRead,
	HGAssetCoinTransactionRead,
	HGAssetCoinGrant,
	HGAssetCoinRefund,
	HGAssetCoinCorrectionRequest,
	HGAssetCoinCorrectionApprove,
	HGAssetCoinCorrectionApply,
	HGAssetPipelineRead,
	HGPaymentRechargeSKURead,
	HGPaymentRechargeSKUWrite,
	HGNotificationChannelRead,
	HGNotificationDeliveryRead,
	HGNotificationChannelSend,
}

// HGKnown 判断是否为已登记的路由权限；仅用于启动配置校验，不代替 IAM 授权查询。
func (hgPermission HGPermission) HGKnown() bool {
	return slices.Contains(hgPermissions[:], hgPermission)
}

// HGPermissions 返回权限字典副本，供契约检查使用；修改返回值不影响启动期校验。
func HGPermissions() []HGPermission { return slices.Clone(hgPermissions[:]) }
