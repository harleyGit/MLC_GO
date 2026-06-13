-- ============================================================
-- 课程平台数据库迁移脚本
-- 版本: 000006
-- 描述: 创建课程平台核心业务表结构
-- 包含模块: 权限体系、用户体系、课程商品、订单、短信、云存储
-- ============================================================

-- ============================================================
-- 权限体系模块 (Permission System)
-- 功能: 管理后台权限控制，包括菜单权限和操作权限
-- ============================================================

-- 权限清单表
-- 存储系统所有权限定义，支持菜单和操作两种类型
CREATE TABLE IF NOT EXISTS `permission` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '权限ID，主键自增',
  `code` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '权限编码，唯一标识一个权限',
  `type` tinyint NOT NULL COMMENT '权限类型: 1=菜单权限 2=操作权限',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '权限名称，用于显示',
  `page_path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '菜单路径，仅菜单权限有效',
  `parent_id` bigint NOT NULL DEFAULT '-1' COMMENT '父级权限ID，-1表示顶级权限',
  `status` tinyint NOT NULL COMMENT '状态: 1=正常 -1=禁用',
  `sort` int NOT NULL DEFAULT '1' COMMENT '排序值，值越小越靠前',
  `desc` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '权限描述说明',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_at` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间，自动更新',
  `update_by` bigint NOT NULL DEFAULT '0' COMMENT '最后更新人ID',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `idx_code` (`code`,`status`) USING BTREE COMMENT '权限编码唯一索引',
  KEY `idx_name` (`name`) USING BTREE COMMENT '权限名称索引',
  KEY `idx_parent_id` (`parent_id`) USING BTREE COMMENT '父级ID索引，用于树形查询'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='权限清单表 - 存储系统权限定义';

-- 管理员表
-- 存储后台管理员账号信息
CREATE TABLE IF NOT EXISTS `admin_user` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '管理员ID，主键自增',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '管理员姓名',
  `nick_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '管理员昵称',
  `email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '邮箱，用于搜索和通知',
  `mobile` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '手机号，用于登录',
  `lark_open_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '飞书OpenID，用于飞书登录',
  `password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '登录密码，加密存储',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1=正常 -1=禁用',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_at` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `create_by` bigint NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `update_by` bigint NOT NULL DEFAULT '1' COMMENT '更新人ID',
  `sex` tinyint NOT NULL DEFAULT '3' COMMENT '性别: 1=男 2=女 3=其他',
  `is_delete` tinyint NOT NULL DEFAULT '0' COMMENT '软删除标记: 0=正常 1=已删除',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `idx_email` (`email`) USING BTREE COMMENT '邮箱唯一索引',
  UNIQUE KEY `idx_mobile` (`mobile`) USING BTREE COMMENT '手机号唯一索引',
  KEY `idx_name` (`name`) USING BTREE COMMENT '姓名索引',
  KEY `idx_nick_name` (`nick_name`) USING BTREE COMMENT '昵称索引，用于后台管理员搜索'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='管理员表 - 存储后台管理员信息';

-- 角色表
-- 存储后台角色定义，供 admin_user_role 和 role_permission 关联使用
CREATE TABLE IF NOT EXISTS `role` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '角色ID，主键自增',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '角色名称',
  `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '角色描述',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1=正常 -1=禁用',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `create_by` bigint NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `update_by` bigint NOT NULL DEFAULT '0' COMMENT '更新人ID',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uidx_name` (`name`) USING BTREE COMMENT '角色名称唯一索引',
  KEY `idx_status_id` (`status`,`id`) USING BTREE COMMENT '角色列表分页索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='角色表 - 存储后台角色定义';

-- 角色权限关联表
-- 存储角色与权限的对应关系
CREATE TABLE IF NOT EXISTS `role_permission` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `role_id` bigint NOT NULL COMMENT '角色ID',
  `permission_id` bigint NOT NULL COMMENT '权限ID',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `create_by` bigint NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `update_by` bigint NOT NULL DEFAULT '0' COMMENT '更新人ID',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `idx_role_id` (`role_id`) USING BTREE COMMENT '角色ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='角色权限关联表 - 角色与权限的多对多关系';

