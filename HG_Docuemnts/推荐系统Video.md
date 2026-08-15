- [视频推荐系统设计](#视频推荐系统设计)
- [推荐模块文件分布](#推荐模块文件分布)

<br/>

***
<br/><br/><br/>
># <h1 id="视频推荐系统设计">视频推荐系统设计</h1>


B站（哔哩哔哩）的视频推荐，本质上不是简单的 SQL 查询：

> “找最近热门的视频，然后随机给用户展示”

而是一个**大规模推荐系统（Recommendation System）**：

```
用户画像
    +
视频画像
    +
实时行为
    +
历史兴趣
    +
机器学习模型
    ↓
召回（几千个候选视频）
    ↓
粗排（几百个）
    ↓
精排（几十个）
    ↓
重排（最终几十个展示）
    ↓
首页推荐列表
```

B站公开资料中也提到，其推荐系统基于用户数据、内容数据以及深度学习算法，对内容进行分类、排序和推荐。([美国证券交易委员会][1])

下面从工程角度拆解。

---

# 1. 用户打开B站首页，推荐视频如何产生？

例如：

用户 A 打开首页。

系统不会去 MySQL：

```sql
select *
from video
order by hot_score desc
limit 30;
```

因为：

* 视频库可能几十亿
* 每秒请求百万级
* 用户兴趣不同

这样无法满足。

真实流程：

```
                用户请求
                   |
                   |
             recommendation-service
                   |
       +-----------+------------+
       |                        |
 用户画像服务              视频特征服务
       |                        |
       +-----------+------------+
                   |
              召回系统
                   |
          生成候选视频集合
          (5000~10000条)
                   |
              排序模型
                   |
             Top 30
                   |
              返回APP
```

---

# 2. 第一阶段：用户画像(User Profile)

首先系统知道：

这个用户喜欢什么。

比如用户：

```
UID: 10001

观看历史:

Go语言教程       80分钟
Docker教程       60分钟
Linux服务器     40分钟

点赞:
Go微服务架构

收藏:
Kubernetes部署


跳过:
游戏视频
明星娱乐
```

系统生成：

## 用户兴趣向量

例如：

```
User Vector:

{
 "golang":0.92,
 "backend":0.88,
 "cloud_native":0.75,
 "docker":0.80,
 "game":0.05
}
```

这叫：

> 用户Embedding向量

机器学习里面：

```
用户
 ↓
神经网络
 ↓
[0.21,0.54,0.87....]
```

---

# 3. 第二阶段：视频画像(Video Profile)

每一个视频上传后，会生成视频特征。

例如：

视频：

《Go语言百万连接服务器设计》

数据库：

video表：

```
video_id

title

category

tags

author_id

duration

create_time
```

但是推荐系统需要更多：

```
Video Feature:

{
 category:"backend",

 tags:[
   "golang",
   "network",
   "high concurrency"
 ],

 author_level:8,

 quality_score:0.92,

 hot_score:0.85
}
```

包括：

## 内容特征

### 文本

标题：

```
Go高并发服务器设计
```

拆词：

```
Go
服务器
网络
高并发
```

---

## 图片特征

封面：

AI分析：

```
技术类封面
代码截图
人物
文字比例
```

---

## 视频特征

例如：

```
平均观看时间

完播率

弹幕数量

评论数量

收藏数量

投币数量
```

---

# 4. 第三阶段：视频召回(Candidate Recall)

这是推荐最核心的一步。

假设B站有：

```
10亿视频
```

不可能全部计算。

所以先找：

```
10000个候选视频
```

召回一般多路。

---

## 召回方式1：兴趣召回

用户喜欢 Go。

找到：

```
tag=golang

tag=backend

tag=distributed-system
```

得到：

```
1000条
```

---

## 召回方式2：协同过滤

类似：

> 和你兴趣相似的人看了什么

例如：

用户A：

```
看:
Go
Docker
K8s
```

用户B：

```
看:
Go
Docker
K8s
etcd
```

那么：

B看过的视频推荐给A。

---

## 召回方式3：关注关系

例如：

你关注：

```
某UP主
```

他发布：

```
新视频
```

优先推荐。

---

## 召回方式4：热门召回

例如：

最近：

```
AI

ChatGPT
大模型
```

爆火。

增加：

```
热门池
```

---

## 召回方式5：地域召回

例如：

台湾用户：

```
台湾新闻
台湾UP主
本地活动
```

---

最终：

```
兴趣召回 2000
热门召回 1000
关注召回 500
相似用户召回 2000
新视频探索 1000

合并:

6500个候选视频
```

---

# 5. 第四阶段：排序模型

现在有：

```
6500个视频
```

需要选：

```
30个
```

进入排序模型。

---

模型输入：

```
用户:

年龄
地区
历史观看
兴趣向量


视频:

标签
作者
质量


上下文:

时间
设备
网络
```

---

模型预测：

例如：

视频1:

```
点击概率:
0.85

观看完成概率:
0.75

互动概率:
0.60
```

综合：

```
score =
点击率 * 0.3
+
观看时长 * 0.3
+
互动率 * 0.2
+
收藏概率 *0.1
+
新鲜度 *0.1
```

排序：

```
视频A 0.92
视频B 0.88
视频C 0.84
...
```

取：

```
Top 100
```

---

# 6. 第五阶段：重排(Re-ranking)

最后还有规则。

因为机器模型可能：

连续推荐：

```
10个Go视频
```

用户会疲劳。

所以重排：

加入：

## 多样性

```
Go
Docker
AI
数据库
架构
```

混合。

---

## 去重

例如：

用户刚看：

```
Redis教程
```

不要马上：

```
Redis底层源码
Redis集群
Redis面试
```

连续出现。

---

## 作者限制

避免：

首页：

```
UP主A
UP主A
UP主A
UP主A
```

---

最终：

```
推荐列表:

1 Go高并发服务器
2 AI Agent开发
3 Kubernetes实战
4 Redis源码
5 上海程序员生活
...
```

---

# 7. 推荐系统依赖哪些数据？

## 用户行为表

类似：

user_behavior_log

```
id

user_id

video_id

action

duration

timestamp
```

action:

```
play
pause
like
coin
collect
comment
share
follow
```

例如：

```
10001
观看
BV123
1200秒
收藏
```

---

## 视频统计表

video_stat

```
video_id

view_count

like_count

coin_count

favorite_count

comment_count

share_count

avg_watch_time
```

---

## 用户兴趣表 Redis

实时：

```
user_interest:10001

{
golang:0.9,
docker:0.8,
ai:0.7
}
```

---

## 视频Embedding库

例如：

Milvus / Faiss

存：

```
video_id

[0.123,0.543,...]
```

用于：

向量搜索。

---

# 8. 面向亿万数据的B站级架构

大概：

```
                 App
                  |
             API Gateway
                  |
        Recommendation Service
                  |
        +---------+---------+
        |                   |
 User Profile          Video Feature
        |                   |
        +---------+---------+

              Recall Service

        +------+------+------+
        |      |      |
   ES搜索  Redis   Milvus
        |
    Candidate IDs

              |
        Ranking Model

              |
        ReRank

              |
          Result
```

数据链路：

```
用户观看视频

        |
        v

Kafka

        |

Flink实时计算

        |

更新:

Redis用户画像

        |

训练样本

        |

GPU训练模型

        |

上线模型
```

B站技术文章中也公开介绍过推荐模型的数据流，包括用户行为采集、特征计算、Kafka/Flink等近实时流程。([BestHub][2])

---

# 9. 如果自己用Go实现一个“小B站推荐系统”

可以简化：

## 数据库

MySQL:

```
video

video_tag

user_behavior
```

Redis:

```
user:interest:{uid}

video:hot
```

ES:

```
视频搜索
```

Milvus:

```
视频向量
```

服务：

```
recommend-service

    |
    |
    +-- recall
    |
    +-- rank
    |
    +-- cache
```

---

# 总结一句话

B站推荐视频不是“查询出来”，而是：

> **通过用户行为建立用户画像，通过视频内容建立视频画像，利用多路召回找到几千候选视频，再通过机器学习排序模型预测用户最可能喜欢的视频，最后经过去重、多样性、新鲜度调整生成首页推荐列表。**

对于你之前研究的 **Go实现B站级视频系统（视频、评论、关注、弹幕、Kafka、Redis）**，推荐系统应该独立成：

```
recommend-service
```

不要和：

```
video-service
user-service
comment-service
```

混在一起。它属于独立的大数据/机器学习业务域。


***
<br/><br/><br/>
> <h2 id="推荐模块文件分布">推荐模块文件分布</h2>

如果对标 **B站、抖音这种千万并发、亿级用户规模**，推荐系统绝对不能写在 `video-service` 或 `user-service` 里面，而应该作为**独立业务域（Domain Service）**存在：

```text
                    APP
                     |
              API Gateway
                     |
        +------------+-------------+
        |                          |
   video-service              recommend-service
        |                          |
        |                    推荐系统核心
        |
   视频元数据


```

推荐模块应该独立成：

```
recommend-service
```

这是大厂推荐系统的基本拆分方式。

---

# 一、整体微服务架构设计

类似 B站：

```
                         Client
                           |
                    API Gateway
                           |
        +------------------+------------------+
        |                  |                  |
   user-service      video-service      recommend-service
        |                  |                  |
        |                  |                  |
    用户信息           视频信息          推荐结果
        |                  |                  |
        +------------------+------------------+
                           |
                        Kafka
                           |
                  数据计算平台
                           |
                 +---------+---------+
                 |                   |
             Flink实时             离线训练
                 |                   |
            用户画像库          推荐模型库
```

---

# 二、recommend-service负责什么？

不要理解成：

```
recommend-service
    |
    查询视频
```

不是。

它负责：

## 1. 推荐请求入口

例如：

APP首页：

```
GET /api/v1/recommend/feed
```

请求：

```json
{
    "user_id":"u10001",
    "cursor":"xxx",
    "size":20
}
```

进入：

```
recommend-service
```

---

# 三、recommend-service内部模块设计

推荐服务内部建议拆成：

```
recommend-service

├── api
│   └── feed_handler.go
│
├── recall
│   ├── user_recall.go
│   ├── hot_recall.go
│   ├── tag_recall.go
│   ├── vector_recall.go
│
├── rank
│   ├── rank.go
│   └── model.go
│
├── rerank
│   └── rerank.go
│
├── feature
│   ├── user_feature.go
│   └── video_feature.go
│
├── cache
│   └── redis.go
│
└── client
    ├── video_client.go
    └── user_client.go
```

---

# 四、每个模块职责

## 1. API层

负责：

```
用户请求推荐
```

例如：

```go
func RecommendFeed(
    ctx context.Context,
    uid string,
) ([]Video, error)
```

不要在这里写推荐算法。

---

# 2. Recall召回模块

负责：

> 从海量视频中找到可能喜欢的视频。

例如：

数据库：

10亿视频。

召回：

```
10亿

↓

5000
```

代码：

```
recall/
```

里面：

---

## 用户兴趣召回

根据：

```
用户最近观看
点赞
收藏
搜索
```

Redis:

```
user_interest:u10001

{
golang:0.9,
docker:0.8
}
```

找到：

```
tag=golang
```

---

## 热门召回

Redis:

```
video_hot_rank
```

例如：

```
BV001
1000000播放

BV002
800000播放
```

---

## 向量召回

大厂：

Milvus/Faiss。

例如：

用户向量：

```
[0.23,0.56,0.89]
```

搜索：

相似视频。

---

# 五、Rank排序模块

召回：

```
5000
```

排序：

```
5000

↓

100
```

这里才使用机器学习。

例如：

模型：

```
DeepFM
DIN
Transformer
```

输入：

```
用户特征

+
视频特征

+
上下文
```

输出：

```
score
```

例如：

```json
{
video:"BV123",
score:0.923
}
```

---

# 六、ReRank重排

排序以后：

```
100
```

最终：

```
20
```

处理：

## 去重

不要：

```
Go教程1
Go教程2
Go教程3
Go教程4
```

---

## 多样性

调整：

```
技术
娱乐
生活
新闻
```

比例。

---

## 新视频扶持

例如：

新UP主：

提高曝光。

---

# 七、视频服务和推荐服务如何交互？

## video-service

负责：

```
视频是什么
```

例如：

接口：

```
GET /videos/{id}
```

返回：

```json
{
"id":"BV123",
"title":"Go高并发",
"author":"10001"
}
```

---

## recommend-service

负责：

```
为什么推荐它
```

返回：

```json
[
{
 video_id:"BV123",
 reason:"你喜欢Go相关内容"
}
]
```

然后：

gateway:

批量调用：

```
recommend-service

        |
        |
 video-service
```

获取完整信息。

---

# 八、用户行为如何进入推荐系统？

例如：

用户：

观看视频：

```
BV100
```

流程：

```
APP

 |
 |
behavior-service

 |
 |
Kafka

 |
 |
Flink

 |
 +-------------+
 |             |
Redis       Feature DB

 |
 |
recommend-service
```

---

Kafka消息：

```json
{
"user_id":"10001",
"video_id":"BV100",
"action":"watch",
"duration":320
}
```

---

# 九、数据库设计

## 视频服务数据库

video_db:

```
video

id
title
author_id
url
```

---

## 推荐数据库

recommend_db:

用户兴趣：

```
user_interest


user_id

tag

weight
```

例如：

```
10001

golang

0.92
```

---

视频特征：

```
video_feature


video_id

tag

quality_score

embedding
```

---

行为日志：

不要MySQL。

使用：

```
Kafka
+
ClickHouse
```

---

# 十、Go工程目录建议

如果你使用Go：

```
go-bilibili/


services/

├── gateway

├── user-service

├── video-service

├── comment-service

├── follow-service

├── like-service


└── recommend-service
        |
        ├── cmd
        │    └── main.go
        |
        ├── internal
        │
        │── api
        │
        │── recall
        │
        │── rank
        │
        │── feature
        │
        │── repository
        │
        │── cache
        │
        │── kafka
        |
        └── model
```

---

# 十一、B站/抖音级别最终形态

真实大厂会进一步拆：

```
recommend-system

├── online-recommend-service
│        在线推荐
│
├── recall-service
│        召回
│
├── rank-service
│        排序
│
├── feature-service
│        特征
│
├── model-service
│        模型推理
│
├── behavior-service
│        行为采集
│
└── data-platform
         |
         |
      Flink
      Spark
      Hive
```

---

所以如果你现在设计一个**Go版B站架构**：

推荐采用：

```
gateway
   |
recommend-service
   |
+--------------+
|              |
video-service  user-service

Kafka
 |
behavior-service
 |
Flink
 |
Redis + Feature DB
```

这是从小规模架构平滑演进到 B站/抖音级推荐系统的正确拆分方式。你之前设计的视频、评论、关注、弹幕系统，都应该把推荐系统作为一个独立服务接入。

---

# 十二、当前工程实现记录

## 修改了哪些文件

新增推荐系统模块：

- `internal/modules/video_recommend/model/hg_video_recommend_model.go`
- `internal/modules/video_recommend/dto/hg_video_recommend_dto.go`
- `internal/modules/video_recommend/cache/hg_video_recommend_cache.go`
- `internal/modules/video_recommend/repository/hg_video_recommend_repository.go`
- `internal/modules/video_recommend/service/hg_video_recommend_service.go`
- `internal/modules/video_recommend/handler/hg_video_recommend_handler.go`
- `internal/modules/video_recommend/module/hg_video_recommend_assembly.go`
- `internal/modules/video_recommend/module/hg_video_recommend_module.go`

新增路由、SQL 和测试：

- `internal/pkg/hg_router/hg_route_video_recommend.go`
- `internal/pkg/hg_router/hg_video_recommend_route_catalog_test.go`
- `internal/pkg/mysql/queries/hg_video_recommend_queries.go`
- `internal/pkg/config/hg_video_recommend_config_test.go`
- `internal/modules/video_recommend/service/hg_video_recommend_service_test.go`
- `internal/modules/video_recommend/repository/hg_video_recommend_repository_test.go`

修改接入文件：

- `main_mlc_project.go`
- `hg_kafka_application.go`
- `config/base/app.yaml`
- `internal/infrastructure/kafka/hg_runtime.go`
- `internal/pkg/config/hg_env_config.go`
- `internal/pkg/config/hg_api_gateway_config_test.go`
- `internal/pkg/hg_router/hg_api_gateway.go`
- `internal/pkg/hg_router/hg_route_groups.go`
- `internal/pkg/server/hg_router_rules.go`
- `internal/pkg/redis/hg_redis_key.go`

## 做了什么改动

新增认证推荐接口：

```text
GET /api/v1/video_recommend/feed?cursor=<opaque>&pageSize=20
```

核心链路：

```text
JWT 用户
  -> 64 分片 Redis Feed 召回
  -> 有界候选归并
  -> Redis MGET 视频卡片
  -> MySQL 唯一键批量冷回源
  -> Redis Pipeline 互动计数
  -> 当前页作者/分区多样性重排
  -> 复合游标返回
```

具体能力：

- 用户身份仅从 JWT context 获取，客户端不能指定 `userId`。
- 复用现有 `video.published -> Kafka -> Redis ZSET` 投影。
- Feed 按 64 个 Redis ZSET 分片读取，单次 Pipeline 完成网络请求。
- 使用发布时间 score 与 `submission_id` 组成稳定游标，禁止 offset 深分页。
- 单页默认 20 条，最大 50 条。
- 候选和 SQL 批次均有硬上限。
- 视频卡片通过 Redis `MGET` 批量读取。
- 缓存冷缺失只执行一次 MySQL `IN` 批量查询。
- SQL 再次限定 `status='published'`、`visibility='public'`。
- 互动计数通过单次 Redis Pipeline 批量获取。
- 卡片缓存 TTL 带抖动，降低缓存雪崩风险。
- Redis/MySQL 异常快速返回标准 `503`，不会降级为主表扫描。
- stale、删除或下架候选会推进游标，避免反复读取同一个无效候选。
- Kafka Feed 写侧和在线读侧共享 generation、分片数和容量配置。
- 推荐模块已加入 API Gateway 令牌桶、单实例并发舱壁和路由目录。

## 为什么这样改

当前项目已经有成熟的 Kafka Feed 写侧，重新建设一套召回存储会造成重复链路和一致性风险。因此首版直接补齐在线读侧：

- 亿级视频表不执行实时排序、统计或全表扫描。
- Redis 分片避免单个全局 ZSET 成为热点。
- Pipeline/MGET 避免候选数量放大为大量网络往返。
- MySQL 只承担缓存冷缺失的唯一键批量补全。
- 复合游标适合高写入、高数据量场景，不存在深分页退化。
- 重排限制在已经选中的当前页内，避免跨页候选被游标跳过。
- 固定候选窗口和请求超时，防止请求期 CPU、内存和连接池无界增长。

## 准确性检查结果

已检查：

- Feed 读写双方 generation、shard count、max items 配置一致。
- SQL 使用 `submission_id` 唯一索引批量点查。
- SQL 严格过滤已发布且公开的视频。
- 没有循环内 MySQL/Redis 单条查询。
- 没有无界 goroutine、channel、候选集合或分页尺寸。
- 游标可以跨同分值候选稳定翻页。
- 无效候选不会导致分页永久卡住。
- 重排不会改变游标扫描边界。
- API Gateway 白名单、配置数量和路由目录已经同步。
- 未新增第三方依赖和数据库迁移。

## 潜在影响

- `api_gateway.modules` 从 8 个模块增加到 9 个模块，部署环境如果覆盖了完整 `api_gateway` 配置，也必须增加 `video_recommend`。
- 当前 Redis 基础设施使用 `*redis.Client`，不是 `ClusterClient`。代码的 Feed key 已按 cluster slot 设计，但真正大规模生产部署仍需基础设施层切换 Redis Cluster。
- 当前推荐属于“高并发新鲜内容召回 + 互动特征 + 规则重排”，尚未具备抖音/B站完整的用户行为画像、向量召回和模型精排能力。
- MySQL 卡片冷回源适合作为过渡方案。极高流量下应由 Kafka 构建完整 Redis 视频卡片读模型。
- `HG_Docuemnts/推荐系统Video.md` 在实现前是未跟踪文件，本次随推荐系统代码一并纳入版本控制。

## 格式化/编译/测试说明

已执行并通过：

```text
gofmt：本次全部新增和修改的 Go 文件
go test ./internal/modules/video_recommend/... -count=1
go test -race ./internal/modules/video_recommend/...
go test -race ./internal/consumer/feed
go test ./internal/pkg/hg_router ./internal/pkg/server ./internal/pkg/config ./internal/consumer/feed ./internal/infrastructure/kafka -count=1
go build -o /var/folders/2z/dxhnl1vd6jzdg_70q_2h00bh0000gn/T/opencode/mlc-go .
```

扩大验证执行了：

```text
go test ./internal/... -count=1
```

新增推荐模块及绝大多数内部包通过，但整体验证存在仓库既有失败：

- `internal/modules/user/middleware`
- `internal/pkg/middleware`

失败原因是既有 JWT 测试期望错误码 `101001`，当前实现实际返回 `300001`。本次未修改 JWT、中间件或错误码逻辑，与推荐模块无关。

## 后续生产级优化建议

1. 建设曝光、播放时长、完播、跳过等行为事件，进入 Kafka、Flink 和 ClickHouse。
2. 通过 Kafka 建立完整视频卡片 Redis 读模型，彻底移除在线 MySQL 冷回源。
3. 增加用户兴趣、关注、热门、地域、新视频和协同过滤多路召回。
4. 接入 Milvus/Faiss 和模型推理服务，形成粗排、精排和实验分桶。
5. 将 Redis 客户端演进为 ClusterClient，并按容量规划拆分 Feed、特征和缓存集群。
6. 增加推荐接口压测，验证 P50/P95/P99、Redis commands/request、缓存命中率、拒绝率和实例资源上限。
