-- 账户权威切换采用原地改名，保留原表主键、索引、数据及自增序列。
-- 不执行历史数据复制、回填或删除；切换前后须停止旧版资金写入方。
-- 需 MySQL 支持 INSTANT 加列；不支持时直接失败，禁止降级 COPY 或扫描回填。
-- DDL 分步自动提交，失败须核查已完成步骤后人工恢复，不能直接重跑整份脚本。
-- down 保留新增列，因此 down 后再次 up 必须核查列定义并跳过已完成的加列步骤。
RENAME TABLE `user_coin_wallets` TO `account`,
             `coin_asset_transactions` TO `account_ledger`;

ALTER TABLE `account`
    ADD COLUMN `frozen_balance` BIGINT UNSIGNED NOT NULL DEFAULT 0
        COMMENT '冻结余额，单位为最小 MLC_COIN；当前版本不开放冻结业务',
    ADD COLUMN `currency` VARCHAR(16) NOT NULL DEFAULT 'MLC_COIN'
        COMMENT '账户币种；当前一个用户仅一个 MLC_COIN 账户，不支持多币种',
    ADD COLUMN `account_type` VARCHAR(24) NOT NULL DEFAULT 'user'
        COMMENT '账户类型；当前值为 user，供后续账户类型扩展',
    ADD COLUMN `version` BIGINT UNSIGNED NOT NULL DEFAULT 0
        COMMENT '余额版本号；每次余额更新递增，当前仍使用行锁而非乐观锁',
    ALGORITHM=INSTANT;

ALTER TABLE `account_ledger`
    ADD COLUMN `balance_before` DECIMAL(20,0)
        GENERATED ALWAYS AS (
            CAST(`balance_after` AS DECIMAL(20,0)) - CAST(`signed_delta` AS DECIMAL(20,0))
        ) VIRTUAL
        COMMENT '变更前余额；使用 DECIMAL 减 signed_delta，避免 unsigned 减法溢出，允许历史记录',
    ALGORITHM=INPLACE, LOCK=NONE;
