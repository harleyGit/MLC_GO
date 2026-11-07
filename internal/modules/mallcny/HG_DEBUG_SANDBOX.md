# 独立人民币商城 debug 沙箱

- 仅供本机隔离 `debug` 数据库验证；金额为人民币**分**，`hg_mall_cny_*` 与既有 M币商品、订单、资产完全隔离。不连接支付宝，也不发生真实资金划转。
- 启动前在目标 `debug` 库确认 `schema_migrations` 无 dirty 且版本为 62，并准备已审核的 `mall_merchant`。迁移入口为 `go run ./cmd/hg_config_check --env=debug --migrations-dir ./migrations`；它会执行目标库中尚未应用的全部增量迁移，**禁止**在未知版本或非隔离库上直接运行。`000062` 已在本机隔离 debug 库执行，运行前须再次核对连接目标。
- 只有同时满足进程 `SERVER_ENV=debug`、配置已加载为 debug、进程 `MLC_MALL_CNY_SANDBOX=true` 才启用读写接口与后台过期退款任务。其他环境返回 403，不访问缺失的 CNY 表。启动主服务仍需独立确认本机基础设施与后台任务的副作用。
- Web 登录后访问 `/mall/cny`，按后端实时报价预览、领券、开团/参团、提交幂等订单、模拟付款。支付流水 `alipay_sandbox` 是内部模拟记录，不是支付宝交易号；单买可模拟全额退款，拼团仅在未成团到期后退款。退款不恢复已核销的优惠券；取消未支付订单会释放券与预占库存。
- 隔离库验证：`SERVER_ENV=debug MLC_MALL_CNY_INTEGRATION=1 go test ./internal/modules/mallcny -count=1`。集成用例会写本机 debug 库并按唯一业务 ID 清理自身数据；请勿对共享或生产库启用该标志。
- 此链路不提供真实支付回调、发货/售后、地址簿、图片归属绑定或生产资金对账；不可把沙箱结果宣称为真实人民币收付或直接在生产开放。
