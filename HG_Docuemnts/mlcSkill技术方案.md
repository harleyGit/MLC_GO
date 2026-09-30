MLC_GO工程路径：`/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO`
MLC_React工程路径：`‌/Users/ganghuang/HGFiles/GitHub/MLC_React `
iOS工程MLC路径：`‌/Users/ganghuang/HGFiles/GitHub/MLC `


/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/opencode.json 中instructions 是强制加载路径下的规则，这个不正确，我想要的是skill是按需加载按需调用。

将/Users/ganghuang/HGFiles/GitHub/AITools/Skills/mlc-engineering/mlc-go-project/references 、/Users/ganghuang/HGFiles/GitHub/AITools/Skills/mlc-engineering/mlc-ios-project/references 、/Users/ganghuang/HGFiles/GitHub/AITools/Skills/mlc-engineering/mlc-react-project/references 和/Users/ganghuang/HGFiles/GitHub/AITools/Skills/mlc-engineering 中对应的skill模块中进行对比，去除重复的。然后将其中的规则抽 然后参考如下的结构：


```
~/HGFiles/GitHub/AITools/
│
├── Skills/                         # 公共 AI 工程规范仓库
│   │
│   └── mlc-engineering/
│       │
│       ├── README.md
│       │
│       ├── common/                 # 所有工程通用
│       │   ├── security/
│       │   │   └── SKILL.md
│       │   ├── code-review/
│       │   │   └── SKILL.md
│       │   └── engineering-workflow/
│       │       └── SKILL.md
│       │
│       ├── go/                     # Go 专用
│       │   ├── SKILL.md
│       │   ├── http/
│       │   │   └── SKILL.md
│       │   ├── redis/
│       │   │   └── SKILL.md
│       │   ├── kafka/
│       │   │   └── SKILL.md
│       │	 ├── mysql/
│       │   │   └── SKILL.md
│       │	 ├── static/
│       │   │   └── SKILL.md
│       │	 ├── danmaku/
│       │   │   └── SKILL.md
│       │	 ├── clickhouse/
│       │   │   └── SKILL.md
│       │	 ├── concurrency/
│       │   │   └── SKILL.md
│       │	 ├── api/
│       │   │   └── SKILL.md
│       │	 ├── development/
│       │   │   └── SKILL.md
│       │   └── clickhouse/
│       │       └── SKILL.md
│       │
│       ├── react/                  # React 专用
│       │   ├── SKILL.md
│       │
│       └── ios/                    # iOS 专用
│           ├── SKILL.md
│
└── ...
```

***
<br/><br/>


## 形成如下的结构

比如/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO ：

```
MLC_GO/
├── AGENTS.md
└── opencode.json
```

/Users/ganghuang/HGFiles/GitHub/MLC：

```
MLC/
├── AGENTS.md
└── opencode.json
```

/Users/ganghuang/HGFiles/GitHub/MLC_React：

```
MLC_React/
├── AGENTS.md
└── opencode.json
```

***
<br/> <br/>

### AGENTS.md遵守的规则是：

```text
项目是什么
项目目录结构
该工程的版本：比如Go版本，React版本，iOS版本
核心架构
命名规范
不能做什么
如何运行测试
如何启动服务
关键依赖
```

`opencode.json`告诉 OpenCode：公共 Skill 在哪里，例如：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "skills": [
    "~/HGFiles/GitHub/AITools/Skills/mlc-engineering"
  ]
}
```


<br/> <br/>

### 比如 MLC_Go中的 AGENTS.md， 参考如下模版：

```
# MLC_GO 工程开发规范

## 1. 项目说明

本项目是 MLC 后端 Go 服务。

主要技术栈：

- Go
- net/http
- MySQL
- Redis
- Kafka
- ClickHouse

---

## 2. AI 工作原则

在修改代码之前：

1. 先理解现有代码。
2. 优先复用已有实现。
3. 不要无理由引入新的第三方依赖。
4. 修改完成后检查编译和测试。

---

## 3. Go 版本

项目使用 Go 1.23+。

必须遵循当前 go.mod 中声明的 Go 版本。

不要因为个人习惯修改 Go 版本。

---

## 4. 工程结构

主要目录：

```


<br/>

### MLC_React的AGENTS.md， 参考模版如下：

```
# MLC React 工程开发规范

## 1. 项目说明

本项目是 MLC Web 前端项目。

主要技术：

- React
- Vite

---

## 2. React 开发规范

当前项目使用 React Class Component。

默认不要改成 Function Component。

不要主动将：

class Xxx extends React.Component
改成
function Xxx() {}
```


<br/>

### iOS工程MLC，参考模版如下：

```
# iOS 工程开发规范

## 1. UI 技术

项目使用 UIKit。

默认不要使用 SwiftUI。

---

## 2. Swift

使用项目当前 Swift/Xcode 版本。

不要因为个人习惯升级 Swift 或 Xcode。

---

## 3. UI 布局

项目主要使用：

- SnapKit
- Frame Layout

修改 UI 时优先复用项目已有布局方式。


```


***
<br/> <br/>

## 比如Go中的SKill负责：


```text
Go Redis 怎么写
Go Kafka 怎么写
ClickHouse 怎么写
API 怎么设计
性能优化怎么做
代码 Review 怎么做
```

其他的Reac工程、iOS工程Skill按照对应的特点进行设计对应的skill。

***
<br/><br/>

最终形成一个很清晰的层次,比如MLC_GO可以记成：

```text
                    OpenCode
                       │
              ┌────────┴────────┐
              │                 │
          opencode.json       AGENTS.md
              │                 │
              │                 └── 项目基本规则
              │
              └── Skills 路径
                       │
                       ↓
              公共 Skill 仓库
                       │
        ┌──────────────┼──────────────┐
        ↓              ↓              ↓
     Go Skill       Redis Skill    Kafka Skill
        │              │              │
        ↓              ↓              ↓
      Go规则         Redis规则      Kafka规则
```

所以你最开始这个：

```json
"instructions": [
  "~/HGFiles/GitHub/AITools/Skills/mlc-engineering/mlc-go-project/SKILL.md"
]
```

按照MLC_GO这个架构来设计MLC_React和MLC。


***
<br/><br/>

## 三层关系可以这样记,以后一直遵守的原则：

```
                    OpenCode CLI
                         │
                         ↓
                  ┌──────────────┐
                  │ opencode.json│
                  └──────┬───────┘
                         │
                 找到公共 Skills
                         │
                         ↓
             ┌──────────────────────┐
             │  AITools/Skills      │
             │                      │
             │ Go                   │
             │ Redis                │
             │ Kafka                │
             │ React                │
             │ iOS                  │
             └──────────┬───────────┘
                        │
                 按任务按需使用
                        │
                        ↓
               ┌────────────────┐
               │   AGENTS.md    │
               │                │
               │ 当前项目规则   │
               └───────┬────────┘
                       │
                       ↓
                  当前项目代码
```


