# IM 后端接入与当前验收边界

依据 `/Users/ganghuang/Desktop/IM-消息.md`，在现有 `internal/modules/im` 增量实现单聊实时指针投递、私有媒体和 LiveKit 单聊呼叫。**代码接入不等于完整单聊 IM 已上线，也没有千万连接/亿级容量证据。** 本轮没有可用的隔离外部实例，三个扩展开关默认关闭，pre/prod 仍不注册 IM。

## 代码与数据边界

- `internal/modules/im/`：现有会话/消息/设备 ACK，新增实时任务、私有附件、单聊呼叫及 HTTP 入口。
- `internal/modules/im/transport/`：Centrifugo v6 HTTP API/连接 JWT、SeaweedFS S3 SigV4、受信审核器 HTTP 协议、LiveKit JWT/Twirp/回调验签。使用现有 `golang-jwt/jwt/v5`，没有新增 Go 依赖。
- `internal/modules/im/module/`：默认关闭的环境配置、路由目录/Guard 共用 RouteSpec、独立 Kafka 消费及固定数量后台任务。应用关闭时取消并等待任务，再关闭连接池。
- `internal/consumer/im/`：独立消费组，Inbox 与投递任务同事务；不把 Kafka offset 提交当作设备送达。
- `internal/pkg/mysql/queries/hg_im_*_queries.go`：集中参数化 SQL。
- `migrations/000054_im_realtime_media_call.*.sql`：新增 8 张扩展表，并给旧 `im_device_delivery` 增加 `(message_id,device_id)` 索引。**未执行**；必须单独授权迁移。`LOCK=NONE` 仍会产生元数据锁、索引构建 I/O 和复制压力，需实际表规模评审。down 会丢失扩展业务数据，不是生产回滚方案。

沿用 000052 的单聊会话、消息 ID、seq 和 `clientMsgId`，不替换已应用迁移；原 `im_media` 表保持兼容，新上传使用 `im_private_attachment`。没有实现群聊、好友/屏蔽策略、消息撤回、离线 APNs/FCM/Web Push、录制或 React SDK 对接。

## 启用配置

配置通过现有 `InitRuntimeEnv` 注入；以下是变量名，真实凭据不得提交。非本机外部端点必须 HTTPS/WSS；所有外部 HTTP 调用禁止重定向。

| 变量 | 约束 |
| --- | --- |
| `MLC_IM_REALTIME_ENABLED` | 默认 false；显式 true 且 `SERVER_ENV=debug` 才启用 |
| `MLC_IM_MEDIA_ENABLED` / `MLC_IM_CALL_ENABLED` | 默认 false；均依赖实时开关 |
| `MLC_IM_TOPIC` | 必须显式配置，例如 `im_message_events_v1`；禁止与共享业务 topic 重名 |
| `MLC_IM_CONSUMER_GROUP` | 独立消费组，例如 `mlc-debug-im-realtime-v1`；禁止与其他投影共用 |
| `MLC_IM_CENTRIFUGO_API_URL` | API 根地址，不含 `/api/publish` |
| `MLC_IM_CENTRIFUGO_WS_URL` | 客户端完整 WebSocket URL，包含 `/connection/websocket` |
| `MLC_IM_CENTRIFUGO_API_KEY` | 服务端专用，至少 16 字符；请求头 `X-API-Key` |
| `MLC_IM_CENTRIFUGO_TOKEN_SECRET` | 独立连接 JWT 密钥，至少 32 字符 |
| `MLC_IM_S3_ENDPOINT` / `MLC_IM_S3_REGION` / `MLC_IM_S3_BUCKET` | SeaweedFS S3 端点、签名 region、独立私有桶 |
| `MLC_IM_S3_ACCESS_KEY` / `MLC_IM_S3_SECRET_KEY` | 上传隔离区写权限、封存区只读权限；不复用公开 CDN 身份 |
| `MLC_IM_REVIEW_URL` / `MLC_IM_REVIEW_TOKEN` | 必需的审核/封存服务；token 至少 32 字符，缺失不得启用媒体 |
| `MLC_IM_LIVEKIT_API_URL` / `MLC_IM_LIVEKIT_WS_URL` | LiveKit HTTP 控制根地址、客户端信令地址 |
| `MLC_IM_LIVEKIT_API_KEY` / `MLC_IM_LIVEKIT_API_SECRET` | API Key 与独立签名密钥；Secret 至少 32 字符 |

没有自动创建 Kafka topic、桶、容器或密钥。Kafka 复用现有业务集群的可靠生产者/Outbox，但消费组和 topic 独立。需先配置副本、ISR、保留期、ACL 与 `hg.dlq.business` 故障处理；分区键是会话 ID。现有并发 Outbox 发布不承诺按 seq 到达 Kafka，客户端必须按权威 seq 补洞排序，不能声称全链路严格到达有序。

当实时扩展关闭时，保留原 debug HTTP 路由与共享业务 topic 兼容路径。切换前产生的旧事件不会被新独立消费组自动补发，历史依赖现有授权补拉；应在隔离环境定义切换水位，不能直接重写既有 Outbox。

