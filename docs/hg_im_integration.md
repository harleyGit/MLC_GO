# IM 后端接入与当前验收边界

依据 `/Users/ganghuang/Desktop/IM-消息.md`，在现有 `internal/modules/im` 增量实现单聊实时指针投递、私有媒体和 LiveKit 单聊呼叫。**代码接入不等于完整单聊 IM 已上线，也没有千万连接/亿级容量证据。** 本机 debug 已启动隔离存储、ClamAV、Centrifugo 和仅覆盖裸露类别的本地审核模型，私有配置中实时与媒体已显式开启（须重启原 VS Code debug 进程后生效）；默认关闭，pre/prod 仍不注册 IM。

## 代码与数据边界

- `internal/modules/im/`：现有会话/消息/设备 ACK，新增实时任务、私有附件、单聊呼叫及 HTTP 入口。
- `internal/modules/im/transport/`：Centrifugo v6 HTTP API/连接 JWT、SeaweedFS S3 SigV4、受信审核器 HTTP 协议、LiveKit JWT/Twirp/回调验签。使用现有 `golang-jwt/jwt/v5`，没有新增 Go 依赖。
- `internal/modules/im/module/`：默认关闭的环境配置、路由目录/Guard 共用 RouteSpec、独立 Kafka 消费及固定数量后台任务。应用关闭时取消并等待任务，再关闭连接池。
- `internal/consumer/im/`：独立消费组，Inbox 与投递任务同事务；不把 Kafka offset 提交当作设备送达。
- `internal/pkg/mysql/queries/hg_im_*_queries.go`：集中参数化 SQL。
- `migrations/000054_im_realtime_media_call.*.sql`：新增 7 张扩展表，并给旧 `im_device_delivery` 增加 `(message_id,device_id)` 索引。本机 debug 经只读核对为 `version=54 dirty=false`，本轮迁移执行 `no change`；**不能据此推断 pre/prod 已执行**。`LOCK=NONE` 仍会产生元数据锁、索引构建 I/O 和复制压力，需实际表规模评审。down 会丢失扩展业务数据，不是生产回滚方案。DDL 各语句分别提交，部分失败可能留下已建索引/表，不能直接 force 版本或盲目重跑；须核对已执行语句后制定恢复步骤。

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

本机 debug 的 `make kafka-init` 现会创建/校验独立 `mlc.im.messages.v1`（12 分区、3 副本及 ISR）；pre/prod 不自动创建 Topic，桶、容器和密钥也须分别管理。Kafka 复用现有业务集群的可靠生产者/Outbox，但消费组和 topic 独立。需先配置副本、ISR、保留期、ACL 与 `hg.dlq.business` 故障处理；分区键是会话 ID。现有并发 Outbox 发布不承诺按 seq 到达 Kafka，客户端必须按权威 seq 补洞排序，不能声称全链路严格到达有序。

当实时扩展关闭时，保留原 debug HTTP 路由与共享业务 topic 兼容路径。切换前产生的旧事件不会被新独立消费组自动补发，历史依赖现有授权补拉；应在隔离环境定义切换水位，不能直接重写既有 Outbox。

## Centrifugo 网关与可靠性

`deployments/im/hg_centrifugo.example.json` 是 v6 配置模板；本机生成的私有配置已通过 v6.2.4 `checkconfig`、单节点 `/health` 和授权 HTTP publish 检查，但**未实测该模板的 Redis 多节点模式**。部署时锁定具体版本和镜像 digest，注入独立 JWT/API 密钥，替换精确 Origin 和隔离 Redis 地址。多节点须连接同一 Redis broker；API 端点仅对 Go 服务开放。禁止 `client.insecure`、任意 Origin 和客户端直接发布。

