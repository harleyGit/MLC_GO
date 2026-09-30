# Debug 模拟充值

模拟充值是隔离测试用途的真实 coin 资产写入，不是免费币功能，也不会在真实环境默认开启。

## 开关

仅当进程环境同时满足以下条件时可用：

```text
SERVER_ENV=debug
MLC_WALLET_DEBUG_PAYMENT_ENABLED=true
```

`pre`、`prod` 或未显式设置开关均拒绝 `platform_debug`，并且开关不会写入任何真实环境配置文件。微信和支付宝当前未配置，始终返回“支付渠道未配置，暂不能付款”，不会退化为模拟支付。

还必须由正常启动流程 `LoadConfig("debug")` 实际加载 debug 配置，配置未加载或已加载 pre/prod 时即使进程变量为 debug 也拒绝。不要绕过配置加载。确认隔离依赖且获准启动后可使用 `SERVER_ENV=debug MLC_WALLET_DEBUG_PAYMENT_ENABLED=true go run -tags production .`；本轮未执行启动。关闭开关并重启所有实例后，既有 pending 订单拒付，paid 的同渠道重试仍只读返回原结果。不要混用不同配置的实例。

上线代码前需单独审批和执行 `000034_wallet_debug_payment.up.sql`；本轮只新增迁移文件，没有执行，也未改动已执行的 `000033` 或现有配置迁移目标。旧订单默认 `unavailable`，开启开关不能将其升级为 debug 订单，需用新的 requestId 创建订单。创建幂等重放始终保留原模式、原快照和原有效期。未来真实支付实现也必须校验绑定模式，不得接受 `platform_debug` 订单。

## HTTP 协议

VS Code 的 `Launch MLC_GO Debug` 启动项显式注入上述开关，并使用 `-tags=production` 选择业务入口（不是 prod 环境）。已有进程必须重启才会取得新配置；pre/prod 启动项不注入此开关。注意该 debug 启动项的 `ensure-debug-deps` 前置任务会启动依赖并执行 debug 迁移，必须单独确认授权，不能当作只读检查运行。

按钮不可用时先看订单响应：`paymentMode=unavailable` 表示创建时没有绑定 debug 能力，重启后也不能升级旧订单，应返回充值中心重新选档，用新的 requestId 创建订单；`platform_debug` 但能力未开放时，核对上述三个条件和实际连接的后端，再重新查询原订单。查询失败先重查原订单，不能把网络错误当作未入账或自动新建订单。缺少迁移可能导致 HTTP 500，但不能仅凭按钮禁用推断迁移未执行。

创建订单仍为 `POST /api/v1/wallet/recharge/orders`，请求为 `{"skuId":"...","requestId":"..."}`。服务端在订单中绑定创建时支付模式：debug 开关开启时为 `platform_debug`，否则为 `unavailable`。

付款为严格请求：

```json
{"orderId":"<orderId>","paymentMethod":"platform_debug"}
```

`paymentMethod` 必填且只接受 `platform_debug`、`wechat`、`alipay`。订单响应包含 `status`（`pending`、`expired`、`paid`）、`paymentMode`、`paymentAvailable`、`availableMethods`；成功入账的响应额外包含 `paidAt` 和十进制字符串 `balanceAfter`。

重复点击或回放同一订单只会产生一次 coin 入账；已支付订单重试返回已保存的原结果。订单 owner、模式和有效期均由服务端校验。订单状态、coin request/transaction、lot、outbox 在一个 MySQL 短事务中提交，失败全部回滚；请求超时可通过订单详情查询最终状态。

单笔充值继续受 coin 权威上限 1000 币约束。不要在本地通过迁移、真实依赖或生产配置验证此开关；启用前必须使用隔离 debug 数据库并完成数据清理评估。

## 前端对接细节

所有接口沿用签名守卫和JWT，不是匿名接口。响应沿用现有 `code/message/result` 包装，付款成功的 `result` 与创建/详情相同。详情为 `GET /api/v1/wallet/recharge/orders/detail?orderId=...`，当前余额为 `GET /api/v1/wallet/balance`。

- 仅在 `paymentAvailable=true` 且 `availableMethods` 包含 `platform_debug` 时展示模拟付款按钮，明确标注“隔离测试模拟充值，写入真实平台币”，不要写成微信/支付宝付款。
- `availableMethods` 只会为 `[]` 或 `["platform_debug"]`，微信和支付宝保留未配置提示，不能自动替换请求渠道。
- `paymentMode` 是不可变的 `platform_debug` 或 `unavailable`；`paymentAvailable` 为当前可否新付款，paid/expired 一律 false。
- `paidAt` 是 UTC RFC3339Nano 字符串，`balanceAfter` 是当次入账后的十进制余额字符串，只有 paid 返回；后续消费不改变此快照。刷新当前余额须调用 balance。
- 金额 `payAmount` 为人民币分，`totalCoin` 为币数；模拟模式不收取人民币，不承诺分与币固定兑换率。
- 缺失/未知 paymentMethod、非法orderId、多JSON及未知请求字段：HTTP 400，code 100001。
- 他人/不存在订单：HTTP 404，code 100001，统一“订单不存在或无权访问”。
- debug订单过期：HTTP 409，沿用ConflictCode，message“充值订单已过期”。
- 模式不符、开关关闭或微信/支付宝未配置：HTTP 503，沿用InternalError.Code，message“支付渠道未配置，暂不能付款”。
- 超过1000币：HTTP 422，code 100001；数据库/事务失败：HTTP 500，沿用InternalError.Code，不暴露内部诊断。
- 超时/网络断开/500不代表一定未入账。先查详情，必要时同orderId、同paymentMethod有限重试，不能自动新建订单；paid返回原paidAt/balanceAfter。不要依据单次报错覆盖已知paid状态。

## 风险与验证边界

debug免费获得的是可消费的真实coin，必须隔离MySQL及其outbox下游，禁止连接生产数据。事务内无Kafka/Redis/HTTP调用；outbox沿用 `mlc.domain.events` 异步投递。单请求5秒预算、16KiB请求体、单笔最多1000币；同用户写锁存在热点，继承现有网关/鉴权链，不新增无限队列或后台任务。没有新增累计日限额，隔离debug用户仍能反复新建订单获取币，不得当成生产支付。

点查依赖000033的订单唯一键及coin现有唯一键；实际生产索引和DDL锁表成本未验证。000034增加列，无批量资产更新；已有paid数据时禁止执行down迁移或删除支付标记，否则会丢失审计语义。没有容量压测或真实InnoDB并发验收，不宣称已验证生产规模。
