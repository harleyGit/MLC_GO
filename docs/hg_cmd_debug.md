# cmd 入口运行与调试

在 VS Code 中打开 `MLC_GO` 根目录，使用 `.vscode/launch.json` 的两个配置：

1. **当前文件入口**：打开 `cmd/hg_recharge_migrate/main.go`（或其他 cmd 入口），在“运行和调试”下拉框选择 **🛠️ Launch cmd（当前入口）**，按 F5 或点击绿色启动按钮。
2. **面板选择入口**：选择 **🛠️ Launch cmd（选择入口）** 并启动，在弹出的列表里选择命令；默认是 `hg_recharge_migrate`，不依赖当前编辑文件。

两个配置均按整个包编译，工作目录 `cwd` 固定为 `${workspaceFolder}`，默认 `SERVER_ENV=debug`、`MLC_CONFIG_DIR=${workspaceFolder}/config`。可在 `main()` 设置断点。默认 `args: ["--help"]` 显示命令参数后退出；需要执行具体任务时修改所选配置的 `args`。

| 入口 | 用途 |
| --- | --- |
| `cmd/hg_recharge_migrate` | 按显式主键区间迁移旧充值订单 |
| `cmd/hg_config_check` | 配置及依赖检查、显式数据库迁移 |
| `cmd/hg_crawler` | 采集任务及采集服务 |
| `cmd/hg_video_comment_reaction_backfill` | 视频评论互动分片数据回填 |

`cmd/server/main.go` 目前只有 `package server`，不能作为可执行入口；主服务使用 **🧪 Launch MLC_GO Debug**。

## 充值迁移

从工程根目录验证启动与帮助输出：

```bash
go run ./cmd/hg_recharge_migrate --help
```

帮助模式正常退出，不连接数据库。实际迁移必须先满足 [000038 切换说明](../migrations/000038_wallet_unified_recharge.md) 中的停写、备份和 schema 前提，再显式提供 `--env`、`--confirm-stopped-writes`、`--low`、`--high`；每次主键区间最多 500。默认帮助启动不会执行迁移。

cmd 调试配置不挂载主服务的 `ensure-debug-deps` 前置任务；数据库迁移和依赖初始化仍按具体任务显式执行。
