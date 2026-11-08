-- SIMULATED 仅为 debug 隔离账本；不表示支付宝/微信支付成功。回调事件先验签再入库，同订单行锁处理付款/关闭竞争。
ALTER TABLE commerce_order DROP CHECK chk_commerce_order_status;
ALTER TABLE commerce_order ADD CONSTRAINT chk_commerce_order_status CHECK (status BETWEEN 1 AND 6);
CREATE TABLE commerce_simulated_payment_event (
 event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '模拟渠道事件唯一标识',
 payment_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '本次模拟支付尝试 ID',
 trade_order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '生产 CNY 订单 ID',
 external_payment_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '仅 debug 模拟交易号，不得当成真实渠道流水',
 amount_cent BIGINT NOT NULL COMMENT '事件声明的模拟收款人民币分数',
 payload_hash BINARY(32) NOT NULL COMMENT '已验签原始请求正文摘要，检测同事件异内容',
 result VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'PAID、REFUNDED_LATE、REFUNDED_EXPIRED',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '模拟事件持久化时间，UTC毫秒',
 PRIMARY KEY (event_id), UNIQUE KEY uk_commerce_sim_payment_external (external_payment_id),
 KEY idx_commerce_sim_payment_order (trade_order_id,created_at,event_id),
 CONSTRAINT chk_commerce_sim_amount CHECK (amount_cent>0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='隔离 debug 模拟收款与自动退款事件台账，绝非真实支付证明';
