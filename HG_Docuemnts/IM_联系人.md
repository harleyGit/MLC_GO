# Go 即时通信系统：好友关系 + 单聊 + 群组完整设计方案

这次是在现有 Go + Centrifugo + LiveKit + SeaweedFS + React 即时通信系统基础上，增加一套类似微信的社交关系体系。

这不是简单增加一个好友列表和群聊页面，而是要建立完整的：

1. 用户身份体系（系统 ID、自定义 ID）。
2. 好友申请与双向好友关系。
3. 双向好友验证后的单聊权限。
4. 群组创建、邀请、申请加入、成员管理。
5. 单聊和群聊统一消息系统。
6. 联系人搜索、好友列表、黑名单。
7. 消息权限校验、实时推送、历史消息查询。
8. 与现有 Centrifugo、LiveKit、SeaweedFS 集成。

最重要的业务规则：

- A 添加 B，不代表 A 和 B 已经是好友。
- B 接受 A 的好友申请后，双方才建立好友关系。
- 只有双方互为好友，才允许建立普通单聊。
- 群聊不要求群成员之间互为好友。
- 群组成员关系由群组成员表控制，不能用好友关系代替。
- 好友关系、群成员关系和消息权限必须由 Go 服务端统一控制，不能只依赖 React 前端隐藏按钮。

## 一、整体技术架构：在现有架构上扩展

不推翻已有即时通信方案，保留现有技术选型和消息链路。

### 1.1 技术栈

| 模块   | 技术         | 职责               |
| ---- | ---------- | ---------------- |
| 后端   | Go         | 用户关系、好友、群组、消息业务  |
| 实时通信 | Centrifugo | WebSocket、消息实时推送 |
| 音视频  | LiveKit    | 一对一、群组音视频通话      |
| 文件存储 | SeaweedFS  | 图片、视频、文件、语音      |
| 主数据库 | MySQL 8    | 用户关系、会话、群组、消息    |
| 缓存   | Redis      | 在线状态、未读数、限流、临时状态 |
| 消息队列 | Kafka      | 消息事件、异步通知、业务事件   |
| 前端   | React      | 联系人、聊天、群组、消息中心   |

### 1.2 整体模块关系

```
                    React 前端
                        |
          +-------------+-------------+
          |             |             |
       联系人模块     消息模块       群组模块
          |             |             |
          +-------------+-------------+
                        |
                    Go API
                        |
      +-----------------+------------------+
      |                 |                  |
  User Service      Contact Service     Group Service
  用户身份服务       好友关系服务         群组服务
      |                 |                  |
      +-----------------+------------------+
                        |
                  Conversation Service
                     会话管理
                        |
                   Message Service
                     消息管理
                        |
              MySQL + Transactional Outbox
                        |
                      Kafka
                        |
                 Event Consumer
                        |
                   Centrifugo
                        |
                 React 实时接收
```

外围服务：

```
       Go Backend
          |
    +-----+------+
    |            |
  LiveKit     SeaweedFS
  音视频        文件存储
```

### 1.3 需要增加的业务模块

在原有 IM 模块中新增：

```
internal/
├── user/
│   ├── profile/           # 用户资料
│   └── identity/          # 用户身份查询
│
├── contact/
│   ├── service.go         # 联系人业务入口
│   ├── search.go          # ID/手机号/邮箱搜索
│   ├── request.go         # 好友申请
│   ├── friendship.go      # 好友关系
│   ├── blacklist.go       # 黑名单
│   └── repository.go
│
├── group/
│   ├── service.go         # 群组业务
│   ├── create.go          # 创建群组
│   ├── member.go          # 成员管理
│   ├── invitation.go      # 邀请入群
│   ├── join_request.go    # 申请入群
│   ├── permission.go      # 群权限
│   └── repository.go
│
├── conversation/
│   ├── service.go
│   ├── authorization.go   # 会话权限
│   ├── direct.go          # 单聊
│   └── repository.go
│
├── message/               # 保留原有消息模块
├── realtime/              # 保留 Centrifugo
├── media/                 # 保留 SeaweedFS
└── call/                  # 保留 LiveKit
```

核心设计原则：

Contact 管理人与人的关系，Group 管理群组及成员，Conversation 管理聊天会话，Message 管理聊天内容。

不要将好友、群组和消息全部写进一个 Service。

# 二、用户身份体系：支持四种方式添加好友

这是第一个需要设计好的基础模块。

你的需求是：

用户可以通过以下方式找到其他用户：

1. 系统用户 ID。
2. 手机号。
3. 邮箱。
4. 用户自己设置的 ID（类似微信号）。

这里需要区分两个概念。

### 2.1 系统 ID 和自定义 ID

例如：

张三

用户资料示例

| 属性      | 值                     |
| ------- | --------------------- |
| 系统用户 ID | 100001                |
| 自定义 ID  | zhangsan888           |
| 手机号     | 138\*\*\*\*8888       |
| 邮箱      | zhangsan\@example.com |

它们的作用不同：

| 类型        | 特点         | 是否允许修改  |
| --------- | ---------- | ------- |
| user_id   | 数据库内部用户主键  | 不允许     |
| public_id | 对外展示的系统 ID | 不允许     |
| username  | 用户自己设置的 ID | 可以按规则修改 |
| phone     | 手机号        | 可以更换    |
| email     | 邮箱         | 可以更换    |

推荐：

- `user_id`：内部数据库主键。
- `public_id`：对外公开的唯一用户编号。
- `username`：用户自定义的唯一 ID。

例如：

```
user_id  = 100001
public_id = U202610310001
username  = zhangsan888
```

注意：不能将手机号、邮箱直接作为数据库用户主键。

### 2.2 用户资料表

