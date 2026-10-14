-- 仅扩展结构；历史数据必须停写后通过有界手动工具搬迁，不能自动回填。
ALTER TABLE payment_recharge_order
    ADD COLUMN display_name VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'wallet用户展示名快照；非wallet来源默认为空',
    ADD COLUMN description VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'wallet充值说明快照；非wallet来源默认为空',
    ADD COLUMN payment_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'unavailable' COMMENT '创建时绑定渠道：unavailable或platform_debug；不表示真实支付成功',
    ADD COLUMN paid_transaction_id BIGINT UNSIGNED NULL COMMENT 'debug入账的coin权威流水ID；未入账为空',
    ADD COLUMN balance_after BIGINT UNSIGNED NULL COMMENT 'debug入账后的平台币余额快照；未入账为空',
    ADD COLUMN snapshot_source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'native' COMMENT '快照来源：native原统一表，wallet新钱包单，wallet_legacy旧钱包迁移单',
    DROP CHECK ck_payment_recharge_order_coin_equation,
    MODIFY COLUMN coin_amount BIGINT NULL COMMENT '基础平台币数量；仅wallet_legacy历史缺失时为空，不得用当前SKU补造',
    MODIFY COLUMN bonus_coin BIGINT NULL DEFAULT 0 COMMENT '赠送平台币数量；仅wallet_legacy历史缺失时为空，不得用当前SKU补造',
    ADD CONSTRAINT ck_payment_recharge_order_coin_equation CHECK (
        (snapshot_source = 'wallet_legacy' AND coin_amount IS NULL AND bonus_coin IS NULL)
        OR (snapshot_source <> 'wallet_legacy' AND coin_amount IS NOT NULL AND bonus_coin IS NOT NULL
            AND CAST(total_coin AS DECIMAL(30,0)) = CAST(coin_amount AS DECIMAL(30,0)) + CAST(bonus_coin AS DECIMAL(30,0)))
    );
