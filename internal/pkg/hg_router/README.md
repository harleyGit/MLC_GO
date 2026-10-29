# 路由权限与独立入口

`RouteSpec` 是接口权限声明源。请求经过公共请求头/签名 Guard、JWT 验证后，由绑定到具体 HTTP 方法的包装器查询 IAM；客户端 `X-Role` 不参与授权。业务服务继续执行资源归属、状态、幂等、MFA 和审批检查。

## 权限枚举

统一使用 [`permission.HGPermission`](../permission/hg_permission.go) 的枚举常量；每个常量注明用途及限制，值与 `000046`、`000047`、`000048` 迁移的 IAM 字典一致。新增常量不会向数据库注册权限，也不会给任何用户或角色授权。

```go
NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath,
    "/roles/list", true, "获取角色列表", nil,
    HGPermissionPackage.HGIAMRoleRead)
```

`Permissions` 使用强类型切片，JSON 仍是字符串数组；`PermissionMatch` 使用枚举，JSON 仍是 `all` 或 `any`。启动时拒绝未登记权限、重复权限、匿名权限、无效匹配模式及超过 16 项的权限集合。配置检查使用代码字典，实际授权使用 IAM 数据，不能混淆。

| 接口类别 | 主要权限枚举 | 规则 |
| --- | --- | --- |
| IAM 角色、绑定、权限目录查询 | `HGIAMRoleRead` | 只读 |
| IAM 角色管理、授权申请、MFA 管理 | `HGIAMRoleGrant` | 业务层继续检查活动管理员、MFA 和审批 |
| IAM 审批裁决 | `HGIAMRoleGrant` + `HGIAMApprovalApprove` | `all`；禁止自批 |
| 运维菜单、文件、Bilibili 标签 | `HGOpsMenu*`、`HGOpsFile*`、`HGOpsBilibiliTag*` | 区分读写权限 |
| 平台用户目录 | `HGUserDirectoryRead` | 本人资料接口不需要此管理权限 |
| 采集任务查询、保存、运行 | `HGCrawlerTaskRead/Write/Run` | 保存并运行要求 `Write` + `Run` |
| 资产余额、流水、赠币、退款 | `HGAssetCoin*` | 各操作独立授权 |
| 硬币修正审批并应用 | `HGAssetCoinCorrectionApprove` + `HGAssetCoinCorrectionApply` | `all`；保留业务双人审批 |
| 硬币操作目标搜索 | 余额读、流水读、赠币、退款、修正申请 | 明确使用 `HGWithAnyPermission()`，任一相关权限可用 |
| 管理端充值档位 | `HGPaymentRechargeSKURead/Write` | 区分读写权限 |
| 商城审核 | `HGMallMerchantReview` | 沿用现有统一审核权限，未擅自切换为商品审核码 |

普通登录用户的本人资料、钱包、买家订单、视频互动等接口保留登录和原有业务校验。创作者与商家的身份建立、投稿和商品归属、店铺订单状态在业务服务查询可信资源后判断；固定 IAM 编码同样引用枚举。`Permissions` 为空不代表可以操作他人资源。旧 `/ops/roles/*` 角色接口在通过认证授权后仍返回 410，客户端应使用 `/api/v1/iam`。

路由权限查询继承请求取消，整组查询预算为 2 秒。缺少可信 JWT 返回 401，无权限返回 403，授权器未注入或权限存储故障返回 503；失败不进入业务 handler，不向客户端暴露数据库错误。现有业务层授权仍保留，会产生额外查询；未进行容量压测。

## 独立服务与专用入口

`HGRouteBindingHTTP` 是默认绑定类型。普通 HTTP 路由必须有对应 Guard 方法、认证配置，即使 handler 为 nil 的目录构建分支也执行检查。

`NewHGExternalRouteSpec` 明确声明由独立服务或专用入口承载的目录项，输出 `binding: "external"`。它不注册标准 HTTP mux，不要求普通 JSON Guard 规则，也不能携带 HTTP handler 或路由 IAM 权限，防止声明了权限却没有执行者。

- `/video_danmaku/ws`：独立 gnet 服务校验一次性票据；普通 HTTP 服务不承载该路径。
- `/video_upload/external-stream` 的 GET/HEAD：现有专用入口校验短期媒体凭证。

两类入口的 `needAuth` 仍为 true。`external` 描述绑定位置，不表示免认证。

## 受控授权与发布

本次没有执行数据库迁移。`000048_route_permissions` 增加菜单、文件、标签、用户目录共 7 项权限，仅给内置 `super_admin` 建立关联。其他角色不会自动获得权限，发布前由获授权人员通过现有 IAM 流程配置：

1. 创建空权限自定义角色，或选定已有可编辑角色。
2. 在 `/iam/roles/permissions` 提交完整权限集合与 MFA，等待另一名合格管理员审批；此接口为全量替换，必须保留所需已有权限。
3. 在 `/iam/bindings/grant` 为目标活动管理员申请角色、作用域和可选有效期，再完成审批。
4. 审批流程复核申请人和审批人的权限、可委派范围及角色状态；授权、策略版本和审计在同一事务写入。

先由发布流程在隔离环境验证迁移，再执行迁移并发布应用。未登记或未授予权限时接口默认拒绝。回滚先回退应用；`000048` 的 down 脚本会删除这 7 项权限及其角色关联，包括上线后的人工授权，需事先保存必要授权记录。不能将代码构建成功视为迁移或完整审批链已验收。

## 验证入口

```bash
go test ./internal/pkg/permission ./internal/pkg/hg_router ./internal/modules/iam/... -count=1
go test ./internal/pkg/middleware -run 'Test(APIGuard|HGAPIGuard)' -count=1
go build -tags production -o /tmp/mlc-go-route-permissions .
HG_EXTERNAL_INTERACTION_CHECK=0 HG_CRAWLER_PIPELINE_INTEGRATION=0 go test -tags production . -count=1
```

权限字典测试只读取迁移文件，不执行 SQL；路由测试使用签名/JWT 请求和可控授权替身，覆盖拒绝、复合权限、依赖故障、JSON 兼容、Guard 配置漂移以及独立入口分离。真实 MySQL/Redis 授权和审批验收需另行在获准的隔离环境执行。
