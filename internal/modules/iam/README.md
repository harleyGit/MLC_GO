# IAM 管理模块

本模块在 `/api/v1/iam` 提供角色、权限目录和管理员作用域角色绑定。所有接口经现有 JWT 与 API Guard，Service 再检查活动 `admin_user` 和全局 `iam:role:read` 或 `iam:role:grant`。写入及追加式审计在同一个 MySQL 事务提交；MySQL 故障时默认拒绝。旧 `/api/v1/ops` 角色接口暂未迁移，两个角色存储互不混用。

## 管理端协议

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/roles/list?cursor=0&pageSize=20` | 按 `id DESC` 游标列出活动角色，返回 `list/nextCursor/hasMore` |
| POST | `/roles` | 创建角色；`code/name/description/permissions`，权限同事务写入 |
| POST | `/roles/update` | 修改自定义角色名称与描述，使用十进制字符串 `id` |
| POST | `/roles/disable` | 停用没有有效绑定的自定义角色 |
| GET | `/permissions/list?roleId=...` | 权限目录及角色已选权限 |
| POST | `/roles/permissions` | 用 `roleId/permissions` 全量替换自定义角色权限 |
| GET | `/bindings/list?userId=...&cursor=0&pageSize=20` | 按精确管理员业务 ID 列出活动绑定 |
| POST | `/bindings/grant` | 授予 `userId/roleId/scopeType/scopeId/expiresAt` |
| POST | `/bindings/revoke` | 按 `userId/bindingId` 精确撤销 |

`scopeType` 只接受 `global`、`shop`、`org`；`global` 的 `scopeId` 必须为空。非全局作用域须先由可信业务流程登记到 `iam_organizations`，不能使用前端自行填入的资源归属做授权依据。管理端只给活动管理员授予角色，业务用户的角色授予需由对应业务服务接入内部授权流程。单页最多 100 条角色或绑定；角色最多 50 项权限，权限目录超过 200 项时拒绝返回，避免编辑页静默丢失已有权限。

`HGService.Authorize` 是业务服务可调用的授权入口。业务服务须从可信数据源读取资源归属、作用域、状态；写入仍须在 SQL 条件或事务中重复约束归属与状态，避免授权后资源状态变化。当前视频、商城、爬虫等旧接口尚未调用它，管理端的 IAM 权限不能替代这些接口现有的服务端鉴权。

## 发布边界

`migrations/000046_create_iam.up.sql` 创建六张表及内置角色/权限，并从现有活动 `super-admin` 管理员绑定引导新超级管理员。执行迁移是外部数据库写操作，本次代码交付未执行。发布前在隔离环境确认旧角色绑定、数据库会话时区、外键和索引；备份后迁移并验证 IAM 入口可访问。回滚迁移会删除新表及审计数据，不能作为无损回退。

当前授权直接读取 MySQL 权威数据，撤销后无需等待缓存 TTL，但热点业务接口接入前必须设计可失效的缓存、限流、连接池预算与降级策略。审批、MFA、跨服务通知和历史审计归档尚未实现；敏感操作投产需先接入这些控制。容量和延迟尚未压测。
