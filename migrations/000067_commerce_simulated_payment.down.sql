-- 仅限无模拟支付/退款事实的隔离环境回退；否则订单状态约束会阻止删除。
DROP TABLE commerce_simulated_payment_event;
ALTER TABLE commerce_order DROP CHECK chk_commerce_order_status;
ALTER TABLE commerce_order ADD CONSTRAINT chk_commerce_order_status CHECK (status BETWEEN 1 AND 5);