```
CREATE TABLE im_user_profile (
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户内部ID',

    public_id VARCHAR(32) NOT NULL COMMENT '公开系统ID',

    username VARCHAR(32) DEFAULT NULL COMMENT '用户自定义ID',

    nickname VARCHAR(64) NOT NULL DEFAULT '' COMMENT '昵称',

    avatar_url VARCHAR(512) DEFAULT NULL COMMENT '头像',

    signature VARCHAR(256) DEFAULT NULL COMMENT '个性签名',

    discoverable_by_phone TINYINT NOT NULL DEFAULT 1
        COMMENT '是否允许手机号搜索',

    discoverable_by_email TINYINT NOT NULL DEFAULT 1
        COMMENT '是否允许邮箱搜索',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (user_id),

    UNIQUE KEY uk_public_id (public_id),

    UNIQUE KEY uk_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

手机号和邮箱建议单独放入用户身份表，而不是将明文直接放在公开资料表中。

生产环境建议：

```
im_user_identity

identity_id
user_id
identity_type       phone / email
identity_value_enc  加密后的原始值
identity_hash       HMAC-SHA256
verified_at
created_at
```

其中：

- 手机号、邮箱加密保存。
- 使用 HMAC-SHA256 生成查询索引。
- 只有验证通过的手机号、邮箱才能用于添加好友。
- 查询接口进行限流，防止批量枚举用户手机号和邮箱。

### 2.3 用户搜索接口

```
POST /api/im/contacts/search
Authorization: Bearer JWT
Content-Type: application/json
```

请求：

```
{
  "type": "username",
  "keyword": "zhangsan888"
}
```

支持：

```
{
  "type": "public_id",
  "keyword": "U202610310001"
}
```

```
{
  "type": "phone",
  "keyword": "13800138000"
}
```

```
{
  "type": "email",
  "keyword": "zhangsan@example.com"
}
```

返回：

```
{
  "userId": "100001",
  "publicId": "U202610310001",
  "username": "zhangsan888",
  "nickname": "张三",
  "avatar": "https://cdn.example.com/avatar/100001.jpg",
  "friendStatus": "none"
}
```

`friendStatus` 可以是：

```
none              不是好友
outgoing_pending  我已申请
incoming_pending  对方已申请
friend            已经是好友
blocked           已被拉黑或已拉黑对方
```

搜索用户和建立好友关系是两个独立操作。

# 三、好友关系设计：必须双向同意才能单聊

这是整个社交体系最核心的业务规则。

## 3.1 好友关系状态机

例如：

A 添加 B。

```
             A 搜索 B
                 |
                 v
             发送申请
                 |
                 v
          等待 B 确认申请
                 |
          +------+------+
          |             |
          v             v
        B 拒绝         B 接受
          |             |
          v             v
       申请拒绝       建立好友
                        |
                        v
                  创建单聊会话
                        |
                        v
                    允许聊天