1. JWT 登录后注册设备，取得 `deviceId`/`secret`；最多五台活跃设备，30 天未活动失活。
2. `POST /api/v1/im/realtime/token`，body 为设备凭据。返回 `token,url,channel,expiresIn`。连接 JWT 的 `sub=im/<deviceId>`、`iss=mlc-im`、`aud=centrifugo`，只自动订阅本设备 `im:<deviceId>`。不要再调用 `newSubscription()` 订阅此频道。
3. 客户端通过 centrifuge-js 的连接级 `publication`/`subscribed` 事件接收服务端订阅，刷新时重新调用 token 接口。有效期 60 秒；撤销设备后拒绝刷新，已有连接仍可能存续到 token 到期加网关宽限，只收到指针，正文仍须 JWT 资源鉴权。
4. 先建立订阅，再分页读取 `/conversations` 和 `/messages?conversationId=...&afterSeq=...`。连接恢复失败、epoch 变化、游标丢失、新设备登录都必须走 MySQL 补拉。频道历史仅 100 条/300 秒，不是正式历史。
5. 网关事件包含 `eventId,eventType,eventVersion,conversationId,messageId,seq` 或 `callId,version`。客户端以 `messageId` 去重、按 seq 连续补拉；持久保存/处理消息后才调用 `/device/ack`。自己的发送回执同样只是 `sent`。
6. 消息/原设备投递记录/Outbox 原子提交；独立消费者校验事件与权威消息后，将 Inbox 与设备任务原子提交。Kafka 重放不会重置已存在任务；同 EventID 不同内容拒绝。
7. 每进程单次最多抢四条投递任务，`FOR UPDATE SKIP LOCKED`、30 秒租约、随机栅栏 token，外部发布每次最多三秒。失败或设备未 ACK 时指数退避，最多十二次。ACK 后或设备撤销/失活后停止。
8. 耗尽任务保留 `state=3`，**不标记 delivered**。原 `/device/pending` 和消息历史仍可恢复；没有自动无限重试/自动死信重放。通话指针在网关确认后完成任务，不代表设备确认接听。

新增 `/device/revoke` 接受本人设备凭据。重连后应先处理当前设备 pending（以 `messageId` 游标翻页）再核对每个会话 seq。新注册设备不回填所有旧设备任务，必须同步会话历史。

任务完成要求更新命中当前租约；零行命中返回错误且不增加耗尽计数。发布确认与数据库提交之间仍可能重放，客户端去重不可省略。消费协议错误目前会阻塞该分区重试，需监控和人工处置，不能当作已实现自动毒消息隔离。

## 私有媒体 API 与审核契约

### 独立审核进程（仅隔离 debug 的有限类别审核）

`cmd/hg_im_review` 是可单独构建/运行的 HTTP 服务，`internal/modules/im/review/` 实现请求认证、固定桶与随机对象键校验、128MiB 上限、并发舱壁、实际 SHA256、ffprobe 格式/尺寸/时长探测及 ClamAV INSTREAM 恶意文件扫描。图片和短视频另用本地固定版本 NudeNet/ONNXRuntime 检测显式裸露，视频仅接受不超过 12 秒、按每秒 2 帧抽样；模型误判/漏判与抽帧间隔不能消除风险，超长视频拒绝。依赖缺失、模型异常及超时不能返回 ready。审核器以专属 S3 身份对 `sealed/<mediaId>` 做 `If-None-Match: *` 条件写入，并读回核对长度和 SHA256。**它不覆盖暴恐、涉政、版权、未成年人安全等完整内容合规类别；pre/prod 继续关闭。**

本机使用 ClamAV `1.4.3`（amd64 仿真，digest `sha256:75fb5fd95fcbe1d7e6d240c369c1572b686ee2c95949d1042b5148de8eddebb4`）、SeaweedFS `4.51`（digest `sha256:f7a49e3f3972ebed5be2ed4befa4a0c68eb7bdddba6e692d42729ceb545134b8`）和 Centrifugo `v6.2.4`（digest `sha256:b4d47454c5d9303ada0327954aa06afd75e1dd7adc00ee8f066dbd5072df894a`），均仅绑定本机端口。隔离 Python 通过 `deployments/im/hg_moderator_requirements.txt` 安装，`MLC_IM_REVIEW_PYTHON` 与 `MLC_IM_REVIEW_MODERATOR` 指定其可执行文件和脚本；模型只覆盖裸露类。进程启动时验证本机模型和 clamd PING。clamd `StreamMaxLength`、`MaxFileSize`、`MaxScanSize` 与 `AlertExceedsMax` 必须实机检查。跨主机连接需要受控 TLS 代理；本地运行不是可用性、误判率或容量证明。

按 `config/hg_im_debug.env.example` 把变量填入未提交的 `config/MLC.local.env` 或进程环境；`MLC_IM_REVIEW_TOKEN` 需同时提供给 Go 与审核进程，审核专用 `MLC_IM_REVIEW_S3_ACCESS_KEY` / `MLC_IM_REVIEW_S3_SECRET_KEY` 应有隔离区只读、封存区只读与条件写入权限，不能和浏览器预签名身份共用。单独运行 `go run ./cmd/hg_im_review`，然后通过 VS Code `🧪 Launch MLC_GO Debug` 重启已有进程以加载私有配置。审核调用预算 23 秒，后台单次 25 秒，超时返回失败并依既有租约重试，不默认通过。当前容器私有配置存于工程外系统临时目录，重启系统可能失效，需要恢复或重新生成；浏览器双用户验收和容量、故障演练仍未完成。

