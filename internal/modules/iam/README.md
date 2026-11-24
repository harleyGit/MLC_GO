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
| GET | `/bindings/list?userId=...&cursor=0&pageSize=20` | 按精确业务用户 ID 列出活动绑定 |
| POST | `/bindings/grant` | 用 `userId/roleId/scopeType/scopeId/expiresAt/mfaCode` 申请授权；返回 `approvalId/status=pending` |
| POST | `/bindings/grant-direct` | 受限即时授权：目标是现有 `users.user_id`（可为普通账号），提交角色、作用域及本人 `mfaCode`，成功返回 `granted=true`；不允许自授或直接授予 `super_admin`，独立审计 |
| POST | `/bindings/grant-direct-batch` | 一次 MFA 提交 `userId/mfaCode/bindings[{roleId,scopeType,scopeId}]`；最多 12 项、4 个角色，所有项共用一个短事务；仅支持已登记的 `global/shop/org`，全部成功或全部回滚 |
| GET | `/scope-types/list?cursor=0&pageSize=50` | 游标读取作用域字典，包含未启用的设计类型；实际授权资格由服务端校验 |
| GET | `/scope-types/detail?code=...` | 按稳定编码查询编辑页回填数据 |
| POST | `/scope-types`、`/scope-types/update`、`/scope-types/disable` | 创建/编辑提交 `code/name/description/status`，`status` 仅接受整数 `0`（停用）或 `1`（启用）；省略或 `null` 时创建默认停用、编辑保留原值；停用有任意历史绑定的类型或 `global/shop/org` 时拒绝 |

`000076` 迁移新增字典和授权表的 `scope_type` 索引，涉及大表 DDL，须在授权环境评估建索引时长与发布窗口。字典预置文档中的八种类型及旧 `org` 兼容编码，初始仅 `global/shop/org` 启用。管理页可修改字典启停状态，但授权接口仍仅支持已接入资源校验的 `global/shop/org`；启用其他类型不会自动接通权威业务资源表与权限引擎。状态、名称与审计在同一短事务保存，编辑停用与删除停用共用历史绑定检查，检查或审计失败均回滚。现有 `status` 列可直接复用，无须新增迁移。文档建议的 `iam_role_scope_types` 角色约束尚未纳入现有授权协议，不能把字典视为角色可授予作用域的证明。
| POST | `/bindings/revoke` | 按 `userId/bindingId` 精确撤销 |
| POST | `/mfa/enroll` | 复核当前登录密码 `password`；一次性返回 `secret/otpauthUri`，禁止缓存；已启用 MFA 返回 HTTP 409 / 业务码 `404008`，不同于普通 IAM 冲突 `100009` |
| POST | `/mfa/confirm` | 用 6 位 `code` 验证并启用 TOTP |
| GET | `/approvals/list` | `id DESC` 游标读取待审批申请，单页最多 50 条 |
| POST | `/approvals/approve`、`/approvals/reject` | `id/mfaCode`；申请人不可自批，24 小时后过期 |

### 使用前需要的身份与权限

路由使用 `permission.HGPermission` 字符串枚举（`internal/pkg/permission/hg_permission.go`）；枚举只是权限编码，不会自动给账号授权。登录令牌中的 `user_id` 必须对应 `admin_user` 中 `status=1`、`is_delete=0` 的活动管理员，并通过有效的 `iam_user_role_bindings` **全局**（`scope_type=global`、`scope_id` 为空）绑定启用角色；该角色在 `iam_role_permissions` 中还须拥有接口要求的权限。手机号、旧 ops 角色和登录密码本身都不是 IAM 授权依据。

| 接口 | 必需权限枚举（字符串值） | 额外条件 |
| --- | --- | --- |
| 角色、绑定、权限目录、审批列表查询 | `HGIAMRoleRead`（`iam:role:read`） | 只读，不能据此执行变更 |
| 创建/更新/停用角色、授权申请与撤销、设置角色权限 | `HGIAMRoleGrant`（`iam:role:grant`） | 授权申请需本人已启用 MFA，部分变更须由另一位合格管理员审批；角色及可委派权限仍受业务校验 |
| 本人 `/mfa/enroll`、`/mfa/confirm` | `HGIAMRoleGrant`（`iam:role:grant`） | enroll 复核当前登录密码、Redis 每人五分钟最多五次尝试及服务端 MFA 加密密钥；confirm 用六位 TOTP 验证并启用，不会赋予角色权限 |
| `/approvals/approve`、`/approvals/reject` | `HGIAMRoleGrant` **和** `HGIAMApprovalApprove`（`iam:approval:approve`） | 本人 MFA 验证；不可审批自己的申请 |
| `/bindings/grant-direct` | `HGIAMRoleGrant` **和** `HGIAMRoleGrantDirect`（`iam:role:grant_direct`） | 仅受控权限可即时授予他人非超管角色；操作者 MFA 消费、可委派权限复核、绑定、缓存版本与独立审计同事务完成。`scopeType` 是绑定字段，不属于角色定义 |

