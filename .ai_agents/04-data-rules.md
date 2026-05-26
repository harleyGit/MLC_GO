# 04-data-rules.md

## 数据层
- 数据访问层写法保持一致
- 不擅改 SQL、ORM、查询封装方式、repository / dao 职责边界
- 大流量写入链路检查热点行/索引、唯一键冲突、事务范围、连接池压力、批量写入策略

## SQL / ORM
- 查询风格保持一致，不因局部优化改变字段映射、筛选、排序、分页语义
- 事务沿用现有模式
- 固定 SQL 优先集中到 `internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go`
- SQL 文件过大时允许按业务领域平行拆分，如 `user_queries.go`，但不得在 repository / dao / service / handler 中散写固定 SQL
- SQL 改动必须同步检查字段类型、索引、外键和查询结果扫描顺序

## 缓存
- Redis 字符串值若可能经 JSON 序列化后入库，读取后比较前先做解码兼容
- 影响查询结果的写操作后，按现有 key 规则清理单体、列表分页和 total 缓存
- 沿用现有延迟双删或消息驱动失效机制，不擅自更换一致性策略
- Redis 用于限流、幂等、锁、计数、会话状态时，明确 key、TTL、原子性、热点、失败和清理策略
- Redis key 前缀必须集中定义在 `internal/infrastructure/persistence/redis/hg_redis_key.go`，业务模块只组合具体业务 ID，不散落硬编码前缀
- Redis 多命令组合若影响一致性或并发正确性，优先用 Lua；脚本集中定义在 `internal/infrastructure/persistence/redis/hg_redis_script.go` 或同目录领域文件，并注释 KEYS/ARGV

## 兼容
- 不擅改数据库字段含义
- 不擅改 model 与库字段映射
- 不擅改序列化字段和 tag

## 模型
- 不混用数据库模型、接口模型、业务模型
- 不随意合并或拆分模型