所有业务路径前缀为 `/api/v1/im`，保留项目 JWT、请求签名和响应外壳。

| 路径 | 协议 |
| --- | --- |
| `POST /media/upload/init` | `{conversationId,messageType,mime,size,sha256}`；返回 `{mediaId,uploadUrl,method,headers,size,expiresIn}` |
| `POST /media/upload/complete` | `{mediaId}`，只声明完成，进入 verifying |
| `GET /media/status?mediaId=...` | 上传者读取 `state,width,height,durationMs` |
| `POST /messages` | 原协议，新增 audio/file；content 仍为 `{mediaId,width,height,durationMs}` |
| `GET /media/read?mediaId=...` | JWT + 会话成员 + ready + 消息关联验证；支持单段 `Range: bytes=start-end` |
| `POST /media/delete` | `{mediaId}`，本人撤销后续发送/读取；不立即物理删除 |

图片最多 10MiB，其他单文件最多 128MiB。每用户最多 100 个上传预留、总申请大小不超过 512MiB；失败/删除不自动返还配额。PUT 凭证十分钟，签名绑定 Content-Length、Content-Type 和 `x-amz-acl: private`。浏览器直接上传 Blob，让浏览器生成匹配的 Content-Length；禁止压缩/转换代理改写请求体。本机 SeaweedFS 4.51 已实测 Go 预签名上传、带 ACL 的分前缀身份、条件封存及 `localhost:5173` 的 PUT 预检响应；**真实浏览器跨域发送、签名过期与代理链尚未实测**。桶策略还必须禁止匿名读取、公开 ACL 和用户覆盖封存区，不能只依赖 Go 参数检查。

审核器 `POST MLC_IM_REVIEW_URL`，Authorization Bearer，仅接受 Go 服务身份。请求示例（无凭据）：

```json
{"mediaId":"32位小写十六进制ID","bucket":"im-private","objectKey":"quarantine/<id>","sealedKey":"sealed/<id>","sha256":"64位小写摘要","mime":"image/png","size":1024}
```

返回 `status=pending|rejected|ready`。只有 ready 同时携带以下**服务端实际检测值**才可用：`sha256,size,mime,sealedKey,width,height,durationMs`。

审核服务必须实现：有界获取对象、真实格式与尺寸/时长探测、实际 SHA256、恶意文件与所需类别内容审核、写入不可变 `sealed/<id>` 后核对封存字节，并对 mediaId 幂等；不得先审核隔离区再无保护复制已被替换的内容。本机使用 SeaweedFS `3.83` 实测第二次 `If-None-Match: *` 仍返回 200，**不得使用该版本**；`4.51` 首次 200、重复 412，原字节未变。此证据不代替线上版本/桶策略验收或完整内容审核。审核 HTTP 调用最多 23 秒，pending/错误三十秒后重试，最多十二次，之后 rejected。

发送事务锁定 ready 附件并校验上传者、会话、类型及实际元数据，同事务建立 `im_message_attachment`；不扫描 JSON 做读授权。读取每次重新授权后由 Go 流式代理，客户端拿不到对象下载签名 URL。下载每进程最多十六条并发，满载 429，单请求预算 25 秒；大视频需 Range 分段。撤销只阻止后续请求，不追溯中断已授权在途流。不提供公开 CDN。

Range 仅接受 `bytes=start-end` 或 `bytes=start-`；206 响应须同时匹配请求起止、实际分段长度及审核总大小，拒绝未知总大小、编码变换和畸形范围。对象存储返回合法完整 200 时允许完整转发。图片/视频/音频使用 `inline`，file 使用 `attachment`；浏览器仍须携带项目鉴权，不能直接把无认证的 URL 填到媒体标签。

未完成：隔离区/未关联对象自动清理、保留期、配额释放的可恢复流程、缩略图/转码和真实审核策略验收。不得按此临时配额直接扩到生产海量用户。

## LiveKit 单聊呼叫

