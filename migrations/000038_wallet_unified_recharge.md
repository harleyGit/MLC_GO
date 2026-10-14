# 000038 统一充值写入切换说明

## 目标

`internal/modules/wallet/repository` 新写入只落 `payment_recharge_order`，并在同一 MySQL 短事务内创建对应的 `trade_order`。钱包 API 的 `orderId`、`requestId` 幂等、debug 开关、单笔 `100000` 上限、coin 入账及 `wallet_debug:<orderId>` 幂等语义保持不变。

debug 入账只更新本地交易和充值完成状态，不写 `payment_order`，不声明第三方支付成功；`payment_status` 保持 `INIT`。coin、充值单、交易单三者成功后才提交同一事务。

## 新增字段

`payment_recharge_order` 新增 `display_name`、`description`、`payment_mode`、`paid_transaction_id`、`balance_after`、`snapshot_source`。同时允许 `wallet_legacy` 行的 `coin_amount` 与 `bonus_coin` 为 NULL；`total_coin` 保留旧快照值，不用当前 SKU 反推历史拆分。

新单 `snapshot_source=wallet`，原生统一订单为 `native`，旧表迁移单为 `wallet_legacy`。`request_hash` 为无空白 JSON `{"skuId":"<sku>"}` 的 UTF-8 字节 SHA-256 小写十六进制，SKU区分大小写，创建和读取均校验。

统一表的 user_id 唯一键采用 general_ci，旧表采用 bin。迁移前必须检查用户标识大小写/重音等折叠碰撞及跨业务 request_id 碰撞；出现碰撞必须停下人工处理，不可覆盖或改号。owner读取保留二进制比较，重放也校验原始user_id。

提供 `cmd/hg_recharge_migrate` 一次性手动 CLI；它不启动后台任务、不执行 schema migration，复用 `LoadConfig` 和 `NewSQLManager` 的配置/连接校验。默认不执行，只有显式提供全部安全参数才会连接并写入。命令不会输出 DSN 或配置凭据。

精确命令（示例区间最多500个主键，执行前必须已获授权停止旧 wallet 写入和 debug 支付，并确认 038 已在目标环境完成）：

```bash
go run ./cmd/hg_recharge_migrate \
  --env pre \
  --config-dir ./config \
  --confirm-stopped-writes \
  --low 1 \
  --high 500 \
  --timeout 10s
```

仅检查参数、不执行迁移：

```bash
go run ./cmd/hg_recharge_migrate --help
```

`--env` 必须明确为 `debug`、`pre` 或 `prod`；`--confirm-stopped-writes`、`--low`、`--high` 均为必填安全条件。`high-low < 500`，空缺任一项、区间超过500、环境非法或 timeout 非正都会在数据库连接前失败。CLI 每次只处理一个区间，默认超时10秒，也可显式传 `--timeout`。

每批最多500行会有逐行点查/插入，应依据锁等待、连接池和复制延迟降低区间或暂停。生产索引及DDL耗时尚未实测。不要把命令挂到业务HTTP入口，不要自动循环区间，不要在未授权环境执行。

## 安全切换顺序

1. 备份并核对 `wallet_recharge_orders`、`payment_recharge_order`、`trade_order` 的主键、唯一键和状态计数；确认 038 已在目标环境完成并确认 DDL 锁表和复制延迟。
2. 停止旧 wallet 写入入口，禁止旧版本实例继续创建或 debug 支付；只允许读流量和迁移工具访问旧表。
3. 使用 `HGMigrateLegacyRange(ctx, low, high)` 按旧表主键闭区间迁移。要求 `low > 0`、`high >= low`、`high-low < 500`，每次最多扫描 500 行，一个区间一个事务，记录已提交区间和最后主键。
4. 对每个区间复核 `order_id/user_id/request_id/sku_id/title/description/currency/pay_amount/total_coin/status/paid_*`。任何重复键、交易单不一致、非法标识、未知状态或已存在快照不完全相同都停止，不得使用 `INSERT ... SELECT`，不得 `INSERT IGNORE`。
5. 旧已支付订单只迁移原有 `paid_transaction_id` 与 `balance_after` 快照，不再次调用 coin 入账；旧未支付订单只创建待支付交易事实。缺失的 `coin_amount/bonus_coin` 必须保持 NULL。
6. 迁移完成后核对旧表行数与统一表 `snapshot_source=wallet_legacy` 数量、`total_coin` 总和、已支付订单快照和交易状态，再切换新版本实例。
7. 新版本稳定后保留旧表只读和迁移审计窗口；确认无旧写入、无未迁移区间且有备份后，才由单独审批决定归档旧表。不得在本迁移中删除旧表。

提交结果未知时重跑同一区间：工具会按统一表快照逐行比对，完全一致则跳过，不一致则返回冲突。任何批次失败保留已提交前序区间，修复原因后从记录的下一个区间继续。

## 回退要求

不能通过回退 038 自动恢复旧写入：新表可能已有统一订单，旧表迁移行还可能缺少基础币/赠币拆分。回退必须停新版本、保留统一表和 coin 流水，完成经审批的双表差异核对后再决定是否恢复只读/兼容入口；不得把新订单直接反写旧表，也不得重复入账。
