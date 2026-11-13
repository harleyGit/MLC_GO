# Media 模块：隔离上传、审核封存与延迟清理

本模块采用 SeaweedFS S3 Gateway；文件从浏览器直接传至私有隔离桶，调用方必须提交 `purpose`。MySQL 保存会话、资源、引用和可恢复删除任务。隔离对象键为 `quarantine/{随机32位ID}`，符合现有 IM 审核器严格协议；业务场景和文件类型在元数据与上传策略中隔离，不在 Key 中透出。审核器尚不支持 GIF/字幕，初始化对此类申请失败关闭。**不要把新模块 `asset_id` 直接传给旧的 IM、商城或头像接口**：那些接口当前仍按各自原协议写入/读取旧对象，未自动切换。

## 前置条件与启用

在获准环境按顺序审查并部署 `000070_media_assets`、`000071_media_review_cleanup`、`000072_media_asset_refs`；本模块不会自动执行迁移。新增表缺失时，显式启用媒体模块会启动失败。旧环境与新表并行，不自动回填现有数据。2026-11-13 已获准在本机独占 debug `HG_MLC_DB` 执行 `000070`～`000072`，只读复核 `migration_version=72 dirty=false`；这不代表其他环境已应用。

通过进程环境注入 `MLC_MEDIA_S3_ENDPOINT`、`MLC_MEDIA_S3_REGION`、`MLC_MEDIA_S3_BUCKET`、`MLC_MEDIA_S3_ACCESS_KEY`、`MLC_MEDIA_S3_SECRET_KEY`、`MLC_MEDIA_CLEANUP_S3_ACCESS_KEY`、`MLC_MEDIA_CLEANUP_S3_SECRET_KEY`、`MLC_MEDIA_REVIEW_URL`、`MLC_MEDIA_REVIEW_TOKEN`。上传身份仅需隔离键 PUT/HEAD 和封存键 HEAD；SeaweedFS 的 HEAD 授权等同 `s3:GetObject`。独立清理身份仅授予目标私有桶 `quarantine/`、`sealed/` 的 `s3:DeleteObject`，不授予 PUT/GET；服务端组合两种身份复查与回收。可选只读身份 `MLC_MEDIA_READ_S3_ACCESS_KEY` / `MLC_MEDIA_READ_S3_SECRET_KEY` 专供低敏图片的封存短时 GET 签名；缺少时下载入口失败关闭。所有身份分离，不能复用上传或审核身份。无 S3 端点时不注册模块；配置 S3 但缺清理身份或审核器时启动失败。非 debug 环境必须使用 HTTPS。**不要提交真实凭据或带签名 URL**。S3 Gateway 的域名/Host 须与预签名 URL 一致；私有桶禁止匿名读写，签名 Content-Length/Content-Type 及封存条件写入 `If-None-Match` 必须在隔离环境实测。`MLC_MEDIA_REVIEW_URL` 采用现有 IM 审核器的 JSON 协议：它必须真实计算摘要、探测格式、做安全扫描并将文件幂等复制到 `sealed/{asset_id去连字符}`，拒绝覆盖封存键；服务端不能仅回显上传者声明。对审核器的 HTTP 调用沿用 `internal/modules/im/transport` 的既有受信协议客户端，不复用 IM 的业务授权。

## HTTP 接口

| 接口 | 用途 |
| --- | --- |
| `POST /api/v1/media/uploads/init` | JWT，本人申请上传；JSON 必填 `purpose`、`mime_type`、`file_size`、`sha256`、`idempotency_key`，直传授权十分钟。 |
| `POST /api/v1/media/uploads/complete` | JWT，JSON `{ "upload_id": "..." }`；真实长度符合后进入 `processing`，仍不可引用。 |
| `GET /api/v1/media/assets/status?asset_id=...` | JWT，查询本人申请元数据和状态。 |
| `GET /api/v1/media/assets/download?asset_id=...` | JWT，仅低敏图片用途的原上传者且 `ready` 可获得 30 秒私有读授权；聊天、实名认证、售后、视频源禁止走此入口，仍须业务鉴权网关。 |

