# IAM 作用域：类型、资源与权限生效边界

## 为什么以前只有三种可分配

`iam_scope_types` 是**类型字典**，不是具体资源清单。迁移 `000076` 已登记
`global/shop/org/creator/organization/environment/crawler_project/content_channel/region`。
前 3 种原本可授权：`global` 无资源 ID；`shop` 来自已登记店铺或审核通过的商家；
`org` 来自已启用的 `iam_organizations`。其余 6 种初始为 `status=0`，
既无资源登记，也不在即时授权与 `Authorize` 允许列表中；只将字典状态改为 1 并不会让业务 API 受控。
`org` 是旧编码，`organization` 是新编码，两者不共用 ID。

## 本次第一阶段（迁移 `000080`，未自动执行）

六类新类型在迁移后状态为 1，新增 `iam_scope_resources` 登记表。
只有活动管理员同时持有全局 `iam:role:grant` 和 `iam:role:grant_direct`
才可通过 `POST /api/v1/iam/scope-resources` 传入 `{scopeType,name}` 登记。
服务端生成不可变随机 ID，审计登记行为；`created_by + request_id` 保证同一次请求幂等。
`GET /api/v1/iam/scope-resources/list?scopeType=...&cursor=...&pageSize=...`
分页返回已启用资源的 `id/name/nextCursor/hasMore`，最多 50 条/页。
编辑用户页只展示启用且服务端已支持的类型；选中后从候选目录选择 ID，
仅有一个候选时自动填充。没有候选时不能通过构造 ID 授权。

即时授权在入参校验及写入事务内复核类型状态和资源存在性，继续保持角色权限委派、
禁止自授、禁止直接授予超管、MFA 与审计等原有约束。显式调用
`Authorize(ctx,userID,permission,HGResource{ScopeType:...,ScopeID:...})`
时还会拒绝不存在或停用的新登记资源；**仅角色绑定或前端显示并不等于业务操作可用**。

## 尚未接入的业务语义

登记的是 IAM 候选资源，而非创作者账号、企业组织、运行环境、爬虫项目、
内容频道或区域的实际业务实体。本阶段不凭登记推断资源归属、业务状态或租户资格；
现有视频、爬虫、运维等接口仍按原有规则鉴权，不因新增绑定自动改权。
逐模块上线前必须确定真实业务实体与 IAM 资源 ID 的映射、创建/停用同步、
用户归属/跨租户边界、服务端传入的可信 `HGResource` 及负向测试；不能把客户端 ID 当作来源。
新增资源当前不提供停用接口，避免在尚无业务生命周期规则时误撤销已绑定的资源。

## 发布与回退

先在隔离环境审阅并运行 `000080`，再部署包含登记查询和授权校验的新服务与前端；
混合新旧服务实例时，旧实例仍会拒绝六类绑定，不能视为完成切换。
迁移下行脚本主动拒绝自动回滚：若已登记资源或产生绑定，必须先按受控流程
核对业务影响并撤销，不能直接删除表或静默停用类型。验收不得直接写生产依赖。