- `POST /calls`：`{conversationId,requestId,mediaType:"audio"|"video",deviceId,secret}`。幂等创建，双方固定顺序忙线占位，45 秒振铃，呼叫与状态信令 Outbox 同事务。
- `GET /calls?callId=...`：只允许当前会话参与者读取权威状态。
- `POST /calls/action`：`{callId,action:"accept"|"reject"|"cancel"|"end",deviceId,secret}`。接听行锁竞争，只有一个被叫设备胜出；主叫取消与接听互斥。
- `POST /calls/token`：`{callId,deviceId,secret}`。仅 accepted 且绑定设备可获取；外部 CreateRoom 后在短事务中重新检查成员、设备撤销状态及数据库时钟下的有效期，授权记录持共享锁直到签发提交。重复更新清理窗口的零行变更不表示过期。JWT 仅指定房间、设备身份、音频/摄像头源，禁止管理/录制/数据发布，Go 初始令牌 60 秒有效。
- 显式 CreateRoom 请求限制两名参与者，业务状态一小时后进入超时清理。accepted 仅代表业务接听，不冒充实际 WebRTC 连接状态；媒体连接的最终时限仍依赖外部清理与重入控制。
- 管理面 `POST /internal/im/livekit/webhook`：使用 LiveKit HS256、issuer、exp、原始正文 Base64 SHA256 校验，正文最多 64KiB，事件 ID 与载荷摘要去重。仅 `room_finished` 更新终态、释放忙线并写 Outbox；房间 SID 防止同名重建的旧回调关闭新实例。其他已验签事件不改变状态。
- 结束/拒绝/取消/超时后，后台在 `cleanup_until` 前重复 DeleteRoom；仅正常 JSON 成功响应或 Twirp `404 + code=not_found` 确认清理，普通代理 404 不算成功。**当前 120 秒窗口只覆盖 Go 初始令牌，无法保证覆盖 LiveKit 自主刷新的令牌。** 官方当前 `RoomManager.refreshToken` 会签发至少十分钟有效的令牌；重连还可能继续刷新，因此不能把重入风险描述为“一分钟内”，也不能仅将清理窗口延长到十分钟就称为可靠撤权。必须锁定实际版本，确定结束后拒绝重新入房/续签的方案并实测后才能解除生产门禁。

尚需验证 TURN、TLS、UDP、公网地址、带宽、浏览器权限及多设备断网重连。回调无可靠到达保证，漏回调时一小时业务超时兜底；没有完整参与者状态对账、录制或多人会议。同名房间跨节点创建/重建与迟到回调的竞态也需真实故障验收，当前 SID 比较不能替代版本绑定的入房控制。

## 检查、容量与上线阻断

进程指标：`mlc_im_publish_failures_total`、`mlc_im_delivery_exhausted_total`、`mlc_im_worker_failures_total{worker=delivery|media|calls}`；复用 Kafka lag 指标。Ready 检查任务是否启动/异常退出及最近后台错误，不能证明所有外部服务均可用。还需持久队列最老年龄、任务积压、ACK 延迟、审核拒绝原因及回调延迟告警。标签不含用户/消息/设备 ID。

单聊消息仍是单库自增 ID 与会话行锁；没有分片路由、归档切换或亿级数据实测。任务与 Inbox 保留期/清理没有自动调度；按固定索引和有界批次处理，不宣称当前 worker 的吞吐可以支撑千万在线。媒体代理带宽是独立容量瓶颈。扩容前必须提供连接数、发送/扇出 QPS、消息大小、媒体码率、SLO、隔离数据生成与清理边界。

上线前仍阻断：pre/prod 迁移与独立隔离环境未验收、完整内容合规类别未接入及模型误判率未评估、Centrifugo 多节点重试/ACK/恢复未实测、LiveKit 自主续签与终态重入控制未完成、LiveKit 与私有媒体/React 页面端到端未实测、通知全量在线重建/切换与归档调度未完成、千万连接和亿级数据未压测。

### 聊天首页 Web 接入（代码，不是上线验收）

`MLC_React/src/manager_antd/page_modules/home/hg_home_page.jsx` 已替换本地演示消息并接入 Go 权威会话、设备补拉、Centrifugo 指针、审核媒体与 LiveKit 单聊呼叫；`MLC_React/docs/hg_im_web_integration.md` 记录页面功能、准确性边界和隔离环境验收清单。Web 锁定 `centrifuge@5.8.0`、`livekit-client@2.22.0`，SeaweedFS 浏览器上传直接使用 Go 预签名 PUT，不将 S3 签名密钥或 Go JWT 交给对象存储。Go CORS 允许白名单 Origin 的 `Range` 预检、206 暴露 `Content-Range`，没有放宽 Origin 白名单。

上述 Web 接入已完成代码装配。本机 debug 已有 `000054` 和基础审核外部实例，但**未完成两用户浏览器真实收发及失败路径验收**，不能据此解除媒体/通话撤权、数据清理和容量上线阻断项。浏览器仅 ACK 上屏并经服务端补拉的消息；未展示的离线会话保留 pending，离线漏来电也没有权威列表接口，需要另行设计和验收。不要把 Web 构建或本机审核 API 调用当作端到端 IM 通过。

