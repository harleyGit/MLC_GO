-- 仅供确认无订单与库存事实的隔离环境回退；生产不得直接清空交易记录。
DROP TABLE commerce_inventory_reservation;
DROP TABLE commerce_order;
DROP TABLE commerce_inventory;
