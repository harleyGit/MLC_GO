-- 新媒体的业务引用必须与业务主表在同一 MySQL 实例/事务中写入；不得通过无业务资格验证的通用 HTTP 接口建立引用。
CREATE TABLE media_asset_refs (
  ref_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '业务引用主键',
  asset_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '经审核 ready 的媒体资源 ID',
  owner_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '上传者受信 JWT ID；商家或会话资格仍由业务服务检查',
  biz_type VARCHAR(32) NOT NULL COMMENT '业务域：user/product/shop/chat/video/comment/support/marketing',
  biz_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '权威业务主键，不信任前端单独声明',
  slot VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '稳定引用槽，如 avatar/main/detail_0001',
  created_at DATETIME(3) NOT NULL COMMENT '引用创建时间，UTC 毫秒',
  PRIMARY KEY (ref_id),
  UNIQUE KEY uk_media_business_slot (biz_type,biz_id,slot),
  KEY idx_media_ref_asset (asset_id,ref_id)
) ENGINE=InnoDB;
