-- 新增六类独立权威业务实体，ID 与 iam_scope_resources.scope_id 对齐，但不从 IAM 占位资源自动生成业务对象。
-- 旧稿件/任务/商家没有可信归属时不回填；后续由业务创建事务写入实体和 IAM 登记，关联迁移另行按主键分批实施。
-- 所有列表由状态+主键游标读取；任务/稿件/商家关联按业务唯一 ID 点查，避免大表无索引扫描。
CREATE TABLE hg_creator_spaces (
  scope_id VARCHAR(64) NOT NULL COMMENT '创作者空间业务 ID，与 creator 类型 IAM 资源 ID 相同',
  owner_user_id VARCHAR(64) NOT NULL COMMENT '创建时经用户权威表复核的空间所有者用户业务 ID，不等于管理者角色',
  name VARCHAR(128) NOT NULL COMMENT '空间显示名称，不作为权限或唯一性依据',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用，0 停用；停用后不允许新投稿或授予空间角色',
  created_by VARCHAR(64) NOT NULL COMMENT '建立该空间的操作用户业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后状态或名称变更时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(scope_id), KEY idx_hg_creator_owner_status(owner_user_id,status,scope_id),
  CONSTRAINT chk_hg_creator_space_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='创作者空间权威实体，不自动继承投稿用户 IAM 权限';

CREATE TABLE hg_business_organizations (
  scope_id VARCHAR(64) NOT NULL COMMENT '新组织业务 ID，不复用旧 iam_organizations.org 标识',
  name VARCHAR(128) NOT NULL COMMENT '组织显示名称，不作为租户 ID',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用，0 停用；停用后成员权限不生效',
  created_by VARCHAR(64) NOT NULL COMMENT '建立组织的操作用户业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后状态或名称变更时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(scope_id), KEY idx_hg_organization_status(status,scope_id),
  CONSTRAINT chk_hg_business_organization_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='企业或运营组织权威实体，与旧 org 隔离';

CREATE TABLE hg_organization_members (
  organization_id VARCHAR(64) NOT NULL COMMENT '新组织业务 ID，所属组织须启用',
  user_id VARCHAR(64) NOT NULL COMMENT '经身份服务确认的成员用户业务 ID',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 有效，0 移除；成员身份不自动授予 IAM 角色',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '加入时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(organization_id,user_id), KEY idx_hg_org_member_user(user_id,status,organization_id),
  CONSTRAINT chk_hg_org_member_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='组织成员关系，必须与精确作用域 IAM 授权共同校验';

CREATE TABLE hg_operation_environments (
  scope_id VARCHAR(64) NOT NULL COMMENT '运维环境业务 ID，与 environment 类型 IAM 资源 ID 相同',
  environment_code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '环境权威代号，不存凭据、不据客户端代号确定部署目标',
  name VARCHAR(128) NOT NULL COMMENT '环境显示名称',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用，0 停用；真实控制目标仍需服务端绑定核验',
  created_by VARCHAR(64) NOT NULL COMMENT '建立环境目录的管理员用户业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后状态或名称变更时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(scope_id), UNIQUE KEY uk_hg_environment_code(environment_code),
  KEY idx_hg_environment_status(status,scope_id),
  CONSTRAINT chk_hg_environment_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运维环境目录，不替代部署身份与生产审批';

CREATE TABLE hg_crawler_projects (
  scope_id VARCHAR(64) NOT NULL COMMENT '爬虫项目业务 ID，与 crawler_project 类型 IAM 资源 ID 相同',
  organization_id VARCHAR(64) DEFAULT NULL COMMENT '可空的权威组织业务 ID，空表示平台直属项目',
  name VARCHAR(128) NOT NULL COMMENT '项目显示名称',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用，0 停用；调度与手动执行均须复核',
  created_by VARCHAR(64) NOT NULL COMMENT '建立项目的操作用户业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后状态或名称变更时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(scope_id), KEY idx_hg_project_org(organization_id,status,scope_id),
  KEY idx_hg_project_status(status,scope_id),
  CONSTRAINT chk_hg_crawler_project_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='爬虫项目权威实体，不等于单条任务定义';

CREATE TABLE hg_content_channels (
  scope_id VARCHAR(64) NOT NULL COMMENT '内容频道业务 ID，与 content_channel 类型 IAM 资源 ID 相同',
  organization_id VARCHAR(64) DEFAULT NULL COMMENT '可空的权威组织业务 ID，空表示平台频道',
  name VARCHAR(128) NOT NULL COMMENT '频道显示名称，不等于稿件 category 字符串',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用，0 停用；停用后不能发布到频道',
  created_by VARCHAR(64) NOT NULL COMMENT '建立频道的操作用户业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后状态或名称变更时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(scope_id), KEY idx_hg_channel_org(organization_id,status,scope_id),
  KEY idx_hg_channel_status(status,scope_id),
  CONSTRAINT chk_hg_content_channel_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='内容频道权威实体，不按标签字符串授权';

CREATE TABLE hg_business_regions (
  scope_id VARCHAR(64) NOT NULL COMMENT '运营区域业务 ID，与 region 类型 IAM 资源 ID 相同',
  region_code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '稳定区域编码，不使用用户收货地址作为权限来源',
  name VARCHAR(128) NOT NULL COMMENT '区域显示名称',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用，0 停用；停用后区域角色不生效',
  created_by VARCHAR(64) NOT NULL COMMENT '建立区域的操作用户业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后状态或名称变更时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(scope_id), UNIQUE KEY uk_hg_region_code(region_code),
  KEY idx_hg_region_status(status,scope_id),
  CONSTRAINT chk_hg_business_region_status CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='业务区域权威实体，独立于买家地址';

-- 下列关联仅供业务写入时同步维护；绝不基于旧数据文本列自动猜测归属。
CREATE TABLE hg_crawler_task_projects (
  task_id BIGINT UNSIGNED NOT NULL COMMENT 'crawler_task_definitions.id，一个任务至多归属一个项目',
  project_id VARCHAR(64) NOT NULL COMMENT '项目业务 ID，执行及列表须点查项目状态和权限',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '关联建立时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(task_id), KEY idx_hg_project_task(project_id,task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='爬虫任务与项目的权威关联，无关联旧任务保留原有权限路径';

CREATE TABLE hg_submission_scope_links (
  submission_id VARCHAR(64) NOT NULL COMMENT 'video_submissions.submission_id，一份稿件一个空间和频道',
  creator_id VARCHAR(64) NOT NULL COMMENT '创作者空间业务 ID',
  channel_id VARCHAR(64) NOT NULL COMMENT '内容频道业务 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '关联建立时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(submission_id), KEY idx_hg_submission_creator(creator_id,submission_id),
  KEY idx_hg_submission_channel(channel_id,submission_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='稿件归属空间与频道的权威关联，不自动回填旧稿件';

CREATE TABLE hg_merchant_regions (
  merchant_id VARCHAR(64) NOT NULL COMMENT 'mall_merchant.merchant_id，商家业务 ID',
  region_id VARCHAR(64) NOT NULL COMMENT '区域业务 ID；多区域商家须逐区域独立校验',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '关联建立时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(merchant_id,region_id), KEY idx_hg_region_merchant(region_id,merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商家运营区域的显式关联，不能从收货地址或客户端地区推断';
