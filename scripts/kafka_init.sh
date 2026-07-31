#!/usr/bin/env bash

set -euo pipefail

KAFKA_CONTAINER="${KAFKA_CONTAINER:-mlc-kafka-1}"
KAFKA_BOOTSTRAP_SERVERS="${KAFKA_BOOTSTRAP_SERVERS:-kafka-1:9092,kafka-2:9092,kafka-3:9092}"
KAFKA_TOPIC="${KAFKA_TOPIC:-mlc.domain.events}"
KAFKA_PARTITIONS="${KAFKA_PARTITIONS:-12}"
KAFKA_REPLICATION_FACTOR="${KAFKA_REPLICATION_FACTOR:-3}"
KAFKA_TOPICS_BIN="/opt/kafka/bin/kafka-topics.sh"
KAFKA_READY_RETRIES="${KAFKA_READY_RETRIES:-60}"

for ((attempt = 1; attempt <= KAFKA_READY_RETRIES; attempt++)); do
    if docker inspect "$KAFKA_CONTAINER" >/dev/null 2>&1 \
        && [ "$(docker inspect -f '{{.State.Running}}' "$KAFKA_CONTAINER")" = "true" ] \
        && docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
            --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
            --list >/dev/null 2>&1; then
        break
    fi

    if [ "$attempt" -eq "$KAFKA_READY_RETRIES" ]; then
        echo "[ERROR] Kafka 未在预期时间内就绪: $KAFKA_CONTAINER" >&2
        exit 1
    fi
    sleep 1
done

echo "[INFO] 确保 Kafka topic 存在: $KAFKA_TOPIC"
docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
    --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
    --create \
    --if-not-exists \
    --topic "$KAFKA_TOPIC" \
    --partitions "$KAFKA_PARTITIONS" \
    --replication-factor "$KAFKA_REPLICATION_FACTOR"

topic_description="$(docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
    --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
    --describe \
    --topic "$KAFKA_TOPIC")"
printf '%s\n' "$topic_description"

topic_summary="$(printf '%s\n' "$topic_description" | grep 'PartitionCount:' | head -n 1)"
partition_lines="$(printf '%s\n' "$topic_description" | grep $'\tPartition:')"

if [[ "$topic_summary" != *"PartitionCount: $KAFKA_PARTITIONS"* ]] \
    || [[ "$topic_summary" != *"ReplicationFactor: $KAFKA_REPLICATION_FACTOR"* ]]; then
    echo "[ERROR] Kafka Topic 分区数或副本数不符合预期: $KAFKA_TOPIC" >&2
    exit 1
fi

partition_count="$(printf '%s\n' "$partition_lines" | grep -c $'\tPartition:')"
if [ "$partition_count" -ne "$KAFKA_PARTITIONS" ]; then
    echo "[ERROR] Kafka Topic 分区明细数量不符合预期: expected=$KAFKA_PARTITIONS actual=$partition_count" >&2
    exit 1
fi

while IFS=$'\t' read -r _ _ _ replicas_field isr_field; do
    replicas="${replicas_field#Replicas: }"
    isr="${isr_field#Isr: }"
    replica_count="$(printf '%s' "$replicas" | tr ',' '\n' | grep -c .)"
    isr_count="$(printf '%s' "$isr" | tr ',' '\n' | grep -c .)"
    if [ "$replica_count" -ne "$KAFKA_REPLICATION_FACTOR" ] || [ "$isr_count" -ne "$KAFKA_REPLICATION_FACTOR" ]; then
        echo "[ERROR] Kafka Topic 存在副本或 ISR 不完整的分区: replicas=$replicas ISR=$isr" >&2
        exit 1
    fi
done <<<"$partition_lines"

echo "[INFO] Kafka Topic 拓扑检查通过: partitions=$KAFKA_PARTITIONS replication-factor=$KAFKA_REPLICATION_FACTOR ISR=完整"