此前迭代未启动外部服务；本轮隔离 debug 的更新证据见下文。没有新增 Go 测试文件；编译/本机单附件审核不能代替浏览器发送和接收验收。

### 2026-10-31 本机 debug 增量核验

- `./scripts/ensure_debug_deps.sh debug`：迁移 `no change`，版本 `54 dirty=false`。Kafka 三节点就绪，首次 `make kafka-init` 因现存 4 个 crawler topic 各为 1 分区而中断；获授权将其增至 12 分区后重跑成功并检查 ISR 完整。独立 `mlc.im.messages.v1` 建为 12 分区、3 副本，后纳入 `make kafka-init` 并验证完整 ISR。**扩分区后 key 到分区映射可能变化，必须观察下游幂等与旧分区堆积。**
- SeaweedFS 4.51：隔离区上传身份不能写 sealed，审核身份可读隔离区、条件封存并读回；二次不同内容 `If-None-Match: *` 返回 412 且原字节不变。SeaweedFS 3.83 的同项检查失败（重复写 200），其本轮新建的容器与测试数据卷已定向清理，不能复用该版本。
- ClamAV 1.4.3 病毒库加载后对 EICAR 测试 ZIP 返回 `rejected`，封存对象 404；安全 PNG 与两秒 MP4 经真实 S3/ClamAV/NudeNet HTTP 审核返回 `ready`，同一 mediaId 重试保持 `ready`，SHA 不符拒绝。**把 EICAR 字符串附在 PNG 尾部曾被扫描器放行**，已增图片结构末尾检查，重新实测该样例为 `rejected`；旧的误封存测试对象已按内容 SHA256 精确匹配并清理。不能宣称扫描覆盖所有容器隐藏数据；模型没有违规图片基准集或召回率证据。
- Go 预签名上传至 SeaweedFS 4.51 成功；Centrifugo HTTP API 发布受理且 `/health` 可用。尝试另起 Go 主服务因 `8080`、`8081`、`9091` 被已有 VS Code debug 进程占用而失败；未操作该进程，尚未通过两用户浏览器验收。私有密钥仅在忽略的 `config/MLC.local.env` 和工程外权限受限配置，pre/prod 门禁未改。

### 2026-10-31 收尾验证记录

- 已格式化本轮修改的 Go 文件，检查暂存及工作区差异；验证范围排除 `TestNotes`。
- `go test -tags production . ./internal/modules/im/... ./internal/consumer/im ./internal/events/im ./internal/outbox ./internal/infrastructure/kafka ./internal/pkg/hg_router -count=1`：通过。IM 子包当前没有独立测试文件；Range、Twirp 删除确认、取令牌期间设备撤销/成员移除/过期、零行窗口更新及投递租约失效用例在既有根测试文件运行。仅用 `httptest` 与 `sqlmock`，不连接真实设施。首轮新增 SQL 替身将 JSON 列返回成 string 而非 MySQL 驱动的字节切片，修正夹具后通过。
- `go build -tags production -o /private/var/folders/2z/dxhnl1vd6jzdg_70q_2h00bh0000gn/T/opencode/mlc-go-im .`：通过，产物留在工程外，未启动。
- 迁移静态核对：up/down 的 7 张表及投递索引对应；检查发现 4 处枚举 COMMENT 仅含英文，已在未执行的 000054 草案中补齐中文，字段类型/默认值/索引保持原定义。队列领取/审核/通话过期/清理查询分别匹配 `idx_im_job_due`、`idx_im_attachment_review`、`idx_im_call_expiry`、`idx_im_call_cleanup`。没有实际执行 DDL、EXPLAIN 或验证生产索引。
- 网关模板可被 JSON 解析，关闭客户端自行订阅与发布；未使用 Centrifugo 二进制验证配置。路由与 API Guard 从同一 `RouteSpec` 构建；Start/Close 及初始化失败清理路径完成静态复核，尚无真实进程故障演练证据。

官方协议核对：
- https://centrifugal.dev/docs/server/authentication
- https://centrifugal.dev/docs/server/server_api
- https://centrifugal.dev/docs/server/engines
- https://docs.livekit.io/reference/other/roomservice-api.md
- https://docs.livekit.io/intro/basics/rooms-participants-tracks/webhooks-events.md
- https://github.com/livekit/livekit/blob/master/pkg/service/roommanager.go （`refreshToken`，当前源码协议核对，不代表已锁定部署版本）
