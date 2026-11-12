# 媒体隔离上传（未完成业务接入）

本模块按 `purpose` 将用户、商品、IM、视频、评论、售后和营销文件分类，但**所有上传先落在同一个私有隔离桶**。`complete` 仅核对 S3 对象长度，状态最多为 `processing`；尚无可信内容审核/病毒扫描、不可变封存、引用切换和删除 worker，因此**绝不能用这里的 `asset_id` 替换现有 IM、头像或视频生产链路，也不能对外开放隔离桶的读取或 CDN**。旧链路保持原状；不要把两个系统的元数据表当作同一套数据。

## 启用前置条件

1. 审核并在获准环境部署 `migrations/000070_media_assets.up.sql`。应用不会自动执行迁移。表缺失时显式配置媒体端点会导致启动失败。
2. 由基础设施提供 SeaweedFS S3 Gateway、私有隔离桶、最小权限凭据、长度限制、恶意流量防护与 Bucket 生命周期清理策略；**在完成清理 worker 前必须设置隔离桶 24 小时以上的自动过期回收**，处理中的对象也会被清理，因此不得把它当作可用资源。生产上传域名必须使用 TLS，且签名域名/Host 与浏览器访问一致。自行验证 SeaweedFS 实际执行签名的 Content-Length 与 Content-Type 约束。
3. 通过环境注入 `MLC_MEDIA_S3_ENDPOINT`、`MLC_MEDIA_S3_REGION`、`MLC_MEDIA_S3_BUCKET`、`MLC_MEDIA_S3_ACCESS_KEY` 和 `MLC_MEDIA_S3_SECRET_KEY`。不设置端点则不注册模块；仅 `SERVER_ENV=debug` 允许 HTTP S3 Gateway。不要在仓库配置文件中存入真实凭据。部署前校验 Gateway 的 CORS、Host、PUT/HEAD 权限，以及 CDN、限流、容量和故障恢复策略。

## 当前协议

- `POST /api/v1/media/uploads/init`：JSON 必填 `purpose`、`mime_type`、`file_size`、`sha256`、`idempotency_key`；仅在 `pending` 时返回十分钟 PUT URL 及 `Content-Type` 请求头。请求者身份由 JWT 获取，不接受客户端 Bucket、Object Key 或 owner；上传字节数由浏览器自动设置 `Content-Length`。
- `POST /api/v1/media/uploads/complete`：JSON `{ "upload_id": "..." }`，仅转为 `processing`，不能用于绑定。
- `GET /api/v1/media/assets/status?asset_id=...`：只查询本人资源的状态和**申请时声明的**元数据。

项目现有 API Guard 使用精确路径，故这里未采用规格文档的路径参数形式；若需改回 `/uploads/{id}/complete`，须同步扩展 Guard 并验证不会绕过鉴权。`sha256`、`mime_type` 和 `file_size` 在审核前都不能作为真实文件属性；普通 S3 预签名 URL 在到期前也无法即时吊销。

## 未完成的生产条件

先对接可信扫描/类型探测/摘要校验/不可变封存与审核回调，随后实现业务归属校验、引用事务、Outbox、延迟删除、会话/分片过期清理、私有受控下载和老数据迁移，最后逐业务灰度切换。现有单用户最多 20 个活动会话、累计申请 1 GiB；这是防护阈值，不代表存储网关或整套集群已通过千万并发/亿级对象的压测。部署前需评估隔离桶过期窗口、热点 owner 行锁、网关/数据库连接池、CDN 命中率与分片上传的成本。