## Centrifugo 网关与可靠性

`deployments/im/hg_centrifugo.example.json` 是 **v6 配置模板，尚未经真实二进制校验**。部署时锁定具体版本和镜像 digest，注入 `CENTRIFUGO_CLIENT_TOKEN_HMAC_SECRET_KEY`、`CENTRIFUGO_HTTP_API_KEY`，替换精确 Origin 和隔离 Redis 地址。多节点须连接同一 Redis broker；API 端点仅对 Go 服务开放。禁止 `client.insecure`、任意 Origin 和客户端直接发布。

1. JWT 登录后注册设备，取得 `deviceId`/`secret`；最多五台活跃设备，30 天未活动失活。
2. `POST /api/v1/im/realtime/token`，body 为设备凭据。返回 `token,url,channel,expiresIn`。连接 JWT 的 `sub=im/<deviceId>`、`iss=mlc-im`、`aud=centrifugo`，只自动订阅本设备 `im:<deviceId>`。不要再调用 `newSubscription()` 订阅此频道。
3. 客户端通过 centrifuge-js 的连接级 `publication`/`subscribed` 事件接收服务端订阅，刷新时重新调用 token 接口。有效期 60 秒；撤销设备后拒绝刷新，已有连接仍可能存续到 token 到期加网关宽限，只收到指针，正文仍须 JWT 资源鉴权。
4. 先建立订阅，再分页读取 `/conversations` 和 `/messages?conversationId=...&afterSeq=...`。连接恢复失败、epoch 变化、游标丢失、新设备登录都必须走 MySQL 补拉。频道历史仅 100 条/300 秒，不是正式历史。
5. 网关事件包含 `eventId,eventType,eventVersion,conversationId,messageId,seq` 或 `callId,version`。客户端以 `messageId` 去重、按 seq 连续补拉；持久保存/处理消息后才调用 `/device/ack`。自己的发送回执同样只是 `sent`。
6. 消息/原设备投递记录/Outbox 原子提交；独立消费者校验事件与权威消息后，将 Inbox 与设备任务原子提交。Kafka 重放不会重置已存在任务；同 EventID 不同内容拒绝。
7. 每进程单次最多抢四条投递任务，`FOR UPDATE SKIP LOCKED`、30 秒租约、随机栅栏 token，外部发布每次最多三秒。失败或设备未 ACK 时指数退避，最多十二次。ACK 后或设备撤销/失活后停止。
8. 耗尽任务保留 `state=3`，**不标记 delivered**。原 `/device/pending` 和消息历史仍可恢复；没有自动无限重试/自动死信重放。通话指针在网关确认后完成任务，不代表设备确认接听。

新增 `/device/revoke` 接受本人设备凭据。重连后应先处理当前设备 pending（以 `messageId` 游标翻页）再核对每个会话 seq。新注册设备不回填所有旧设备任务，必须同步会话历史。

## 私有媒体 API 与审核契约

所有业务路径前缀为 `/api/v1/im`，保留项目 JWT、请求签名和响应外壳。

| 路径 | 协议 |
| --- | --- |
| `POST /media/upload/init` | `{conversationId,messageType,mime,size,sha256}`；返回 `{mediaId,uploadUrl,method,headers,size,expiresIn}` |
| `POST /media/upload/complete` | `{mediaId}`，只声明完成，进入 verifying |
| `GET /media/status?mediaId=...` | 上传者读取 `state,width,height,durationMs` |
| `POST /messages` | 原协议，新增 audio/file；content 仍为 `{mediaId,width,height,durationMs}` |
| `GET /media/read?mediaId=...` | JWT + 会话成员 + ready + 消息关联验证；支持单段 `Range: bytes=start-end` |
| `POST /media/delete` | `{mediaId}`，本人撤销后续发送/读取；不立即物理删除 |

图片最多 10MiB，其他单文件最多 128MiB。每用户最多 100 个上传预留、总申请大小不超过 512MiB；失败/删除不自动返还配额。PUT 凭证十分钟，签名绑定 Content-Length、Content-Type 和 `x-amz-acl: private`。浏览器直接上传 Blob，让浏览器生成匹配的 Content-Length；禁止压缩/转换代理改写请求体。**SeaweedFS 对签名头、ACL、CORS、区域及签名有效期的具体支持尚未实测。** 桶策略还必须禁止匿名读取、公开 ACL 和用户覆盖封存区，不能只依赖 Go 参数检查。

审核器 `POST MLC_IM_REVIEW_URL`，Authorization Bearer，仅接受 Go 服务身份。请求示例（无凭据）：

```json
{"mediaId":"32位小写十六进制ID","bucket":"im-private","objectKey":"quarantine/<id>","sealedKey":"sealed/<id>","sha256":"64位小写摘要","mime":"image/png","size":1024}
```

返回 `status=pending|rejected|ready`。只有 ready 同时携带以下**服务端实际检测值**才可用：`sha256,size,mime,sealedKey,width,height,durationMs`。