-- 管理员角色关联表
-- 存储管理员与角色的对应关系
CREATE TABLE IF NOT EXISTS `admin_user_role` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `admin_user_id` bigint NOT NULL COMMENT '管理员ID',
  `role_id` bigint NOT NULL COMMENT '角色ID',
  `update_at` datetime NOT NULL COMMENT '更新时间',
  `update_by` bigint NOT NULL COMMENT '更新人ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uidx_aduid_role_id` (`admin_user_id`,`role_id`) COMMENT '管理员角色唯一索引',
  KEY `idx_role_id` (`role_id`) COMMENT '角色ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='管理员角色关联表 - 管理员与角色的多对多关系';

-- ============================================================
-- 用户体系模块 (User System)
-- 功能: 管理前台用户，支持微信登录
-- ============================================================

-- 用户主表
-- 存储用户基本信息，作为用户中心的核心表
CREATE TABLE IF NOT EXISTS `user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '用户ID，全局唯一',
  `nick_name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户昵称',
  `sex` tinyint NOT NULL COMMENT '性别: 0=其他 1=男 2=女',
  `password` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '登录密码，加密存储',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1=正常 -1=禁用',
  `icon_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '头像资源key',
  `create_at` datetime NOT NULL COMMENT '注册时间',
  `last_login_at` datetime DEFAULT NULL COMMENT '最后登录时间',
  `update_at` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `idx_name` (`nick_name`) COMMENT '昵称索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='用户主表 - 存储用户基本信息';

-- 微信用户表
-- 存储微信用户信息，与user表关联
CREATE TABLE IF NOT EXISTS `wechat_user` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID，自增',
  `user_id` bigint NOT NULL COMMENT '关联user表的用户ID',
  `union_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '微信union_id，跨应用唯一标识',
  `nick_name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '微信昵称',
  `icon_url` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '微信头像URL',
  `create_at` datetime NOT NULL COMMENT '创建时间',
  `update_at` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uidx_user` (`user_id`) USING BTREE COMMENT '用户ID唯一索引',
  UNIQUE KEY `uidx_union` (`union_id`) USING BTREE COMMENT '微信union_id唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='微信用户表 - 存储微信用户信息';

-- 应用用户表
-- 存储用户在不同应用(公众号、小程序)的身份信息
CREATE TABLE IF NOT EXISTS `app_user` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID，自增',
  `user_id` bigint NOT NULL COMMENT '关联user表的用户ID',
  `app_code` int NOT NULL COMMENT '应用编码: 1000=公众号 1001=小程序',
  `open_id` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '微信应用内openid',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1=正常 -1=禁用',
  `create_at` datetime NOT NULL COMMENT '创建时间',
  `update_at` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uidx_user_appcode` (`user_id`,`app_code`) COMMENT '用户应用唯一索引',
  UNIQUE KEY `uidx_openId` (`open_id`) COMMENT 'openid唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='应用用户表 - 存储用户在不同应用的身份';

-- ============================================================
-- 课程商品模块 (Course Goods)
-- 功能: 管理课程商品、目录、课时
-- ============================================================

-- 课程商品表
-- 存储课程商品信息，包括价格、服务时长等
CREATE TABLE IF NOT EXISTS `course_goods` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '商品ID，主键自增',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '商品名称',
  `cover_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '封面图资源key',
  `intro_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '介绍图资源key',
  `desc` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '商品描述',
  `goods_price` bigint NOT NULL DEFAULT '0' COMMENT '商品价格，单位：分',
  `service_time` tinyint NOT NULL COMMENT '辅导服务时长: 1=一个月 2=三个月 3=半年 4=一年',
  `sale_type` tinyint NOT NULL COMMENT '销售类型: 1=免费 2=收费',
  `status` tinyint NOT NULL DEFAULT '-1' COMMENT '状态: -1=下架 1=上架',
  `create_at` datetime NOT NULL COMMENT '创建时间',
  `create_by` bigint NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `update_at` datetime NOT NULL COMMENT '更新时间',
  `update_by` bigint NOT NULL DEFAULT '0' COMMENT '更新人ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uidx_name` (`name`) COMMENT '商品名称唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='课程商品表 - 存储课程商品信息';

