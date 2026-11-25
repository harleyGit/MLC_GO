# 恢复邮箱与 MFA 找回

## 用户行为

- 所有用户可在「个人中心 → 安全中心 → MFA 与恢复邮箱」绑定 TOTP。现有 IAM 管理员接口与权限要求不变；MFA **不作为登录第二步**。
- 输入恢复邮箱，发送并手动输入邮件码，再输入当前验证器动态码。两项均通过才显示「已验证」。更换邮箱不会更改登录邮箱；换址重新冷却。debug 冷却 1 分钟，pre 和生产（前端 release 对应的后台环境）为 1 小时。
- 验证器丢失时先登录，再向**已验证**邮箱发送找回码。输入当前登录密码和找回码、确认重置；旧因子删除，旧登录会话立即失效，随后重新登录并绑定新验证器。没有预先验证的邮箱不能走自助找回。

## 部署顺序

1. **先应用 `migrations/000087_mfa_recovery.up.sql`，再替换应用实例。** 新迁移只新增两个表，不改写或回填 `users`；启动检查只验证表结构，不自动迁移。旧会话按零代兼容，不因升级本身退出。鉴权将代际和 IM 强制下线合并为一次索引点查；仅已重置账号的会话额外校验一个 Redis 代际键。
2. **配置系统密钥及发信账户。** 继续使用各实例一致的 `MLC_AUTH_JWT_SECRET_HEX` 和 `MLC_IAM_MFA_KEY_HEX`；两者分别用于登录 JWT 和 MFA 加密/恢复邮件码摘要。通过部署密钥管理注入，不要提交凭据、邮件验证码或请求体。
3. **确认所有服务实例都支持代际校验，再启用找回。** pre/prod 默认关闭重置，全部旧版本下线后再统一设置 `MLC_MFA_RECOVERY_RESET_ENABLED=1`；debug 默认开放。发布后如需暂停，先关闭开关。已发生重置后不得回滚至不校验代际的旧版本，否则会破坏已撤销会话的保证；不可把 down 迁移当成生产回滚，它会删除已验证状态。

### SMTP 配置

| 环境变量 | 作用 |
| --- | --- |
| `MLC_SMTP_HOST`、`MLC_SMTP_PORT` | 发信服务主机和端口；STARTTLS 默认 587，隐式 TLS 默认 465 |
| `MLC_SMTP_TLS_MODE` | `starttls` 或 `implicit`；未设置时端口 465 走隐式 TLS，其他走 STARTTLS |
| `MLC_SMTP_FROM` | 发信服务允许的已授权发件地址 |
| `MLC_SMTP_USER`、`MLC_SMTP_PASSWORD` | SMTP 用户和该服务发放的应用专用密码、授权码或 SMTP 凭据 |

同一套 SMTP 可以发送给 Gmail、iCloud Mail、QQ 等收件邮箱，不按收件域名挑选 SDK 或登录方式。若使用这些服务作为**发件方**，以账户与服务商规则为准，例如 Gmail 可在允许应用专用密码的账户使用 STARTTLS/587，iCloud Mail 使用 STARTTLS/587 和应用专用密码，QQ 邮箱可用隐式 TLS/465 和 SMTP 授权码。仅允许 OAuth 的发件租户需另行配置支持其认证方案的中继。真实送达依赖供应商限制、域名 SPF/DKIM/DMARC 和收件过滤；SMTP 接受不等于已进入收件箱。

## 协议与失败处理

- 旧 `GET /api/v1/auth/send_email_code` 保留，Web 改用同路径的 `POST`，邮件地址及点选验证码凭证不再进入 Web 请求 URL。两者均只返回发送结果，验证码 5 分钟有效，不在响应、日志或注册成功结果中回显。注册用码与 MFA 绑定、找回用码不能互换。
- 本人 `/api/v1/iam/me/mfa/{enroll,confirm,recovery}` 及 `/recovery/{send-bind,bind,send-reset,reset}` 经过 JWT 鉴权；不接收目标账号 ID，找回发信也不接收收件邮箱。状态只展示邮箱掩码。
- 发信共享每邮箱每分钟一次和每站每分钟 120 次的上限，恢复发信额外限制每账号每分钟一次；单实例最多八个并发 SMTP 连接，每次最长五秒。未配置或邮件服务故障时拒绝发信，其他业务不依赖 SMTP。已经被 SMTP 接收但挑战持久化失败时，客户端会看到失败，需等待限流后重发。
- 恢复邮件码与数据库写入同事务消费；输错密码或 MFA、唯一键冲突、事务回滚都不会吃掉有效邮件码。即使缓存未清理或网关断连没确认，旧 Access、Refresh、重新接入的 IAM 连接和已撤销 IM 设备凭据都会被权威检查拒绝。既有 WebSocket 的物理断连依赖网关确认；断连不作为撤销提交条件。
- SQL 写入锁定一个用户及其专属记录，IM 活跃设备写入有界；生产容量和投递成功率必须按实际负载、发信配额和上线监控评估。

## 隔离验收

仅对有授权的临时实例运行，下列测试不使用部署配置或真实发件账户：

```bash
MLC_MFA_RECOVERY_MYSQL_INTEGRATION=1 go test -tags production ./internal/modules/iam -run '^TestHGMFARecoveryMySQLIntegration$' -count=1
MLC_AUTH_SESSION_REDIS_INTEGRATION=1 go test -tags production ./internal/modules/user/service -run 'TestHG(AuthSession|RegistrationEmail)RedisIntegration' -count=1
go test -tags production ./internal/pkg/email ./internal/modules/user/handler -count=1
```
