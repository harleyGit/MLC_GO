# 生产 CNY 商品与库存订单：本机 debug 隔离验收

## 实际执行

- 经用户确认目标为本机 `debug`，迁移前仅核查非敏感地址 `127.0.0.1:3306`、版本 `62 dirty=false`；未触碰 pre/prod，也未启动依赖初始化脚本。
- 使用 `go run ./cmd/hg_config_check --env=debug --config-dir=./config --migrations-dir=./migrations` 执行 `000063`、`000064`，并以新增 `000065` 补商家订单游标索引；事后复查版本 `65 dirty=false`。已执行过的迁移不原地修改；下次变更须新增版本。
- `SERVER_ENV=debug MLC_COMMERCE_INTEGRATION=1 go test -tags production ./internal/modules/commerce -run '^TestHGCommerceDebugLifecycle$' -count=1 -v`：使用随机买家、商家、类目、媒体/地址元数据；HTTP handler 实测草稿、提交、审批、上架、公开详情；同一 SKU 上 100 个不同请求争抢显式盘点的 10 件库存，成功 10、失败 90；重复取消和过期批次不重复释放；待支付地址越权和未付款发货均拒绝。夹具按随机业务 ID 清理，没有使用已有用户或真实支付渠道。

## 000065 阶段历史边界（以下状态已由 000066 增量更新）

- `commerce_inventory` 在审核通过时只登记 **零可售** SKU 库存；商品草稿 `initial_stock` 保留为建议值。测试显式调用盘点接口加库存，仅用于可控夹具。
- `HGOrderStore` 已实现服务端报价、原子库存预占、统一 `trade_order`/明细与独立 CNY 订单/预占事实、幂等重放、取消/有界过期释放、买家归属查询、已付本店订单读取与发货检查。**000065 阶段**未注册创建订单、付款、盘点、发货接口，也未启动过期 worker；最新边界以文末 000066 增量为准。
- 支付宝/微信未提供协议、回调验签/退款资料及隔离账号；没有创建支付尝试、伪造已付款、执行发货或将沙箱支付接到生产订单。商家发货要求 `trade_order` 与真正外部 `payment_order` 同时具备成功事实；目前无生产渠道可生成该事实。

## 000065/000066 阶段遗留阻断项（以文末 000067 当前边界为准）

1. 取得渠道非敏感协议与隔离测试条件，明确渠道订单号、签名、退款及迟到回调语义；实现支付入账、库存 RESERVED→SOLD、异常实收退款/对账，并用相同订单锁序验证竞争。
2. 确认仓库真实库存来源与盘点审计/权限，实现正式有界库存调整接口及权限规则；按业务需要补充运费、售后、订单金额和收件地址隐私策略。
3. 在开启真实下单前配置可停止的有界过期 worker/运维补偿、回调/outbox、告警和容量保护；对亿级订单和热点 SKU 做 EXPLAIN 与实测。000066 阶段已在 debug 显式开关下增加自动 worker，但仍缺少正式多实例补偿、告警和容量证据。
4. 使用专门的隔离测试账号完成真实浏览器、JWT/API Guard、媒体对象存储及商品/订单页面联调；当前集成用例调用实际 handler 和 debug MySQL，但不等于浏览器或真实渠道联调。

## 后续增量：debug 受控订单（000066）

- 本机 debug 经再次确认目标 `127.0.0.1:3306`，迁移前版本 `65 dirty=false`；获本轮明确授权执行库存盘点审计迁移 `000066`，复查 `66 dirty=false`。不对 pre/prod 迁移。
- `SERVER_ENV=debug`、加载配置环境 `debug`、显式 `MLC_COMMERCE_DEBUG_TRADING=true` **三条件同时满足**才允许 HTTP 盘点、预占、取消及订单查询；关闭开关或运行在 pre/prod 时，交易路径 fail-closed，不查未迁移的交易表。旧 CNY 沙箱开关不影响本模块。启用时注册可停止的每5秒/批20单、4秒超时的过期 worker；进程退出时在关闭 MySQL 前等待结束。启用开关前须先完成 `000066`，启动时校验迁移版本和 dirty 状态，未满足则拒绝启动交易模块。
- 已注册 `GET /api/v1/commerce/merchant/inventory`、`POST /merchant/inventory/adjust`、`POST /orders/preview`、`POST /orders`、`GET /orders`、`GET /orders/detail`、`POST /orders/cancel`。盘点仅能调整本人获批店铺的公开 SKU，需乐观锁版本和非空原因，更新与 `commerce_inventory_adjustment` 审计同事务；测试直接调用实际 HTTP handler + 隔离 MySQL。订单由真实服务端计算价格并校验本人地址，只能处于待支付或取消状态。
- 已注册 `POST /orders/payment` 与 `POST /merchant/orders/ship` 的 **不可用响应**，两者在 debug、pre、prod 均返回 503，不创建支付事实、不发货。授权设计 `SIMULATED` 独立 debug 渠道**不等于渠道协议、验签和退款已实现**；目前没有支付宝/微信隔离测试资料，无法声称 debug 完成付款-退款-发货闭环，也不能断言 pre/prod “只接渠道即可正式使用”。正式启用前必须分别验证渠道验签入账、退款/迟到回调对账、订单锁竞争、审计、浏览器 JWT/API Guard 和多实例容量。