API Guard 目前只支持固定路径；变更为文档示例中的 `{id}` 路径前，必须一并改造 Guard 并测试动态路径认证。`ready` 需要受信审核器返回与申请一致的 MIME、SHA256、大小、宽高/时长和指定封存键，并由服务器再次 HEAD 校验封存对象长度；真实字节内容的审核责任仍在外部审核器。

## 事务与后台

- 审核使用行级 `SKIP LOCKED` 与 330 秒租约。失败有退避，最多 12 次；过期会话不进入 ready。新建资源无业务引用时，超过 24 小时进入回收；引用后标记 `attached`，最后一个引用解除后再保留 24 小时。
- 内部 `HGReplaceReferenceTx` / `HGDetachReferenceTx` **不是通用公网 API**：必须由具体业务服务在同一 MySQL 事务内先核验主体、租户、业务行和版本，再绑定 `ready` 资源或解除引用；跨数据库实例不得假装原子性。旧资源仍被 SKU/消息/订单等引用时不能删除。当前未新增任何业务模块调用方，故原业务路径仍不受影响。
- 过期扫描先把资源标为 `deleting`，同一事务写入隔离对象与可能存在的封存对象两件删除任务；后台租约领取后才执行 S3 Delete，失败最多 12 次并停留待人工处置。每 Pod 的审核、过期、删除和修复各有一个固定 worker，默认每秒各处理至多一件；**这不是亿级对象容量配置**。需要基于真实到期量、S3 延迟和索引执行计划评估分片、批量及扩容。`/readyz` 报告后台错误，但不能替代审核器或 SeaweedFS 故障演练。
- S3 签名在过期前可能继续有效，不能用短期链接实现即时撤销；涉及 IM 私有下载或高敏感材料，必须继续使用会话鉴权代理/可撤销凭据。审核通过前禁止开放 CDN；审核后的公开分发仍须独立配置内容策略、CDN 缓存及回源 ACL。

## 上线剩余条件

本机显式集成测试 `SERVER_ENV=debug MLC_MEDIA_DEBUG_INTEGRATION=1 go test ./internal/modules/media -run '^TestHGDebug(DeletePermissions|MediaLifecycle)$' -v -count=1` 会先用审核器只读身份 HEAD 验证随机新键不存在，再用独立清理身份分别删除**不存在**的 `quarantine/` 和 `sealed/` 键，两个权限均确认后才生成图片、写入隔离桶和数据库、调用真实审核器、尝试拒绝条件覆盖、比对封存字节，并通过持久删除任务删除与复查。2026-11-13 已获准在本机 SeaweedFS 4.51 静态配置追加仅允许目标两类键 `s3:DeleteObject` 的独立身份，SIGHUP 热重载后空键删除通过、封存 PUT 明确被拒；没有修改旧 IM 身份。**真实生命周期两次停在 `HGInit` 创建会话/签名返回阶段，未上传图片、未调用审核器；每次唯一测试记录已在明确开启 `MLC_MEDIA_DEBUG_RECOVER=1` 的恢复测试中，通过持久删除任务清理并核对新对象键不存在。** 不能据此声称完成封存/真实对象删除验收。

注意：本机审核器与旧 IM 共用 `im-private` 桶，S3 策略在相同 `quarantine/`、`sealed/` 前缀上无法区分新旧对象。worker 在应用层校验资源 UUID 和持久任务归属，但测试身份一旦泄漏仍有删除同前缀旧 IM 对象的潜在权限；生产不能把这套 debug 共桶配置直接当作最终最小权限方案，应改为新模块独立桶/审核器身份并验证隔离。

可信审核/不可变封存、真实对象删除、SeaweedFS 故障恢复及容量压测均未完成验收；`HGInit` 返回 `*errors.errorString` 的原因尚待定位，不能重复试跑或跳过清理验证。数据库 DDL 只在上述 debug 库验证，不能推广到其他环境。**不得仅凭编译、单元测试或本地签名测试宣称可切流。** 业务切流与全量离线迁移按 `HG_MIGRATION_ROLLOUT.md` 执行前必须再次核对业务权威归属和目标环境。
