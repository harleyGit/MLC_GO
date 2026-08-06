#!/usr/bin/env bash

# 开启 Bash 严格模式：
# -e：任意命令失败时立即退出，避免错误后继续创建或检查 Topic。
# -u：读取未定义变量时立即报错，避免空变量被误当成有效配置。
# -o pipefail：管道中任意命令失败，整条管道都算失败。
set -euo pipefail

# 下面这些变量都支持由外部环境变量覆盖。
# 写法 ${变量名:-默认值} 表示：外部没有设置时，使用冒号后面的默认值。
# 例如临时检查其他 Topic 时可以执行：KAFKA_TOPIC=example.topic ./scripts/kafka_init.sh。

# KAFKA_CONTAINER 是执行 kafka-topics.sh 的入口容器。
# 只需要进入一个 broker 容器，就可以通过 bootstrap servers 访问整个三节点集群。
KAFKA_CONTAINER="${KAFKA_CONTAINER:-mlc-kafka-1}"
# 容器内部使用 kafka-1、kafka-2、kafka-3 这些 compose 服务名互相访问。
# 这里不能写宿主机的 localhost:19092，因为命令是在容器内部执行的。
KAFKA_BOOTSTRAP_SERVERS="${KAFKA_BOOTSTRAP_SERVERS:-kafka-1:9092,kafka-2:9092,kafka-3:9092}"
# 应用发送领域事件时使用的业务 Topic 名称。
KAFKA_TOPIC="${KAFKA_TOPIC:-mlc.domain.events}"
# Topic 分区数。分区让消息可以并行生产和消费，但同一分区内仍保持消息顺序。
KAFKA_PARTITIONS="${KAFKA_PARTITIONS:-12}"
# 弹幕历史使用独立 Topic，按 video_id 作为 key 保证同视频进入同一分区。
KAFKA_DANMAKU_TOPIC="${KAFKA_DANMAKU_TOPIC:-mlc.video.danmaku.created.v1}"
KAFKA_DANMAKU_PARTITIONS="${KAFKA_DANMAKU_PARTITIONS:-96}"
# 每个分区保存 3 份副本，对应本地三个 broker。
KAFKA_REPLICATION_FACTOR="${KAFKA_REPLICATION_FACTOR:-3}"
# apache/kafka 镜像中 Kafka Topic 管理命令的固定路径。
KAFKA_TOPICS_BIN="/opt/kafka/bin/kafka-topics.sh"
# Kafka 容器刚启动时不一定马上可用。默认最多检查 60 次，每次间隔 1 秒。
KAFKA_READY_RETRIES="${KAFKA_READY_RETRIES:-60}"

# docker compose 返回“容器已启动”只代表进程已开始运行，不代表 Kafka 已完成选主并可处理请求。
# 所以这里最多轮询 KAFKA_READY_RETRIES 次，并同时满足三个条件：
# 1. docker inspect 能找到入口容器；
# 2. 容器状态是 Running；
# 3. kafka-topics.sh --list 能真正连接 Kafka 集群并读取 Topic 列表。
for ((attempt = 1; attempt <= KAFKA_READY_RETRIES; attempt++)); do
    # >/dev/null 2>&1 表示隐藏标准输出和错误输出。
    # 探活阶段只关心命令成功或失败，不需要把每次重试的错误刷到 VS Code 终端。
    if docker inspect "$KAFKA_CONTAINER" >/dev/null 2>&1 \
        && [ "$(docker inspect -f '{{.State.Running}}' "$KAFKA_CONTAINER")" = "true" ] \
        && docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
            --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
            --list >/dev/null 2>&1; then
        break
    fi

    # 已经达到最大重试次数仍未成功时，以非 0 状态退出。
    # VS Code 的 preLaunchTask 会因此失败，阻止应用在 Kafka 未就绪时继续启动。
    if [ "$attempt" -eq "$KAFKA_READY_RETRIES" ]; then
        echo "[ERROR] Kafka 未在预期时间内就绪: $KAFKA_CONTAINER" >&2
        exit 1
    fi
    # 给 Kafka 1 秒初始化时间后再检查，避免无间隔循环持续消耗 CPU。
    sleep 1
done