审核服务必须实现：有界获取对象、真实格式与尺寸/时长探测、实际 SHA256、恶意文件与内容审核、复制/写入不可变 `sealed/<id>` 后核对封存字节，并对 mediaId 幂等；不得先审核隔离区再无保护复制已被替换的内容。已封存对象不得覆盖。**本仓库提供审核调用和状态编排，不包含实际扫描/内容审核供应商或转码服务。未部署上述审核器是上线阻断项。** 每次审核调用五秒超时，pending/错误三十秒后重试，最多十二次，之后 rejected。

发送事务锁定 ready 附件并校验上传者、会话、类型及实际元数据，同事务建立 `im_message_attachment`；不扫描 JSON 做读授权。读取每次重新授权后由 Go 流式代理，客户端拿不到对象下载签名 URL。下载每进程最多十六条并发，满载 429，单请求预算 25 秒；大视频需 Range 分段。撤销只阻止后续请求，不追溯中断已授权在途流。不提供公开 CDN。

未完成：隔离区/未关联对象自动清理、保留期、配额释放的可恢复流程、缩略图/转码和真实审核策略验收。不得按此临时配额直接扩到生产海量用户。

## LiveKit 单聊呼叫

- `POST /calls`：`{conversationId,requestId,mediaType:"audio"|"video",deviceId,secret}`。幂等创建，双方固定顺序忙线占位，45 秒振铃，呼叫与状态信令 Outbox 同事务。
- `GET /calls?callId=...`：只允许当前会话参与者读取权威状态。
- `POST /calls/action`：`{callId,action:"accept"|"reject"|"cancel"|"end",deviceId,secret}`。接听行锁竞争，只有一个被叫设备胜出；主叫取消与接听互斥。
- `POST /calls/token`：`{callId,deviceId,secret}`。仅 accepted 且绑定设备可获取；外部 CreateRoom 后再次锁定验证业务状态；JWT 仅指定房间、设备身份、音频/摄像头源，禁止管理/录制/数据发布，60 秒有效。
- 房间限制两名参与者，通话最多一小时。accepted 仅代表业务接听，不冒充实际 WebRTC 连接状态。
- 管理面 `POST /internal/im/livekit/webhook`：使用 LiveKit HS256、issuer、exp、原始正文 Base64 SHA256 校验，正文最多 64KiB，事件 ID 与载荷摘要去重。仅 `room_finished` 更新终态、释放忙线并写 Outbox；房间 SID 防止同名重建的旧回调关闭新实例。其他已验签事件不改变状态。
- 结束/拒绝/取消/超时后，后台重复 DeleteRoom，覆盖已签发 Token 的有效期。**自建 LiveKit 已发 Token 不能立即吊销，终止后仍有约一分钟重入窗口，清理失败时连接可能延长；这不是即时撤权保证。** 必须经隔离环境故障演练与业务接受后才能解除生产门禁。

尚需验证 TURN、TLS、UDP、公网地址、带宽、浏览器权限及多设备断网重连。回调无可靠到达保证，漏回调时一小时硬截止兜底；没有完整参与者状态对账、录制或多人会议。

## 检查、容量与上线阻断

进程指标：`mlc_im_publish_failures_total`、`mlc_im_delivery_exhausted_total`、`mlc_im_worker_failures_total{worker=delivery|media|calls}`；复用 Kafka lag 指标。Ready 检查任务是否启动/异常退出及最近后台错误，不能证明所有外部服务均可用。还需持久队列最老年龄、任务积压、ACK 延迟、审核拒绝原因及回调延迟告警。标签不含用户/消息/设备 ID。

单聊消息仍是单库自增 ID 与会话行锁；没有分片路由、归档切换或亿级数据实测。任务与 Inbox 保留期/清理没有自动调度；按固定索引和有界批次处理，不宣称当前 worker 的吞吐可以支撑千万在线。媒体代理带宽是独立容量瓶颈。扩容前必须提供连接数、发送/扇出 QPS、消息大小、媒体码率、SLO、隔离数据生成与清理边界。

上线前仍阻断：000054 未执行、组件版本/digest 未锁定及配置未实机验证、真实审核/封存服务未提供、Centrifugo 多节点重试/ACK/恢复未实测、LiveKit 与私有媒体端到端未实测、React 实时/媒体/通话未接入、通知全量在线重建/切换与归档调度未完成、千万连接和亿级数据未压测。

本轮未启动服务、未创建 topic/桶、未执行迁移或真实外部验收；没有新增测试文件，安全协议负向用例追加到现有 `main_mlc_project_shutdown_test.go`。编译/本地替身用例不能代替上述上线条件。

官方协议核对：
- https://centrifugal.dev/docs/server/authentication
- https://centrifugal.dev/docs/server/server_api
- https://centrifugal.dev/docs/server/engines
- https://docs.livekit.io/reference/other/roomservice-api.md
- https://docs.livekit.io/intro/basics/rooms-participants-tracks/webhooks-events.md
