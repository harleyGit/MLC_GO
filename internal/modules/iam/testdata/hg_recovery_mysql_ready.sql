-- 人为推进仅此测试账号的冷却期，不读写真实业务库，也不等待一分钟。
UPDATE user_mfa_recovery SET available_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE user_id='fixture-user';