ensure_topic() {
local topic="$1"
local partitions="$2"
echo "[INFO] 确保 Kafka topic 存在: $topic"
# docker exec 表示在指定容器内执行命令。
# --create 创建 Topic；--if-not-exists 使脚本可以重复运行：
# Topic 不存在时创建，已经存在时不报“重复创建”错误。
# 注意：--if-not-exists 不会自动修正已有 Topic 的错误分区数或副本数，所以下面还要单独校验。
docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
    --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
    --create \
    --if-not-exists \
    --topic "$topic" \
    --partitions "$partitions" \
    --replication-factor "$KAFKA_REPLICATION_FACTOR"

# --describe 会返回一行 Topic 汇总信息和每个分区的一行明细。
# $(...) 是命令替换：先执行括号里的命令，再把完整输出保存到 topic_description 变量。
topic_description="$(docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
    --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
    --describe \
    --topic "$topic")"
# 把原始拓扑打印到终端，方便开发者直接查看每个分区的 Leader、Replicas 和 ISR。
printf '%s\n' "$topic_description"

# Topic 汇总行示例：PartitionCount: 12 ReplicationFactor: 3。
# 分区明细行包含“Partition:”字段，并且各字段之间由 Tab（制表符）分隔。
# $'\tPartition:' 是 Bash 的转义字符串写法，其中 \t 表示一个真正的 Tab 字符。
topic_summary="$(printf '%s\n' "$topic_description" | grep 'PartitionCount:' | head -n 1)"
partition_lines="$(printf '%s\n' "$topic_description" | grep $'\tPartition:')"

# [[ ... ]] 用来进行字符串判断。
# 这里同时要求汇总行包含预期分区数和预期副本数；任意一个不匹配都立即失败。
# 这样可以发现 Topic 曾被人工用错误参数创建，而不是仅确认“Topic 名字存在”。
if [[ "$topic_summary" != *"PartitionCount: $partitions"* ]] \
    || [[ "$topic_summary" != *"ReplicationFactor: $KAFKA_REPLICATION_FACTOR"* ]]; then
    echo "[ERROR] Kafka Topic 分区数或副本数不符合预期: $topic" >&2
    exit 1
fi

# 再数一次分区明细行，防止汇总信息看似正确，但 describe 输出缺失了某些分区。
# grep -c 返回匹配行数，保存到 partition_count 后与预期分区数做整数比较。
partition_count="$(printf '%s\n' "$partition_lines" | grep -c $'\tPartition:')"
if [ "$partition_count" -ne "$partitions" ]; then
    echo "[ERROR] Kafka Topic 分区明细数量不符合预期: expected=$partitions actual=$partition_count" >&2
    exit 1
fi

# 逐行检查每个分区的副本和 ISR：
# Replicas 是 Kafka 为该分区配置的全部副本 broker；
# ISR（In-Sync Replicas）是当前仍与 Leader 保持同步、可以参与故障切换的副本。
# 配置 3 副本但 ISR 只有 2，通常说明有 broker 离线或副本同步落后，不能算完全健康。
# IFS=$'\t' 表示按 Tab 拆字段；前三个字段不需要，因此用 _ 占位。
while IFS=$'\t' read -r _ _ _ replicas_field isr_field; do
    # ${变量#前缀} 会删除字段开头的固定文字，只留下逗号分隔的 broker id。
    # 例如“Replicas: 1,2,3”会得到“1,2,3”。
    replicas="${replicas_field#Replicas: }"
    isr="${isr_field#Isr: }"
    # tr 把逗号替换为换行，grep -c . 再统计非空行，从而得到副本数量。
    replica_count="$(printf '%s' "$replicas" | tr ',' '\n' | grep -c .)"
    isr_count="$(printf '%s' "$isr" | tr ',' '\n' | grep -c .)"
    # 每个分区的实际副本数和 ISR 数都必须等于配置的副本因子。
    # 任一分区不完整就返回失败，阻止应用在降级的本地 Kafka 上启动。
    if [ "$replica_count" -ne "$KAFKA_REPLICATION_FACTOR" ] || [ "$isr_count" -ne "$KAFKA_REPLICATION_FACTOR" ]; then
        echo "[ERROR] Kafka Topic 存在副本或 ISR 不完整的分区: replicas=$replicas ISR=$isr" >&2
        exit 1
    fi
# <<< 是 here-string，把 partition_lines 的内容作为 while/read 的标准输入。
done <<<"$partition_lines"

echo "[INFO] Kafka Topic 拓扑检查通过: topic=$topic partitions=$partitions replication-factor=$KAFKA_REPLICATION_FACTOR ISR=完整"
}

ensure_topic "$KAFKA_TOPIC" "$KAFKA_PARTITIONS"
ensure_topic "$KAFKA_DANMAKU_TOPIC" "$KAFKA_DANMAKU_PARTITIONS"
