ALTER TABLE `outbox_events`
    ADD COLUMN `lease_token` VARCHAR(64) NULL DEFAULT NULL
        COMMENT '当前领取令牌；ack/retry 必须携带相同令牌，防止过期 worker 修改新租约'
        AFTER `next_retry_at`;
