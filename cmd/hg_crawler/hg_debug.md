# Crawler Dashboard 本地调试

## 请求链路与 500 排查

`MLC_React` 的 `loadDashboard` 请求链路为：

```text
http://localhost:5174/api/v1/crawler/dashboard
  -> Vite /api/v1/crawler 代理
  -> http://localhost:8090/api/v1/crawler/dashboard
  -> hgCrawlerRoutes -> handler.dashboard -> HGManager.Dashboard
```

`8090` 由独立入口 `cmd/hg_crawler` 提供。根入口 `main_mlc_project.go` 的 `8080` 不注册这组管理 API；即使开启主服务内嵌 Bilibili worker，也不会监听 `8090`。

只启动主服务时，Vite 因无法连接 `8090` 返回 HTTP 500，响应为 `text/plain`，请求尚未进入 `handler.dashboard`。将代理改到 `8080` 会得到 404。

## VS Code 左上角运行和调试

在 `MLC_GO` 工程的运行和调试下拉框选择：

- **`🧪 Launch MLC_GO Debug`**：先执行既有 `ensure-debug-deps` 任务，再联合启动 `8080` 主服务和 `8090` crawler。停止一个联合调试会话时同步停止另一个。主服务通过 `CRAWLER_BILIBILI_ENABLED=false` 关闭内嵌 Bilibili worker，由独立 crawler 执行采集。
- **`🕷️ Launch hg_crawler Debug`**：当前主服务已在运行时，单独补启动 crawler；此项使用现有 MySQL/Redis，不执行依赖初始化任务。

然后点击左上角绿色运行按钮或按 F5。两个进程都可设置 Go 断点；crawler 的 `program` 固定为整个 `cmd/hg_crawler` 包，不依赖当前编辑器打开的文件。工作目录固定为工程根目录，并显式传入 `--env=debug` 和配置绝对路径。

通用的 `Launch cmd（当前入口）` / `Launch cmd（选择入口）` 默认参数是 `--help`，只显示帮助后退出；启动 crawler 服务应使用上面的专用配置。

联合入口沿用既有依赖任务行为：可启动本地依赖、执行 debug 数据库迁移及 Kafka Topic 初始化。crawler 常驻模式会立即执行一次 Bilibili 采集，成功后写入 debug MySQL 并更新 Redis，随后默认每 5 分钟执行一次。

## 终端启动与验证

从 `MLC_GO` 工程根目录执行：

```bash
SERVER_ENV=debug go run ./cmd/hg_crawler --env=debug --config-dir=./config --addr=:8090
```

服务就绪后检查独立接口和前端代理：

```bash
curl --noproxy '*' -i http://localhost:8090/healthz
curl --noproxy '*' -i http://localhost:8090/api/v1/crawler/dashboard
curl --noproxy '*' -i http://localhost:5174/api/v1/crawler/dashboard
```

Dashboard 预期返回 HTTP 200、`Content-Type: application/json`，业务码 `code=200`，`result` 包含 `stats`、`tasks`、`trend`。上游采集失败会反映为失败任务，Dashboard 仍可正常返回内存快照。
