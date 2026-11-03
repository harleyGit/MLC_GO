-- 仅新增关系表，不迁移现有会话数据；生产大表上线前需评估索引与回滚窗口。
CREATE TABLE im_friend_request (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '申请主键，游标分页使用',
    request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '公开申请业务 ID，UUID',
    requester_id VARCHAR(128) NOT NULL COMMENT '申请人 JWT 用户 ID',
    addressee_id VARCHAR(128) NOT NULL COMMENT '被申请人 JWT 用户 ID',
    message VARCHAR(128) NOT NULL DEFAULT '' COMMENT '好友申请附言，最多 128 字符',
    status TINYINT NOT NULL DEFAULT 0 COMMENT '申请状态：0 待处理，1 接受，2 拒绝',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '申请创建时间，毫秒精度',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态更新时间，毫秒精度',
    PRIMARY KEY (id), UNIQUE KEY uk_im_request_business (request_id),
    KEY idx_im_request_incoming (addressee_id,status,id),
    KEY idx_im_request_outgoing (requester_id,addressee_id,status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友申请历史；只有被申请人可以处理待处理申请';

CREATE TABLE im_friendship (
    user_low_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '按字节序较小的 JWT 用户 ID',
    user_high_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '按字节序较大的 JWT 用户 ID',
    accepted_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '双方同意成为好友的时间，毫秒精度',
    PRIMARY KEY (user_low_id,user_high_id),
    KEY idx_im_friend_high (user_high_id,user_low_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='双方同意后的当前有效好友关系';

CREATE TABLE im_friend_remark (
    owner_user_id VARCHAR(128) NOT NULL COMMENT '备注所有人 JWT 用户 ID，仅本人可读写',
    friend_user_id VARCHAR(128) NOT NULL COMMENT '被备注好友 JWT 用户 ID',
    remark VARCHAR(64) NOT NULL DEFAULT '' COMMENT '仅本人可见的备注名，最多 64 字符',
    tags JSON NOT NULL COMMENT '仅本人可见的标签数组，最多六项',
    phone VARCHAR(32) NOT NULL DEFAULT '' COMMENT '仅本人可见的联系号码备忘，不是账号验证手机号',
    description VARCHAR(128) NOT NULL DEFAULT '' COMMENT '仅本人可见的联系人描述',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '备注更新时间，毫秒精度',
    PRIMARY KEY (owner_user_id,friend_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='单向私有联系人备注';

CREATE TABLE im_user_block (
    blocker_user_id VARCHAR(128) NOT NULL COMMENT '主动拉黑用户 JWT ID',
    blocked_user_id VARCHAR(128) NOT NULL COMMENT '被拉黑用户 JWT ID',
    block_note VARCHAR(128) NOT NULL DEFAULT '' COMMENT '仅拉黑者可见的原因备忘',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '拉黑时间，毫秒精度',
    PRIMARY KEY (blocker_user_id,blocked_user_id),
    KEY idx_im_blocked (blocked_user_id,blocker_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='有方向的黑名单；任一方向拉黑禁止普通单聊';