开通方式：由获授权人员先核对目标账号的业务 `user_id`、活动管理员状态、IAM 全局角色绑定及角色权限。内置 `super_admin` 在迁移中预置权限，但仍以当前数据库的有效绑定为准；其他角色应按现有 MFA + 双人审批流程授予权限和绑定，不要修改代码中的枚举或关闭鉴权来放行。首次尚无 MFA 的管理员需要**先已有** `iam:role:grant`，才能通过 `/mfa/enroll` 绑定自己的因子。若绑定或权限缺失，须由已具备权限的其他管理员或受控引导流程处理，不能靠当前账号自助扩权。

`/mfa/enroll` 返回的 `secret/otpauthUri` 用于在认证器中添加动态码，随后调用 `/mfa/confirm` 激活；它不是登录接口，也不负责创建管理员或授予角色。不要记录、分享或缓存密钥。HTTP 403 的“无权执行此操作”还可能来自服务层密码复核失败、MFA 密钥缺失或限流等拒绝；仅凭该响应不能判定缺少 `iam:role:grant`，请由有权限的人员按服务端配置和授权数据逐项核查，不要提交密码或密钥作为排障材料。

`scopeType` 只接受 `global`、`shop`、`org`；`global` 的 `scopeId` 必须为空。`org` 须预先登记于 `iam_organizations`；`shop` 优先使用已登记作用域，未登记时仅允许 `mall_merchant.merchant_id` 唯一点查且审核状态为通过（`status=1`）的真实商家。预校验和授权事务均重读（非锁定查询；并发撤销后业务接口仍须核对商家资格），未通过或不存在返回“作用域不存在或未生效”；不根据客户端提交的 ID 新建作用域，不修改商家审核状态。管理端只给活动管理员授予角色，业务用户的角色授予需由对应业务服务接入内部授权流程。单页最多 100 条角色或绑定；角色最多 50 项权限，权限目录超过 200 项时拒绝返回，避免编辑页静默丢失已有权限。

已上架商品编辑的预判和最终写接口都从商品权威记录取得 `merchant_id`，先校验店铺审核状态、商品归属和状态，再匹配 `shop/merchant_id` 或原有 `global` 的 `mall:product:update`。其他商品操作仍沿用现有全局权限与业务归属规则；授予 `shop` 不表示拥有全店所有业务操作。

`HGService.Authorize` 是业务授权入口。视频上传/投稿、商城商品发布/编辑/补图、商家订单读取与发货在可信 MySQL 资源读取后执行 IAM 判断；写事务重复约束归属和状态。资产、爬虫与商城审核的旧权限编码由适配器映射到 IAM。普通买家订单支付、确认收货和退款继续使用既有订单事务的本人和状态条件。

## 发布边界

路由权限枚举、独立服务目录项、`000048` 的发布顺序和其他角色的受控授权步骤见 [路由权限说明](../../pkg/hg_router/README.md)。`000048` 新增的 7 项权限仅预置给 `super_admin`；其他角色通过现有 MFA 与双人审批流程申请权限及绑定。本次未执行数据库迁移或自动授权。

`migrations/000046_create_iam.up.sql` 创建六张基础表；`000047_iam_cutover_security.up.sql` 追加策略版本、审批和 MFA 表，回填旧角色/权限/绑定并建立创作者、商家业务角色。迁移前在隔离环境确认至少两名不同的活动管理员有授权与审批资格。先备份并停止旧版本角色写入，再由发布流程依次执行两次迁移；本次代码交付未执行迁移。`000047` 回滚只删除新增安全表，已回填 IAM 数据需人工核对，不能视为无损回退。

低风险只读权限使用 20 秒 Redis 缓存，每次命中前先读取 MySQL `iam_policy_revision`；授权变更在同一事务递增版本使旧 key 失效。IAM 管理、资产写入、爬虫执行、临时授权和按用户首次建立的业务角色权限均不缓存放行。Redis 故障回源 MySQL，MySQL 故障拒绝。申请人与审批人各自消费 TOTP 验证码，审批人不能批准本人申请；批准时重新校验双方权限、目标和角色版本，授权及审计同事务提交。

部署须通过密钥管理注入 `MLC_IAM_MFA_KEY_HEX`，值为 32 字节密钥的 64 位十六进制编码。密钥缺失时 MFA 和审批拒绝；密钥遗失或轮换会使现有因子无法解密，须准备受控恢复流程。MFA 密钥用 AES-GCM 加密，30 秒动态码在 MySQL 记录已消费时间步防重放；Redis 限制每名管理员 5 分钟最多 5 次尝试。首次绑定复核项目当前用户密码散列，后续应单独升级该散列算法。跨服务通知、审计归档及容量压测仍待独立建设；发布前需要在隔离环境验证迁移与完整审批链。