## 当前增量：000067 独立 SIMULATED 渠道与页面（仅本机 debug）

- 本机隔离 debug MySQL 已获授权执行 `000067_commerce_simulated_payment.up.sql`，最近一次复查 `migration_version=67 dirty=false`；未在 pre/prod 迁移。原 `000066` 阶段“模拟支付及发货均不可用”的记述为历史状态，**当前仅本机 debug 显式开关**允许 SIMULATED 下单、模拟收款、退款和发货。生产 CNY 商品、旧 debug CNY 沙箱及 M币商品继续分离，模拟流水和退款均不得冒充支付宝或微信资金往来。
- 开启前必须同时满足加载的 `debug` 环境、`SERVER_ENV=debug`、`MLC_COMMERCE_DEBUG_TRADING=true`、迁移版本/非 dirty 检查以及独立 `MLC_COMMERCE_SIMULATED_WEBHOOK_KEY`（至少 32 字节）。业务 handler 与 store 双重 fail-closed；pre/prod 未开放交易写入，不访问未迁移的订单表。密钥只在服务端持有，不由浏览器输入、记录或返回。
- `POST /orders/payment` 只为本人待支付订单创建唯一的 `SIMULATED` 待回调支付尝试，**不表示付款成功**。`POST /admin/simulated-payment/callback` 须经过 JWT、API Guard、商城审核 IAM 权限，再对原始 JSON 字节 HMAC-SHA256 验签；金额、币种、订单、尝试流水、模拟外部流水及事件 ID 都受校验。同一事件和正文可幂等重放，同一事件改金额/流水或已入账后再次支付拒绝。`POST /admin/simulated-payment/settle` 为管理员本机显式模拟收款：仅提交订单 ID，由服务端生成一次性事件并走同一验签状态机；重复入账返回冲突，不向网页提供密钥。**此操作不是外部支付渠道验收**。
- 已收款且未发货的本人订单可以模拟原路退款，更新统一交易、CNY 订单与退款记录；**已售库存不会因退款自动回补可售，需商家按实物盘点处理**。已发货退款拒绝。取消/过期后到达的签名模拟收款保留支付事实并在同一事务登记模拟退款，绝不恢复履约或重新占库存。库存从预占到已售、过期或取消后的释放遵守订单行锁和库存原子更新；商家发货需本店权限与 `SIMULATED` 支付成功证明。只读 `/admin/simulated-payment/reconcile` 检测漂移，不在线修正。
- 隔离 MySQL 用例 `SERVER_ENV=debug MLC_COMMERCE_INTEGRATION=1 go test -tags production ./internal/modules/commerce -run '^TestHGCommerceDebugLifecycle$' -count=1 -v` 通过：随机夹具验证 10 件库存的并发预占守恒、过期释放、事件验签与篡改拒绝、重放、迟到收款退款、管理员生成模拟事件、重复入账冲突、退款/发货竞争及单订单漂移检查。夹具自动清理；不会发起真实支付。`go test -tags production ./internal/modules/commerce/... ./internal/pkg/hg_router ./internal/pkg/config -count=1` 通过，含管理员收款/回调的未登录、错误令牌、无 IAM、错误 API 签名、pre/prod 环境拒绝路径；这些是本地 handler/路由测试，**不是浏览器验收**。
- React debug 工作台接入同一 JWT 与签名网络链，提供本人地址与服务端报价、可重试幂等预占、模拟支付尝试/退款、商家权威库存盘点与发货、管理员服务端模拟收款及只读对账。页面入口仅在 Vite `debug` 模式展示；release/pre 即使直接访问路由也不渲染交易操作，后端环境校验为最终边界。商家页及目录页均明确标注非真实付款。定向 Node VM 测试 18/18、改动文件 ESLint 与 Vite debug 构建通过；VM 测试不启动浏览器或真实 API。