```

这里有一个重要细节：

A 发送好友申请之后，A 和 B 仍然不是好友。

只有 B 接受申请，好友关系才正式建立。

## 3.2 好友关系数据库设计

建议拆成两个表：

- `im_friend_request`：保存申请记录和申请历史。
- `im_friendship`：保存当前有效关系。

### 表一：好友申请表

```
CREATE TABLE im_friend_request (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    request_id VARCHAR(64) NOT NULL COMMENT '申请业务ID',

    requester_id BIGINT UNSIGNED NOT NULL COMMENT '申请人',

    addressee_id BIGINT UNSIGNED NOT NULL COMMENT '被申请人',

    message VARCHAR(128) NOT NULL DEFAULT ''
        COMMENT '好友申请附言',

    status TINYINT NOT NULL DEFAULT 0
        COMMENT '0待处理 1已接受 2已拒绝 3已撤回 4已过期',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_request_id (request_id),

    KEY idx_addressee_status (
        addressee_id, status, created_at
    ),

    KEY idx_requester (
        requester_id, created_at
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 表二：好友关系表

```
CREATE TABLE im_friendship (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    user_low_id BIGINT UNSIGNED NOT NULL COMMENT '较小用户ID',

    user_high_id BIGINT UNSIGNED NOT NULL COMMENT '较大用户ID',

    requester_id BIGINT UNSIGNED NOT NULL COMMENT '最初申请人',

    addressee_id BIGINT UNSIGNED NOT NULL COMMENT '最初被申请人',

    status TINYINT NOT NULL DEFAULT 0
        COMMENT '0待确认 1已成为好友',

    accepted_at DATETIME(3) DEFAULT NULL COMMENT '接受时间',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_user_pair (user_low_id, user_high_id),

    KEY idx_requester (requester_id),

    KEY idx_addressee (addressee_id)

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

为什么要设计 `user_low_id` 和 `user_high_id`？

因为好友关系是双向的。

例如：

```
A = 100001
B = 100002
```

无论是 A 添加 B，还是 B 添加 A，最终都存储：

```
user_low_id  = 100001
user_high_id = 100002
```

这样可以通过唯一索引：

```
UNIQUE KEY uk_user_pair (user_low_id, user_high_id)
```

保证同一对用户不会重复创建好友关系。

### 3.3 添加好友的业务逻辑

Go 接口：

```
POST /api/im/contacts/requests
```

请求：

```
{
  "targetUserId": "100002",
  "message": "你好，可以认识一下吗？"
}
```

核心代码：

```
func (s *ContactService) SendFriendRequest(
    ctx context.Context,
    requesterID uint64,
    addresseeID uint64,
    message string,
) error {

    if requesterID == addresseeID {
        return ErrCannotAddSelf
    }

    lowID := min(requesterID, addresseeID)
    highID := max(requesterID, addresseeID)

    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 锁定用户关系，防止并发重复申请。
    var status int

    err = tx.QueryRowContext(ctx, `
        SELECT status
        FROM im_friendship
        WHERE user_low_id = ?
          AND user_high_id = ?
        FOR UPDATE
    `, lowID, highID).Scan(&status)

    if err == nil {
        return ErrFriendshipAlreadyExists
    }

    if !errors.Is(err, sql.ErrNoRows) {
        return err
    }

    // 创建好友申请记录。
    requestID := uuid.NewString()

    _, err = tx.ExecContext(ctx, `
        INSERT INTO im_friend_request (
            request_id,
            requester_id,
            addressee_id,
            message,
            status
        ) VALUES (?, ?, ?, ?, 0)
    `, requestID, requesterID, addresseeID, message)

    if err != nil {
        return err
    }

    // 建立待确认的关系记录。
    _, err = tx.ExecContext(ctx, `
        INSERT INTO im_friendship (
            user_low_id,
            user_high_id,
            requester_id,
            addressee_id,
            status
        ) VALUES (?, ?, ?, ?, 0)
    `, lowID, highID, requesterID, addresseeID)

    if err != nil {
        return err
    }

    return tx.Commit()
}
```

这段是核心示意代码，生产实现还需要处理并发唯一键冲突、反向待处理申请和事务错误分类。

特别注意反向申请：

假设：

```
A -> B：已经发送申请，等待 B 同意。

此时 B -> A：也发送了申请。
```

不应该生成第二条待处理关系。

推荐策略：

```
发现同一对用户已有 pending：

如果现有申请方向与当前请求相反：
    自动将现有申请视为双方互相申请
    按业务规则自动建立好友关系

如果申请方向相同：
    返回已有申请状态
```

这样能够避免重复好友关系。

## 3.4 接受好友申请

接口：

```
POST /api/im/contacts/requests/{requestId}/accept
```

事务内完成：

```
1. 查询好友申请。
2. SELECT ... FOR UPDATE 锁定申请。
3. 验证当前用户是 addressee_id。
4. 验证申请状态为 pending。
5. 更新 im_friend_request.status = accepted。
6. 更新 im_friendship.status = accepted。
7. 创建或获取 SINGLE 会话。
8. 写入双方会话成员。
9. 写入 Outbox 事件。
10. COMMIT。
```

这里建议把第 7、8 步与好友接受操作放在同一个 MySQL 事务中。

这样能够保证：

```
好友建立成功
    =
好友关系建立成功 + 单聊会话创建成功
```

不会出现已经成为好友，却没有聊天会话的情况。

# 四、单聊设计：没有互为好友就不能聊天

## 4.1 单聊会话模型

沿用现有的：

```
im_conversation
im_conversation_member
im_message
```

不需要再单独设计一套单聊消息表。

在现有 `im_conversation` 上增加两个字段：

```
ALTER TABLE im_conversation
ADD COLUMN direct_user_low_id BIGINT UNSIGNED DEFAULT NULL
    COMMENT '单聊较小用户ID',
ADD COLUMN direct_user_high_id BIGINT UNSIGNED DEFAULT NULL
    COMMENT '单聊较大用户ID',
ADD UNIQUE KEY uk_direct_pair (
    direct_user_low_id,
    direct_user_high_id
);
```

其中：

```
type = SINGLE

direct_user_low_id  = 100001
direct_user_high_id = 100002
```

群组：

```
type = GROUP

direct_user_low_id  = NULL
direct_user_high_id = NULL
```

MySQL 唯一索引允许多个 NULL，因此不会影响多个群组会话。

## 4.2 消息发送必须验证好友关系

这是最重要的安全控制。

原有消息发送流程：

```
JWT 鉴权
   |
   v
验证会话成员
   |
   v
消息幂等校验
   |
   v
保存消息
   |
   v
Outbox
   |
   v
Kafka
   |
   v
Centrifugo
```

现在需要增加：

```
                  发送消息
                     |
                     v
               JWT 用户鉴权
                     |
                     v
                查询会话类型
                     |
              +------+------+
              |             |
              v             v
           SINGLE         GROUP
              |             |
              v             v
          双向好友校验    群成员校验
              |             |
              +------+------+
                     |
                     v
                 权限通过
                     |
                     v
                 消息幂等
                     |
                     v
                MySQL 持久化
                     |
                     v
              Outbox + Kafka
                     |
                     v
                 Centrifugo
```

Go 代码：

```
func (s *MessageService) CheckConversationPermission(
    ctx context.Context,
    tx *sql.Tx,
    conversationID string,
    senderID uint64,
) error {

    var conversationType string

    err := tx.QueryRowContext(ctx, `
        SELECT type
        FROM im_conversation
        WHERE conversation_id = ?
    `, conversationID).Scan(&conversationType)

    if err != nil {
        return err
    }

    switch conversationType {

    case "SINGLE":

        var lowID, highID uint64

        err = tx.QueryRowContext(ctx, `
            SELECT direct_user_low_id, direct_user_high_id
            FROM im_conversation
            WHERE conversation_id = ?
        `, conversationID).Scan(&lowID, &highID)

        if err != nil {
            return err
        }

        if senderID != lowID && senderID != highID {
            return ErrConversationForbidden
        }

        var exists bool

        err = tx.QueryRowContext(ctx, `
            SELECT EXISTS (
                SELECT 1
                FROM im_friendship
                WHERE user_low_id = ?
                  AND user_high_id = ?
                  AND status = 1
            )
        `, lowID, highID).Scan(&exists)

        if err != nil {
            return err
        }

        if !exists {
            return ErrFriendshipRequired
        }

    case "GROUP":

        // 群组必须检查当前用户是否为有效成员。
        var exists bool

        err = tx.QueryRowContext(ctx, `
            SELECT EXISTS (
                SELECT 1
                FROM im_conversation_member
                WHERE conversation_id = ?
                  AND user_id = ?
                  AND status = 1
            )
        `, conversationID, senderID).Scan(&exists)

        if err != nil {
            return err
        }

        if !exists {
            return ErrConversationForbidden
        }

    default:
        return ErrConversationTypeInvalid
    }

    return nil
}
```

这里的 `status = 1` 假设表示有效成员，实际应统一使用你现有表的状态定义。

注意：生产环境还要检查单聊双方是否互相拉黑、会话是否被禁用，以及群组是否已解散。

不要只在创建单聊时检查好友关系，必须在每次发送消息时检查。

因为好友关系可能发生变化。

# 五、群组设计：类似微信群

群组和好友有一个根本区别：

```
单聊：
A 和 B 必须互为好友。

群聊：
A、B、C、D 可以互相不是好友，
只要他们都是群成员，就可以在群内聊天。
```

## 5.1 群组整体模型

例如：

Go 技术交流群

群号：G202610310001 · 4 名成员

群主

张三

Owner

管理员

李四

Admin

普通成员

王五、赵六

Member

建议群组功能包含：

| 功能    | 说明          |
| ----- | ----------- |
| 创建群组  | 用户创建群聊      |
| 邀请成员  | 邀请其他用户加入    |
| 申请入群  | 用户主动申请加入    |
| 审核入群  | 群主或管理员审批    |
| 移除成员  | 群主或管理员踢人    |
| 退出群组  | 普通成员主动退出    |
| 转让群主  | 将群主权限交给其他成员 |
| 设置管理员 | 授予或撤销管理员    |
| 群公告   | 发布群公告       |
| 群名称   | 修改群名称       |
| 禁言    | 禁止指定成员发言    |
| 群消息   | 群内消息通信      |
| 解散群组  | 群主解散群组      |

## 5.2 群组数据库

建议增加 `im_group`，但是继续复用现有 `im_conversation_member`，不要再维护第二套群成员表。

### 群组表

```
CREATE TABLE im_group (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    group_id VARCHAR(64) NOT NULL COMMENT '群组业务ID',

    conversation_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',

    owner_user_id BIGINT UNSIGNED NOT NULL COMMENT '群主用户ID',

    group_name VARCHAR(128) NOT NULL COMMENT '群名称',

    avatar_url VARCHAR(512) DEFAULT NULL COMMENT '群头像',

    announcement TEXT COMMENT '群公告',

    join_policy TINYINT NOT NULL DEFAULT 1
        COMMENT '1需要审核 2允许直接加入',

    invite_policy TINYINT NOT NULL DEFAULT 1
        COMMENT '1管理员邀请 2所有成员可邀请',

    max_members INT UNSIGNED NOT NULL DEFAULT 500,

    status TINYINT NOT NULL DEFAULT 1
        COMMENT '1正常 2已解散',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_group_id (group_id),

    UNIQUE KEY uk_conversation_id (conversation_id),

    KEY idx_owner (owner_user_id, created_at)

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 群成员

复用：

```
im_conversation_member
```

增加角色字段：

```
ALTER TABLE im_conversation_member
ADD COLUMN role TINYINT NOT NULL DEFAULT 3
    COMMENT '1群主 2管理员 3普通成员',
ADD COLUMN joined_at DATETIME(3) DEFAULT NULL,
ADD COLUMN mute_until DATETIME(3) DEFAULT NULL;
```

状态建议统一：

```
status = 1   正常成员
status = 2   已退出
status = 3   已被移除
```

群成员数据：

| conversation_id | user_id | role   | status |
| --------------- | ------- | ------ | ------ |
| C1001           | 100001  | OWNER  | 1      |
| C1001           | 100002  | ADMIN  | 1      |
| C1001           | 100003  | MEMBER | 1      |
| C1001           | 100004  | MEMBER | 1      |

注意：`conversation_id` 是会话 ID，不是群组 ID。

群组通过 `im_group.conversation_id` 关联会话。

### 群申请表

```
CREATE TABLE im_group_join_request (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    request_id VARCHAR(64) NOT NULL,

    group_id VARCHAR(64) NOT NULL,

    user_id BIGINT UNSIGNED NOT NULL,

    message VARCHAR(128) NOT NULL DEFAULT '',

    status TINYINT NOT NULL DEFAULT 0
        COMMENT '0待审核 1通过 2拒绝 3撤回',

    reviewed_by BIGINT UNSIGNED DEFAULT NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    reviewed_at DATETIME(3) DEFAULT NULL,

    PRIMARY KEY (id),

    UNIQUE KEY uk_request_id (request_id),

    KEY idx_group_status (group_id, status, created_at),

    KEY idx_user (user_id, created_at)

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

群邀请也建议单独设计 `im_group_invitation`，用于记录邀请人、被邀请人、邀请状态、有效期和接受时间。

## 5.3 群组创建流程

```
用户 A 创建群组
       |
       v
生成 group_id
       |
       v
创建 GROUP 类型 conversation
       |
       v
创建 im_group
       |
       v
将 A 添加到 conversation_member
       |
       v
设置 A 为 OWNER
       |
       v
提交事务
       |
       v
群组创建成功
```

群组创建、会话创建和群主入群必须放在同一个事务。

## 5.4 邀请用户入群

```
POST /api/im/groups/{groupId}/invitations
```

请求：

```
{
  "userIds": [
    "100002",
    "100003",
    "100004"
  ]
}
```

业务校验：

```
1. 验证邀请人是群成员。
2. 检查邀请权限。
3. 检查群组是否正常。
4. 检查目标用户是否已在群内。
5. 检查群人数上限。
6. 根据邀请策略直接加入或等待确认。
7. 写入成员记录。
8. 生成群成员变更事件。
9. 推送群成员变更通知。
```

群成员加入后：

```
A、B、C、D
     |
     v
GROUP Conversation
     |
     v
conversation_id = C1001
     |
     v
所有有效成员都可以收发群消息
```

群成员之间不需要成为好友。

# 六、联系人模块：完整 API 设计

建议把联系人相关接口统一放在：

```
/api/im/contacts
```

### 6.1 用户搜索

| Method | API                          | 说明            |
| ------ | ---------------------------- | ------------- |
| POST   | `/contacts/search`           | 按 ID、手机号、邮箱搜索 |
| GET    | `/contacts/profile/{userId}` | 查询用户公开资料      |

### 6.2 好友申请

| Method | API                              | 说明      |
| ------ | -------------------------------- | ------- |
| POST   | `/contacts/requests`             | 发送好友申请  |
| GET    | `/contacts/requests/incoming`    | 收到的好友申请 |
| GET    | `/contacts/requests/outgoing`    | 发出的好友申请 |
| POST   | `/contacts/requests/{id}/accept` | 接受申请    |
| POST   | `/contacts/requests/{id}/reject` | 拒绝申请    |
| DELETE | `/contacts/requests/{id}`        | 撤回申请    |

### 6.3 好友管理

| Method | API                                 | 说明     |
| ------ | ----------------------------------- | ------ |
| GET    | `/contacts/friends`                 | 好友列表   |
| GET    | `/contacts/friends/{userId}`        | 好友资料   |
| DELETE | `/contacts/friends/{userId}`        | 删除好友   |
| PATCH  | `/contacts/friends/{userId}/remark` | 修改好友备注 |
| POST   | `/contacts/blocks/{userId}`         | 拉黑用户   |
| DELETE | `/contacts/blocks/{userId}`         | 解除拉黑   |
| GET    | `/contacts/blocks`                  | 黑名单列表  |

这里建议增加一张好友备注表：

```
CREATE TABLE im_friend_remark (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    owner_user_id BIGINT UNSIGNED NOT NULL,

    friend_user_id BIGINT UNSIGNED NOT NULL,

    remark VARCHAR(64) NOT NULL DEFAULT '',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_owner_friend (
        owner_user_id,
        friend_user_id
    )

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

为什么备注需要单独一张表？

因为好友备注是单向的：

```
A 给 B 设置备注：项目经理

B 给 A 设置备注：老同学
```

双方的备注不能互相覆盖。

### 6.4 群组管理

统一使用：

```
/api/im/groups
```

| Method | API                                            | 说明     |
| ------ | ---------------------------------------------- | ------ |
| POST   | `/groups`                                      | 创建群组   |
| GET    | `/groups`                                      | 我的群组列表 |
| GET    | `/groups/{groupId}`                            | 群组详情   |
| PATCH  | `/groups/{groupId}`                            | 修改群组资料 |
| DELETE | `/groups/{groupId}`                            | 解散群组   |
| GET    | `/groups/{groupId}/members`                    | 群成员列表  |
| POST   | `/groups/{groupId}/invitations`                | 邀请成员   |
| POST   | `/groups/{groupId}/join-requests`              | 申请加入群  |
| GET    | `/groups/{groupId}/join-requests`              | 查看入群申请 |
| POST   | `/groups/{groupId}/join-requests/{id}/approve` | 同意入群   |
| POST   | `/groups/{groupId}/join-requests/{id}/reject`  | 拒绝入群   |
| DELETE | `/groups/{groupId}/members/{userId}`           | 移除成员   |
| DELETE | `/groups/{groupId}/members/me`                 | 退出群组   |
| PATCH  | `/groups/{groupId}/members/{userId}/role`      | 设置管理员  |
| POST   | `/groups/{groupId}/transfer-owner`             | 转让群主   |
| PATCH  | `/groups/{groupId}/announcement`               | 修改群公告  |

建议统一返回格式：

```
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

# 七、Centrifugo 实时通信：如何让好友和群组收到消息

这是和你现有即时通信系统衔接的关键部分。

## 7.1 继续沿用现有 Channel 设计

原有设计：

```
user:{userId}
conversation:{conversationId}
group:{groupId}
system:{userId}
```

建议进一步规范：

| Channel             | 用途      |
| ------------------- | ------- |
| `user:{id}`         | 用户个人事件  |
| `conversation:{id}` | 单聊和群聊消息 |
| `system:{id}`       | 系统通知    |

这里建议统一以 `conversation:{id}` 作为聊天消息通道。

不要为单聊和群聊分别建立两套消息发布机制。

因为两者本质上都是 Conversation。

### 7.2 好友申请的实时通知

例如 A 给 B 发送好友申请。

```
A
|
| POST /contacts/requests
|
v
Go Contact Service
|
| 保存申请记录
|
v
MySQL + Outbox
|
v
Kafka
|
v
Centrifugo
|
| 发布到 user:100002
|
v
B 的 React 客户端
|
v
收到好友申请通知
```

B 收到：

```
{
  "event": "contact.friend_request.created",
  "data": {
    "requestId": "FR202610310001",
    "requesterId": "100001",
    "nickname": "张三",
    "avatar": "https://cdn.example.com/a.jpg",
    "message": "你好，可以认识一下吗？"
  }
}
```

B 点击接受：

```
POST /contacts/requests/FR202610310001/accept
```

服务端处理成功后，分别通知 A 和 B：

```
user:100001
user:100002
```

两边的好友列表和会话列表同步更新。

## 7.3 群消息实时推送

例如群组：

```
group_id = G1001
conversation_id = C1001
```

群内有：

```
A、B、C、D
```

四个用户均订阅：

```
conversation:C1001
```

A 发送消息：

```
A
 |
 v
Go Message Service
 |
 v
验证 A 是群成员
 |
 v
MySQL 持久化
 |
 v
Kafka
 |
 v
Centrifugo
 |
 v
conversation:C1001
 |
 +------ B
 |
 +------ C
 |
 +------ D
```

注意：

群组成员身份必须由 Go 服务端校验，而不是让 React 自己决定能否订阅群组频道。

## 7.4 Centrifugo 权限设计

建议：

```
React
  |
  | 获取连接 JWT
  v
Go /api/im/realtime/token
  |
  v
Centrifugo
  |
  | 订阅 conversation:C1001
  v
Go 后端验证当前用户是否有权限
```

具体可使用 Centrifugo 的 Subscription JWT 或 Subscribe Proxy。

对于动态群成员关系，建议由后端签发短有效期的订阅令牌，并在移除成员时主动撤销对应订阅，避免成员已经退出群组却仍持有有效订阅权限。

Centrifugo 官方提供了用户受限频道、订阅令牌和 Subscribe Proxy 等权限控制机制。尤其需要注意，频道订阅权限和发布权限是两套独立权限，不能只依赖频道名称进行安全控制。

[image](https://www.google.com/s2/favicons?domain=https://centrifugal.dev\&sz=32)

Centrifugo+1



推荐配置原则：

```
客户端：
    禁止直接发布正式聊天消息
    禁止任意订阅群组频道
    禁止任意访问其他用户频道

Go 服务端：
    验证好友关系
    验证群成员身份
    持久化消息
    发布 Centrifugo 事件
```

客户端发送消息始终走：

```
POST /api/im/messages
```

而不是直接调用 Centrifugo 的 publish。

# 八、好友关系变化如何影响聊天权限？

这是生产环境中容易遗漏的地方。

你需要提前定义以下业务规则。

| 操作            | 好友关系 | 是否允许继续发送单聊 |
| ------------- | ---- | ---------- |
| A 申请 B        | 待确认  | 否          |
| B 接受申请        | 已建立  | 是          |
| A 删除 B        | 已解除  | 否          |
| B 删除 A        | 已解除  | 否          |
| A 拉黑 B        | 被限制  | 否          |
| B 拉黑 A        | 被限制  | 否          |
| A 重新添加 B，B 接受 | 已恢复  | 是          |

### 8.1 删除好友

推荐：

```
删除好友
   |
   v
删除有效 friendship 记录
   |
   v
保留历史申请记录
   |
   v
保留历史聊天记录
   |
   v
禁止双方继续发送普通单聊消息
```

这里不建议直接删除 `im_conversation` 和 `im_message`。

否则用户重新加好友之后，原来的聊天记录会丢失。

可以保留会话，同时在会话列表中根据双方关系决定是否显示。

### 8.2 黑名单

建议增加：

```
CREATE TABLE im_user_block (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    blocker_user_id BIGINT UNSIGNED NOT NULL COMMENT '拉黑者',

    blocked_user_id BIGINT UNSIGNED NOT NULL COMMENT '被拉黑者',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_block_pair (
        blocker_user_id,
        blocked_user_id
    ),

    KEY idx_blocked_user (blocked_user_id)

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

黑名单是有方向的：

```
A 拉黑 B：

blocker_user_id = A
blocked_user_id = B
```

建议规则：

- A 拉黑 B 后，A、B 不能继续普通单聊。
- A 不能再向 B 发好友申请。
- B 不能向 A 发起普通好友申请。
- 已有群组是否允许双方共存，需要单独制定规则。
- 群聊是否允许发送消息，应以群组规则为准，不要直接把私聊黑名单等同于群组成员权限。

# 九、React 前端结构设计

建议在现有 React 项目中增加以下目录：

```
src/
└── modules/
    └── im/
        │
        ├── contacts/
        │   ├── pages/
        │   │   ├── ContactPage.tsx
        │   │   ├── FriendSearchPage.tsx
        │   │   ├── FriendRequestPage.tsx
        │   │   └── FriendProfilePage.tsx
        │   │
        │   ├── components/
        │   │   ├── FriendList.tsx
        │   │   ├── FriendItem.tsx
        │   │   ├── FriendRequestItem.tsx
        │   │   └── AddFriendDialog.tsx
        │   │
        │   ├── services/
        │   │   └── contactApi.ts
        │   │
        │   ├── hooks/
        │   │   └── useContacts.ts
        │   │
        │   └── types/
        │       └── contact.ts
        │
        ├── groups/
        │   ├── pages/
        │   │   ├── GroupListPage.tsx
        │   │   ├── GroupDetailPage.tsx
        │   │   └── GroupMembersPage.tsx
        │   │
        │   ├── components/
        │   │   ├── CreateGroupDialog.tsx
        │   │   ├── GroupMemberList.tsx
        │   │   ├── GroupInviteDialog.tsx
        │   │   └── GroupSettings.tsx
        │   │
        │   ├── services/
        │   │   └── groupApi.ts
        │   │
        │   ├── hooks/
        │   │   └── useGroups.ts
        │   │
        │   └── types/
        │       └── group.ts
        │
        ├── conversations/     # 原有会话模块
        ├── messages/          # 原有消息模块
        ├── realtime/          # 原有 Centrifugo 模块
        └── shared/
```

### 9.1 聊天主界面

推荐采用类似微信的三栏结构：

即时通信

消息

联系人

群组

张三

你好，最近怎么样？

10:30

Go 技术交流群

李四：今天讨论 Kafka

09:50

界面结构示意，实际布局可根据桌面端和移动端适配。

联系人页面：

```
联系人
  |
  +-- 新的朋友
  |
  +-- 我的好友
  |     |
  |     +-- 张三
  |     +-- 李四
  |     +-- 王五
  |
  +-- 我的群组
  |     |
  |     +-- Go 技术交流群
  |     +-- 项目讨论群
  |
  +-- 黑名单
```

新增好友页面：

```
添加好友
    |
    +-- 系统 ID
    |
    +-- 自定义 ID
    |
    +-- 手机号
    |
    +-- 邮箱
         |
         v
       搜索用户
         |
         v
       用户资料
         |
         v
       添加好友
         |
         v
       填写验证消息
         |
         v
       等待对方同意
```

# 十、通知系统如何与好友、群组集成

你之前的设计中已经有通知系统和消息中心，这里不建议重新建设通知模块。

直接复用原有通知体系。

新增以下通知事件：

| 事件                        | 接收者    | 说明     |
| ------------------------- | ------ | ------ |
| `friend_request_created`  | 被申请人   | 收到好友申请 |
| `friend_request_accepted` | 申请人    | 好友申请通过 |
| `friend_removed`          | 对方     | 好友关系解除 |
| `group_invited`           | 被邀请人   | 收到群邀请  |
| `group_join_requested`    | 群主、管理员 | 有人申请入群 |
| `group_member_joined`     | 群成员    | 新成员加入  |
| `group_member_removed`    | 被移除成员  | 被踢出群   |
| `group_owner_transferred` | 相关成员   | 群主转让   |

统一事件格式：

```
type IMNotificationEvent struct {
    EventID   string
    EventType string

    ReceiverID uint64

    ActorID uint64

    ResourceID string

    Payload json.RawMessage

    CreatedAt time.Time
}
```

例如：

```
{
  "eventId": "EVT10001",
  "eventType": "friend_request_created",
  "receiverId": "100002",
  "actorId": "100001",
  "resourceId": "FR202610310001",
  "payload": {
    "nickname": "张三",
    "message": "你好，可以认识一下吗？"
  }
}
```

注意区分：

```
通知系统：
    告诉用户发生了什么。

消息系统：
    保存用户之间实际发送的聊天内容。
```

好友申请通知不应该当成普通聊天消息写入 `im_message`。

# 十一、LiveKit 和 SeaweedFS 如何兼容？

这两个模块继续沿用原有架构。

## 11.1 LiveKit 音视频通话

好友单聊：

```
A 发起音视频通话
       |
       v
Go Call Service
       |
       v
验证 A、B 是否互为好友
       |
       v
验证双方是否允许通话
       |
       v
创建 LiveKit Room
       |
       v
生成 A、B 的 LiveKit Token
       |
       v
双方加入 Room
```

群组音视频：

```
群组发起通话
      |
      v
验证发起者是群成员
      |
      v
创建 Group Room
      |
      v
群成员申请加入
      |
      v
Go 验证群成员身份
      |
      v
签发 LiveKit Token
```

建议 Room 命名：

```
im_call_{call_id}
```

不要直接使用用户 ID 作为 Room ID。

LiveKit Token 中应该限制：

- roomJoin
- roomName
- canPublish
- canSubscribe
- canPublishData

并由 Go 服务端根据通话类型、群组成员身份及用户权限签发。

## 11.2 SeaweedFS 文件存储

图片、视频、语音和文件仍然统一存储在 SeaweedFS。

消息内容统一使用：

```
{
  "messageType": "image",
  "content": {
    "fileId": "FILE10001",
    "url": "/api/im/media/FILE10001",
    "width": 1280,
    "height": 720
  }
}
```

重点是：

文件上传成功不代表所有用户都能访问该文件。

下载时需要验证：

```
当前用户
    |
    v
查询文件所属消息
    |
    v
查询消息所属 conversation
    |
    v
验证当前用户是否有访问权限
    |
    v
允许下载
```

好友删除、群成员退出之后，是否仍能访问历史文件，应当遵循历史消息访问策略。

# 十二、生产级数据库关系总览

这是最终建议的关系模型。

```
                         user
                          |
              +-----------+-----------+
              |           |           |
              v           v           v
        im_user_profile  im_user_identity
              |
              |
       +------+-------+
       |              |
       v              v
im_friend_request  im_friendship
       |              |
       |              |
       v              v
  好友申请历史      双向好友关系
                      |
                      |
                      v
                im_conversation
                      |
              +-------+-------+
              |               |
              v               v
    im_conversation_member  im_message
              |               |
              |               v
              |          im_attachment
              |
              v
            im_group
              |
       +------+--------+
       |               |
       v               v
im_group_join_request  im_group_invitation


其他辅助表：

im_friend_remark
im_user_block
im_notification
im_outbox
```

这里需要特别说明：

- `im_friendship` 是好友关系的权威数据。
- `im_conversation` 是单聊、群聊统一会话。
- `im_conversation_member` 是会话成员的权威数据。
- `im_group` 保存群组特有属性。
- `im_message` 是统一消息存储。
- `im_outbox` 保证业务数据和待发布事件的一致性。

不要建立 `im_friend_message`、`im_group_message` 两套消息表。

否则后期消息历史、未读数、消息搜索、撤回、消息同步都需要维护两套逻辑。

# 十三、几个必须处理的生产级问题

这部分决定系统能否长期运行。

## 13.1 并发好友申请

假设 A、B 同时添加对方：

```
请求 1：A -> B
请求 2：B -> A
```

必须通过：

```
UNIQUE KEY uk_user_pair (
    user_low_id,
    user_high_id
)
```

保证关系唯一。

事务中使用 `SELECT ... FOR UPDATE` 和唯一键冲突处理，不能依赖 Go 进程内的 Mutex。

## 13.2 群组并发加入

假设群组最多 500 人，现在已有 499 人。

同时有两个人申请加入。

不能简单执行：

```
SELECT COUNT(*) = 499
INSERT MEMBER
```

因为两个请求都可能同时看到 499，最后变成 501 人。

推荐使用群组人数计数器：

```
ALTER TABLE im_group
ADD COLUMN member_count INT UNSIGNED NOT NULL DEFAULT 1;
```

入群时：

```
UPDATE im_group
SET member_count = member_count + 1
WHERE group_id = ?
  AND status = 1
  AND member_count < max_members;
```

必须检查 `RowsAffected()`：

```
if affected != 1 {
    return ErrGroupFull
}
```

然后在同一个事务内插入群成员记录。

成员退出、被移除也必须同步递减，且保证幂等。

## 13.3 未读数

继续沿用原有 Redis 未读数设计。

例如：

```
A 发送消息给 B。

B 未读数：
    Redis: im:unread:100002:C1001 = 5

B 打开会话：
    将已读位置推进到 message_seq = 105

Redis：
    更新未读数投影
```

群组中：

```
A 发送群消息

B、C、D 分别计算自己的未读状态。
```

未读数不能简单存储在群组表中，因为每个群成员的已读位置不同。

建议使用：

```
im_conversation_member.last_read_seq
```

作为已读位置的权威记录，Redis 作为高频读取缓存。

## 13.4 好友删除和消息发送并发

这个问题需要额外重视。

例如：

```
T1：A 发送消息
T2：B 删除 A
```

两个请求可能同时执行。

建议对单聊权限变更和消息写入采用一致的事务锁定策略：

```
1. 锁定 friendship 记录。
2. 检查 status = accepted。
3. 执行消息持久化。
4. 提交事务。
```

删除好友也必须锁定同一关系记录。

这样能够确定并发操作的先后顺序。

不要只在消息发送前查询一次好友关系，然后在事务外继续写消息。

## 13.5 消息发布失败

继续使用原有：

```
MySQL Transaction
       |
       +-- im_message
       |
       +-- im_outbox
       |
       v
     COMMIT
       |
       v
    Kafka Consumer
       |
       v
    Centrifugo
```

如果 Centrifugo 暂时不可用：

- 消息仍然保存在 MySQL。
- Outbox 事件等待重试。
- 客户端可以通过 `/sync` 补偿同步。
- 不会因为实时推送失败而丢失消息。

这也是为什么不能把 Centrifugo 当成消息数据库。

# 十四、建议的 Go 错误码

统一使用业务错误，而不是在各个 Service 中随意返回字符串。

| 错误                           | 含义       | HTTP |
| ---------------------------- | -------- | ---- |
| `ErrCannotAddSelf`           | 不能添加自己   | 400  |
| `ErrFriendRequestExists`     | 好友申请已存在  | 409  |
| `ErrFriendshipAlreadyExists` | 已经是好友    | 409  |
| `ErrFriendRequestNotFound`   | 申请不存在    | 404  |
| `ErrFriendRequestForbidden`  | 无权处理此申请  | 403  |
| `ErrFriendshipRequired`      | 必须先成为好友  | 403  |
| `ErrUserBlocked`             | 用户已被拉黑   | 403  |
| `ErrGroupNotFound`           | 群组不存在    | 404  |
| `ErrGroupForbidden`          | 没有群组操作权限 | 403  |
| `ErrGroupFull`               | 群组人数已满   | 409  |
| `ErrGroupMemberExists`       | 已经是群成员   | 409  |
| `ErrGroupOwnerRequired`      | 需要群主权限   | 403  |
| `ErrGroupMemberNotFound`     | 群成员不存在   | 404  |

建议在业务错误包中统一维护。

# 十五、具体实施顺序：按照这个顺序编码

不建议一次性把所有模块都写出来。

建议分六个阶段。

1. 第一阶段：用户身份体系

   基础阶段
   - 用户系统 ID
   - 自定义 username
   - 手机号、邮箱身份索引
   - 用户搜索
   - 用户隐私设置
2. 第二阶段：好友关系
   - 好友申请
   - 接受、拒绝、撤回
   - 双向好友关系
   - 好友列表
   - 好友备注
   - 删除好友
   - 黑名单
3. 第三阶段：单聊权限改造
   - 好友接受后创建单聊
   - SINGLE 会话唯一性
   - 发送消息好友关系校验
   - 历史消息权限
   - 实时通知
4. 第四阶段：群组系统
   - 创建群组
   - 群成员管理
   - 群邀请
   - 入群申请
   - 群主和管理员
   - 退出、移除、解散
5. 第五阶段：实时通信集成
   - Centrifugo 订阅鉴权
   - 好友申请事件
   - 群成员变更事件
   - 群消息推送
   - 离线补偿
   - Redis 未读数
6. 第六阶段：音视频与文件集成
   - LiveKit 好友音视频通话
   - LiveKit 群组通话
   - SeaweedFS 文件权限
   - 历史文件访问控制

## 15.1 必须编写的测试

建议至少覆盖以下场景：

| 测试场景            | 预期结果      |
| --------------- | --------- |
| A 添加 B，B 未接受    | 不能单聊      |
| B 接受 A          | 双方可以单聊    |
| A 重复添加 B        | 不产生重复关系   |
| A、B 同时添加对方      | 只产生一条好友关系 |
| A 删除 B          | 双方不能继续单聊  |
| A 拉黑 B          | 禁止普通单聊    |
| A、B 不是好友但都在群内   | 可以群聊      |
| A 被踢出群组         | 不能继续发送群消息 |
| A 退出群组          | 不能继续订阅群消息 |
| 群组达到人数上限        | 拒绝新成员加入   |
| Centrifugo 暂时宕机 | 消息仍然持久化   |
| 群成员并发加入         | 人数不能超过上限  |
| 用户重复接受好友申请      | 不重复创建会话   |

# 十六、最终推荐的工程设计

我建议你将整个即时通信系统划分为以下六个核心领域：

```
                    IM Platform
                         |
       +-----------------+-----------------+
       |                 |                 |
       v                 v                 v
   Identity           Contact            Group
   用户身份            好友关系           群组管理
       |                 |                 |
       +-----------------+-----------------+
                         |
                         v
                    Conversation
                      会话管理
                         |
                         v
                      Message
                      消息管理
                         |
              +----------+----------+
              |                     |
              v                     v
          Centrifugo             Media
          实时推送              SeaweedFS
                                    |
                                    v
                                 LiveKit
                                 音视频
```

实际职责边界：

| 领域           | 权威数据             | 核心职责       |
| ------------ | ---------------- | ---------- |
| Identity     | 用户身份表            | 用户搜索、身份验证  |
| Contact      | 好友、黑名单表          | 好友申请、双向关系  |
| Group        | 群组资料、成员记录        | 群组及成员权限    |
| Conversation | 会话、成员表           | 单聊和群聊会话    |
| Message      | 消息、附件、Outbox     | 消息存储、幂等、同步 |
| Realtime     | Redis、Centrifugo | 实时推送和在线状态  |

最终有三条规则必须贯穿整个系统：

1. 单聊权限由好友关系决定，双方必须互相成为好友。
2. 群聊权限由群成员关系决定，群成员之间不要求互为好友。
3. 所有业务权限由 Go 后端验证，Centrifugo 负责实时传输，MySQL 保存权威业务数据。

### 参考资料

- [Centrifugo Channels and Namespaces](https://centrifugal.dev/docs/server/channels?utm_source=chatgpt.com)：频道和用户受限订阅。
- [Centrifugo Channel Permission Model](https://centrifugal.dev/docs/server/channel_permissions?utm_source=chatgpt.com)：订阅、发布、历史记录的权限控制。
- [Centrifugo Server-side Subscriptions](https://centrifugal.dev/docs/server/server_subs?utm_source=chatgpt.com)：服务端订阅管理。

我的最终建议：先把 Identity、Contact、Group、Conversation 四个领域的数据库和 Go Service 建好，再接入现有 Message Service。 不要直接在原有消息发送接口里堆积好友、群组、黑名单等业务代码。

这样以后增加企业群、群公告、群禁言、群文件、群音视频、好友推荐等功能，都不需要推翻现有架构。