-- 课程目录表
-- 存储课程的章节目录结构
CREATE TABLE IF NOT EXISTS `course_catalog` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '目录ID，主键自增',
  `parent_id` bigint NOT NULL DEFAULT '-1' COMMENT '父目录ID，-1表示顶级目录',
  `level` int NOT NULL DEFAULT '1' COMMENT '目录层级',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '目录名称',
  `good_id` bigint NOT NULL COMMENT '所属商品ID',
  `sort` bigint NOT NULL COMMENT '排序值',
  `update_at` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `update_by` bigint NOT NULL COMMENT '更新人ID',
  PRIMARY KEY (`id`),
  KEY `idx_good_id` (`good_id`) COMMENT '商品ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='课程目录表 - 存储课程章节目录';

-- 课程课时表
-- 存储具体课时信息，包括视频、详情、作业等
CREATE TABLE IF NOT EXISTS `course_lessons` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '课时ID，主键自增',
  `goods_id` bigint NOT NULL COMMENT '所属商品ID',
  `catalog_id` bigint NOT NULL COMMENT '所属目录ID',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '课时名称',
  `enable_trial` tinyint NOT NULL DEFAULT '0' COMMENT '是否可试听: 1=可试听 其他=不可试听',
  `status` tinyint NOT NULL COMMENT '状态: 1=启用 -1=禁用',
  `video_key` varchar(255) COLLATE utf8mb4_general_ci NOT NULL COMMENT '视频资源key',
  `detail` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '课时详情内容',
  `homework` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '课后练习内容',
  `sort` tinyint NOT NULL DEFAULT '0' COMMENT '排序值',
  `update_at` datetime NOT NULL COMMENT '更新时间',
  `update_by` bigint NOT NULL COMMENT '更新人ID',
  PRIMARY KEY (`id`),
  KEY `idx_gid` (`goods_id`) COMMENT '商品ID索引',
  KEY `idx_ct_id` (`catalog_id`) COMMENT '目录ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='课程课时表 - 存储课时详细信息';

-- 用户课程商品关联表
-- 记录用户购买的课程商品及服务有效期
CREATE TABLE IF NOT EXISTS `user_course_goods` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `order_id` bigint NOT NULL COMMENT '订单ID',
  `goods_id` bigint NOT NULL COMMENT '商品ID',
  `goods_type` tinyint NOT NULL COMMENT '商品类型: 1=课程商品',
  `buy_time` bigint NOT NULL COMMENT '购买时间戳',
  `service_expire_time` bigint NOT NULL COMMENT '服务到期时间戳',
  PRIMARY KEY (`id`),
  KEY `idx_uid` (`user_id`) COMMENT '用户ID索引',
  KEY `idx_cid` (`goods_id`) COMMENT '商品ID索引',
  KEY `idx_se_ex_t` (`service_expire_time`) USING BTREE COMMENT '服务到期时间索引，用于到期查询'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户课程商品关联表 - 记录用户购买记录';

-- ============================================================
-- 订单模块 (Order System)
-- 功能: 管理订单生命周期，支持多种订单来源
-- ============================================================

