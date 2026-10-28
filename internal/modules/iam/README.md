# IAM 管理模块

本模块在 `/api/v1/iam` 提供角色、资源授权、管理员 MFA 与双人审批。接口经 JWT 与 API Guard，Service 再检查活动管理员和当前 IAM 权限。授权、策略版本及审计在同一 MySQL 事务提交；MySQL 故障默认拒绝。`000047` 将旧 ops 角色一次性回填，旧 ops 角色读写路由返回 410。

## 管理端协议

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/roles/list?cursor=0&pageSize=20` | 按 `id DESC` 游标列出活动角色，返回 `list/nextCursor/hasMore` |
| POST | `/roles` | 使用 `code/name/description` 创建空权限角色；权限另行申请审批 |
| POST | `/roles/update` | 修改自定义角色名称与描述，使用十进制字符串 `id` |
| POST | `/roles/disable` | 停用没有有效绑定的自定义角色 |
| GET | `/permissions/list?roleId=...` | 权限目录及角色已选权限 |
| POST | `/roles/permissions` | 用 `roleId/permissions/mfaCode` 申请全量替换；返回 `approvalId/status=pending` |
| GET | `/bindings/list?userId=...&cursor=0&pageSize=20` | 按精确管理员业务 ID 列出活动绑定 |
| POST | `/bindings/grant` | 用 `userId/roleId/scopeType/scopeId/expiresAt/mfaCode` 申请授权；返回 `approvalId/status=pending` |
| POST | `/bindings/revoke` | 按 `userId/bindingId` 精确撤销 |
| POST | `/mfa/enroll` | 复核当前登录密码 `password`；一次性返回 `secret/otpauthUri`，禁止缓存 |
| POST | `/mfa/confirm` | 用 6 位 `code` 验证并启用 TOTP |
| GET | `/approvals/list` | `id DESC` 游标读取待审批申请，单页最多 50 条 |
| POST | `/approvals/approve`、`/approvals/reject` | `id/mfaCode`；申请人不可自批，24 小时后过期 |

`scopeType` 只接受 `global`、`shop`、`org`；`global` 的 `scopeId` 必须为空。非全局作用域须先由可信业务流程登记到 `iam_organizations`，不能使用前端自行填入的资源归属做授权依据。管理端只给活动管理员授予角色，业务用户的角色授予需由对应业务服务接入内部授权流程。单页最多 100 条角色或绑定；角色最多 50 项权限，权限目录超过 200 项时拒绝返回，避免编辑页静默丢失已有权限。

`HGService.Authorize` 是业务授权入口。视频上传/投稿、商城商品发布/编辑/补图、商家订单读取与发货在可信 MySQL 资源读取后执行 IAM 判断；写事务重复约束归属和状态。资产、爬虫与商城审核的旧权限编码由适配器映射到 IAM。普通买家订单支付、确认收货和退款继续使用既有订单事务的本人和状态条件。

## 发布边界

`migrations/000046_create_iam.up.sql` 创建六张基础表；`000047_iam_cutover_security.up.sql` 追加策略版本、审批和 MFA 表，回填旧角色/权限/绑定并建立创作者、商家业务角色。迁移前在隔离环境确认至少两名不同的活动管理员有授权与审批资格。先备份并停止旧版本角色写入，再由发布流程依次执行两次迁移；本次代码交付未执行迁移。`000047` 回滚只删除新增安全表，已回填 IAM 数据需人工核对，不能视为无损回退。

低风险只读权限使用 20 秒 Redis 缓存，每次命中前先读取 MySQL `iam_policy_revision`；授权变更在同一事务递增版本使旧 key 失效。IAM 管理、资产写入、爬虫执行、临时授权和按用户首次建立的业务角色权限均不缓存放行。Redis 故障回源 MySQL，MySQL 故障拒绝。申请人与审批人各自消费 TOTP 验证码，审批人不能批准本人申请；批准时重新校验双方权限、目标和角色版本，授权及审计同事务提交。

部署须通过密钥管理注入 `MLC_IAM_MFA_KEY_HEX`，值为 32 字节密钥的 64 位十六进制编码。密钥缺失时 MFA 和审批拒绝；密钥遗失或轮换会使现有因子无法解密，须准备受控恢复流程。MFA 密钥用 AES-GCM 加密，30 秒动态码在 MySQL 记录已消费时间步防重放；Redis 限制每名管理员 5 分钟最多 5 次尝试。首次绑定复核项目当前用户密码散列，后续应单独升级该散列算法。跨服务通知、审计归档及容量压测仍待独立建设；发布前需要在隔离环境验证迁移与完整审批链。