## 当前不能开放 pre/prod 的条件

1. 分别取得支付宝与微信的真实协议、隔离测试条件；完成各自回调验签、金额与外部流水校验、退款及迟到回调对账和负向/竞争验收，不得以 SIMULATED 的 HMAC 替代。
2. 确认生产盘点权威来源、地址隐私与运费/售后政策，以及多实例过期补偿、告警和热点 SKU/亿级订单的 EXPLAIN 与容量实测；本机用例不能证明这些容量条件。
3. 真实浏览器验收仅完成下面记录的**买家只读链路**；商家、管理员、媒体上传和完整交易仍未通过。浏览器账号由用户在隔离配置中亲自输入，未将聊天明文口令放入命令、脚本、日志；不能用本地 handler/VM 测试代替其余浏览器验收。

## 本机浏览器联调增量：部分通过，交易闭环未验收

- 用户在独立 Chrome 配置中手动登录 debug 买家；通过仅监听本机的浏览器调试端口，观察到 `GET /api/v1/profile/addresses` 与 `GET /api/v1/commerce/orders` 均返回 HTTP 200，商品目录 `GET /api/v1/commerce/products` 返回 HTTP 200；页面可渲染买家 debug 工作台。未读取或输出口令、Cookie、JWT、收件信息或完整请求/响应正文。首次运行的原 IDE debug 服务对订单返回 503，定位为未开启独立 debug 交易开关；不能把页面渲染或 HTTP 200 解释成已完成下单。
- 为避免开启交易后暴露在 `*:8080`，经用户确认新增业务 HTTP 监听边界：仅当严格 debug 环境且独立交易开关为 `true` 时，强制绑定 `127.0.0.1`；关闭开关和 pre/prod 保留既有地址。根入口定向单测和 `-tags production` 构建通过，启动后用 `lsof` 核对为 `127.0.0.1:8080`。本机忽略文件中的独立模拟密钥为生成的随机值、保持 `0600`，没有输出；覆盖文件中的交易开关**已恢复 false**，本轮专用进程通过 `MLC_COMMERCE_DEBUG_TRADING=true` 显式开启。不要把此本地进程当作生产开放依据。
- 第二个隔离 Chrome 配置曾被送回 `/login`；用户后来完成手动登录并在浏览器检查时停留于受保护的 `/account/center`，可见“人民币商品·发布审核”菜单。点击后商家工作台渲染、`GET /api/v1/mall/merchant` 和 `GET /api/v1/commerce/admin/products/pending` 均返回 HTTP 200；**但该登录身份未显示已获批商家草稿表单/文件上传控件**，不能据此推定其具备商家经营权限。商家库存/发货、管理员模拟收款及媒体对象存储上传/展示/清理均**未在浏览器执行**。继续验收需要实际获批的隔离商家身份和可下单的隔离商品；无这些前置条件不代替用户创建真实订单。
- 扩大检查范围执行 `go test -tags production . ./internal/pkg/config ./internal/pkg/hg_router -count=1`：`config`、`hg_router` 通过，根包失败在未修改的 `TestHGIMCallTokenRechecksAuthorityAfterRoomCreation` 四个子用例（`im_friendship` SQL mock 预期不符）。本轮回环监听定向测试、生产标签构建和 `go vet -tags production . ./internal/pkg/config ./internal/pkg/hg_router` 通过；**不能将根包全测写成通过**，也不为本次交易验收更改无关 IM 行为。
- 本轮结束时已关闭两个隔离 Chrome 调试窗口并停止本轮启动的回环业务服务；`8080` 未监听。本机覆盖文件交易开关仍为 `false`，密钥仍位于 Git 忽略的 `0600` 本机文件；下次需经确认后再以进程环境显式开启。经用户授权，已仅清理本轮创建的两处隔离浏览器临时配置（会话需要重新手动登录）；未提交任何账号/密钥文件。