-- 订单主表
-- 存储订单核心信息，包括金额、状态、支付等
CREATE TABLE IF NOT EXISTS `orders` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '订单ID，主键自增',
  `order_no` char(18) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '订单编号，18位唯一编号',
  `user_id` bigint NOT NULL COMMENT '下单用户ID',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '订单状态: -1=已取消 1=待支付 2=已支付 3=已完成',
  `order_source` tinyint NOT NULL COMMENT '订单来源: 1=用户下单 2=管理后台 3=系统赠送',
  `order_amount` bigint NOT NULL COMMENT '订单金额，单位：分',
  `order_origin_amount` bigint NOT NULL COMMENT '商品原价，单位：分',
  `payment_amount` bigint NOT NULL COMMENT '实际支付金额，单位：分',
  `trade_no` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '第三方支付单号',
  `inner_trade_no` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '内部交易单号',
  `order_desc` mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '订单描述信息',
  `payment_at` bigint NOT NULL DEFAULT '0' COMMENT '支付时间戳',
  `user_remark` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户备注',
  `receiver_confirm_at` bigint DEFAULT NULL COMMENT '确认收货时间戳',
  `receiver_confirm_type` tinyint DEFAULT NULL COMMENT '收货方式',
  `refund_amount` bigint NOT NULL DEFAULT '0' COMMENT '退款金额，单位：分',
  `refund_at` bigint DEFAULT NULL COMMENT '退款时间戳',
  `cancel_at` bigint DEFAULT NULL COMMENT '取消时间戳',
  `cancel_type` tinyint DEFAULT NULL COMMENT '取消类型',
  `cancel_by` bigint DEFAULT NULL COMMENT '取消人ID',
  `cancel_reason` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '取消原因',
  `create_at` bigint NOT NULL COMMENT '创建时间戳',
  `create_by` bigint NOT NULL COMMENT '创建人ID',
  `transfer_at` bigint NOT NULL DEFAULT '0' COMMENT '转账时间戳',
  `transfer_by` bigint NOT NULL DEFAULT '0' COMMENT '转账操作人ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_odno` (`order_no`) USING BTREE COMMENT '订单编号唯一索引',
  KEY `idx_uid` (`user_id`) USING BTREE COMMENT '用户ID索引',
  KEY `idx_create` (`create_at`) USING BTREE COMMENT '创建时间索引',
  KEY `idx_trade_no` (`trade_no`) USING BTREE COMMENT '第三方支付单号索引',
  KEY `idx_pay_at` (`payment_at`) USING BTREE COMMENT '支付时间索引',
  KEY `idx_osrc` (`order_source`) USING BTREE COMMENT '订单来源索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='订单主表 - 存储订单核心信息';

-- 订单商品明细表
-- 存储订单包含的商品信息
CREATE TABLE IF NOT EXISTS `order_items` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `order_id` bigint NOT NULL COMMENT '订单ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `goods_id` bigint NOT NULL COMMENT '商品ID',
  `goods_type` tinyint NOT NULL DEFAULT '1' COMMENT '商品类型: 1=课程商品',
  `quantity` int NOT NULL DEFAULT '1' COMMENT '商品数量',
  `goods_snap` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '商品快照信息',
  PRIMARY KEY (`id`),
  KEY `idx_oid` (`order_id`) COMMENT '订单ID索引',
  KEY `idx_gid` (`goods_id`) COMMENT '商品ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='订单商品明细表 - 存储订单商品信息';

-- ============================================================
-- 短信模块 (SMS System)
-- 功能: 管理短信模板，支持多短信平台
-- ============================================================

-- 短信模板表
-- 存储短信模板配置，支持不同场景和平台
CREATE TABLE IF NOT EXISTS `sms_template` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '模板ID，主键自增',
  `scene_code` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '场景编码，唯一标识',
  `sign_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '短信签名',
  `platform_tmpl_id` int NOT NULL COMMENT '第三方短信平台模板ID',
  `tmpl_str` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '短信模板内容',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1=正常 -1=禁用',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `admin_user_id` bigint NOT NULL DEFAULT '0' COMMENT '管理员ID',
  `platform` char(8) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '短信平台标识',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uidx_scene` (`scene_code`) USING BTREE COMMENT '场景编码唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='短信模板表 - 存储短信模板配置';

-- ============================================================
-- 云存储资源模块 (Cloud Storage)
-- 功能: 管理上传到云存储的文件资源
-- ============================================================

-- 云存储资源文件表
-- 记录上传到云存储的文件信息
CREATE TABLE IF NOT EXISTS `resource_upload_files` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `scene` char(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '业务场景编码',
  `file_key` varchar(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '文件云存储唯一key',
  `user_id` bigint NOT NULL COMMENT '上传用户ID',
  `user_type` bigint NOT NULL COMMENT '用户类型: 1=普通用户 2=后台管理员',
  `file_type` char(8) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '文件类型: img/video/doc',
  `file_size` bigint NOT NULL COMMENT '文件大小，单位：字节',
  `file_name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '文件原始名称',
  `upload_client_ip` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '上传客户端IP地址',
  `create_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '上传时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uidx_key` (`file_key`) USING BTREE COMMENT '文件key唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='云存储资源文件表 - 记录上传文件信息';
