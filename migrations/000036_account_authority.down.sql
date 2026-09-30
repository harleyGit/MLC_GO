-- 回退只恢复旧表名；不删除新增列，避免静默丢失冻结余额、币种、账户类型和版本数据。
-- 原表主键、索引、流水及 account_ledger.balance_before 数据均保留。
-- 回退前停止资金写入方；冻结尚未开放，若未来启用冻结，不得直接启动不识别冻结的旧程序。
-- 此脚本仅恢复表名而非恢复完整旧结构；再次 up 需跳过已保留的加列步骤。
-- 不是可直接 down/up 的可逆迁移：自动重新 up 会因重复列失败，禁止用删列解决。
-- 重新升级必须停写，核对 SHOW CREATE TABLE 的字段、默认值、生成表达式及原索引，
-- 再由发布负责人审核仅改名的恢复脚本及迁移版本状态；不得盲目清除 dirty 标记。
-- 降级前人工确认 frozen_balance 全部为零、币种仍为 MLC_COIN、类型仍为 user；
-- 若资金扩展已启用则禁止降级旧程序，保留现场并使用向前修复，不转换或丢弃冻结资金。
RENAME TABLE `account` TO `user_coin_wallets`,
             `account_ledger` TO `coin_asset_transactions`;
