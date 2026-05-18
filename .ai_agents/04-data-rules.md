# 04-data-rules.md

## 数据层
- 数据访问层写法保持一致
- 不擅改 SQL、ORM、查询封装方式
- 不擅改 repository / dao 职责边界
- 百万级并发或大流量写入链路必须主动检查热点行、热点索引、唯一键冲突、事务范围、连接池压力和批量写入策略

## SQL / ORM
- 查询风格保持一致
- 不擅改字段映射、筛选、排序、分页语义
- 不因局部优化改变查询含义
- 涉及事务优先沿用现有模式
- 固定 SQL 优先集中到 `internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go`
- SQL 文件过大时允许按业务领域平行拆分，如 `user_queries.go`，但不得在 repository / dao / service / handler 中散写固定 SQL
- SQL 改动必须同步检查字段类型、索引、外键和查询结果扫描顺序

## 缓存
- Redis 字符串值若可能经 JSON 序列化后入库，读取后比较前先做解码兼容
- 涉及会影响查询结果的数据写操作后，按现有 key 规则删除单体缓存、列表分页缓存和 total 缓存
- 如现有项目已有延迟双删或消息驱动失效机制，优先沿用现有实现，不擅自更换缓存一致性策略
- Redis 用于限流、幂等、锁、计数、会话状态时，必须明确 key 设计、TTL、原子性、热点风险、失败策略和清理策略
- Redis key 前缀必须集中定义在 `internal/infrastructure/persistence/redis/hg_redis_key.go`，业务模块只组合具体业务 ID，不散落硬编码前缀
- 限流计数不得只关注命令原子性，还必须评估算法边界；高成本接口优先使用令牌桶/漏桶/滑动窗口，避免固定窗口临界突刺
- Redis 多命令组合若影响一致性或并发正确性，优先使用 Lua 脚本保证原子执行；Lua 脚本必须集中定义在 `internal/infrastructure/persistence/redis/hg_redis_script.go` 或同目录按领域拆分的脚本文件中，并在注释中说明 KEYS/ARGV 含义

## 兼容
- 不擅改数据库字段含义
- 不擅改 model 与库字段映射
- 不擅改序列化字段和 tag

## 模型
- 不混用数据库模型、接口模型、业务模型
- 不随意合并或拆分模型
