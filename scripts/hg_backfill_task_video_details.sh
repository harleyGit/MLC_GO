#!/usr/bin/env bash
# 为指定采集任务补齐 B 站视频详情（默认 taskId=1）。
# 数据来源与 /api/v1/ops/crawler/tasks/contents?taskId=1&cursor=0&pageSize=100
# 相同：crawler_task_external_contents 任务关联表；无需登录管理后台或复制 Cookie。
# 此脚本不直接写详情表，而是通过现有 Outbox -> Kafka -> 抓取/清洗/存储流水线补采。
# 因此“已提交”只表示排队成功，不代表页面立即有详情。

set -euo pipefail

# 使用脚本所在目录定位工程根目录，不受执行时工作目录影响。
HG_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HG_TASK_ID="${TASK_ID:-1}"
HG_PAGES="${PAGES:-10}"
HG_AFTER="${AFTER:-0}"

# 限制目标环境：本工具只允许写入本机 debug 数据库，绝不自动启动依赖或迁移。
if [[ "${SERVER_ENV:-debug}" != "debug" ]]; then
  echo '仅允许 SERVER_ENV=debug' >&2
  exit 1
fi

# 防止把不受控的参数传给 Go 命令；每页固定最多 100 条，可用 AFTER 续跑。
if [[ ! "$HG_TASK_ID" =~ ^[1-9][0-9]*$ || ! "$HG_PAGES" =~ ^[1-9][0-9]*$ || "$HG_PAGES" -gt 100 || ! "$HG_AFTER" =~ ^(0|[1-9][0-9]*)$ ]]; then
  echo 'TASK_ID 必须为正整数；PAGES 为 1-100；AFTER 为非负整数' >&2
  exit 1
fi

# 要求操作人显式确认流水线开关；这个检查只作用于本命令，无法代替检查服务端消费者。
if [[ "${CRAWLER_VIDEO_PIPELINE_ENABLED:-}" != "true" ]]; then
  echo '需要 CRAWLER_VIDEO_PIPELINE_ENABLED=true 且后端 Outbox/Kafka/消费者已运行' >&2
  exit 1
fi

cd "$HG_ROOT"
echo "补采任务 ${HG_TASK_ID}：从关联游标 ${HG_AFTER} 起，最多 ${HG_PAGES} 页，每页 100 条"

# SQL 按任务关联主键稳定翻页；详情已入库或采集正在进行时跳过。
# 失败的页会回滚且不推进游标；成功页显示续跑所需的 -after 值。
SERVER_ENV=debug go run ./cmd/hg_crawler_backfill \
  -apply -task-id="$HG_TASK_ID" -after="$HG_AFTER" -pages="$HG_PAGES"

# 异步流水线可能仍在处理；需要核对时可手动执行：
# SERVER_ENV=debug go run ./cmd/hg_crawler_backfill -verify -task-id=1 -pages=100
# 若输出 pending/raw/cleaned，等待消费者完成；failed 需先排查上游响应。
