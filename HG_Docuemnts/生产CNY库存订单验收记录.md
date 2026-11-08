# 生产 CNY 商品与库存订单：本机 debug 隔离验收

## 实际执行

- 经用户确认目标为本机 `debug`，迁移前仅核查非敏感地址 `127.0.0.1:3306`、版本 `62 dirty=false`；未触碰 pre/prod，也未启动依赖初始化脚本。
- 使用 `go run ./cmd/hg_config_check --env=debug --config-dir=./config --migrations-dir=./migrations` 执行 `000063`、`000064`，并以新增 `000065` 补商家订单游标索引；事后复查版本 `65 dirty=false`。已执行过的迁移不原地修改；下次变更须新增版本。
- `SERVER_ENV=debug MLC_COMMERCE_INTEGRATION=1 go test -tags production ./internal/modules/commerce -run '^TestHGCommerceDebugLifecycle$' -count=1 -v`：使用随机买家、商家、类目、媒体/地址元数据；HTTP handler 实测草稿、提交、审批、上架、公开详情；同一 SKU 上 100 个不同请求争抢显式盘点的 10 件库存，成功 10、失败 90；重复取消和过期批次不重复释放；待支付地址越权和未付款发货均拒绝。夹具按随机业务 ID 清理，没有使用已有用户或真实支付渠道。

## 000065 阶段历史边界（以下状态已由 000066 增量更新）

- `commerce_inventory` 在审核通过时只登记 **零可售** SKU 库存；商品草稿 `initial_stock` 保留为建议值。测试显式调用盘点接口加库存，仅用于可控夹具。
- `HGOrderStore` 已实现服务端报价、原子库存预占、统一 `trade_order`/明细与独立 CNY 订单/预占事实、幂等重放、取消/有界过期释放、买家归属查询、已付本店订单读取与发货检查。**000065 阶段**未注册创建订单、付款、盘点、发货接口，也未启动过期 worker；最新边界以文末 000066 增量为准。
- 支付宝/微信未提供协议、回调验签/退款资料及隔离账号；没有创建支付尝试、伪造已付款、执行发货或将沙箱支付接到生产订单。商家发货要求 `trade_order` 与真正外部 `payment_order` 同时具备成功事实；目前无生产渠道可生成该事实。

## 上线前阻断项

1. 取得渠道非敏感协议与隔离测试条件，明确渠道订单号、签名、退款及迟到回调语义；实现支付入账、库存 RESERVED→SOLD、异常实收退款/对账，并用相同订单锁序验证竞争。
2. 确认仓库真实库存来源与盘点审计/权限，实现正式有界库存调整接口及权限规则；按业务需要补充运费、售后、订单金额和收件地址隐私策略。
3. 在开启真实下单前配置可停止的有界过期 worker/运维补偿、回调/outbox、告警和容量保护；对亿级订单和热点 SKU 做 EXPLAIN 与实测。000066 阶段已在 debug 显式开关下增加自动 worker，但仍缺少正式多实例补偿、告警和容量证据。
4. 使用专门的隔离测试账号完成真实浏览器、JWT/API Guard、媒体对象存储及商品/订单页面联调；当前集成用例调用实际 handler 和 debug MySQL，但不等于浏览器或真实渠道联调。

## 后续增量：debug 受控订单（000066）

- 本机 debug 经再次确认目标 `127.0.0.1:3306`，迁移前版本 `65 dirty=false`；获本轮明确授权执行库存盘点审计迁移 `000066`，复查 `66 dirty=false`。不对 pre/prod 迁移。
- `SERVER_ENV=debug`、加载配置环境 `debug`、显式 `MLC_COMMERCE_DEBUG_TRADING=true` **三条件同时满足**才允许 HTTP 盘点、预占、取消及订单查询；关闭开关或运行在 pre/prod 时，交易路径 fail-closed，不查未迁移的交易表。旧 CNY 沙箱开关不影响本模块。启用时注册可停止的每5秒/批20单、4秒超时的过期 worker；进程退出时在关闭 MySQL 前等待结束。启用开关前须先完成 `000066`，启动时校验迁移版本和 dirty 状态，未满足则拒绝启动交易模块。
- 已注册 `GET /api/v1/commerce/merchant/inventory`、`POST /merchant/inventory/adjust`、`POST /orders/preview`、`POST /orders`、`GET /orders`、`GET /orders/detail`、`POST /orders/cancel`。盘点仅能调整本人获批店铺的公开 SKU，需乐观锁版本和非空原因，更新与 `commerce_inventory_adjustment` 审计同事务；测试直接调用实际 HTTP handler + 隔离 MySQL。订单由真实服务端计算价格并校验本人地址，只能处于待支付或取消状态。
- 已注册 `POST /orders/payment` 与 `POST /merchant/orders/ship` 的 **不可用响应**，两者在 debug、pre、prod 均返回 503，不创建支付事实、不发货。授权设计 `SIMULATED` 独立 debug 渠道**不等于渠道协议、验签和退款已实现**；目前没有支付宝/微信隔离测试资料，无法声称 debug 完成付款-退款-发货闭环，也不能断言 pre/prod “只接渠道即可正式使用”。正式启用前必须分别验证渠道验签入账、退款/迟到回调对账、订单锁竞争、审计、浏览器 JWT/API Guard 和多实例容量。
