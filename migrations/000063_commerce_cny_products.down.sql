-- 仅供隔离环境在确认无商品、无外部引用时按迁移流程回退；生产不能直接执行数据清理。
DROP TABLE commerce_publish_request;
DROP TABLE commerce_media;
DROP TABLE commerce_sku;
DROP TABLE commerce_product_version;
DROP TABLE commerce_product;
DROP TABLE commerce_category;
