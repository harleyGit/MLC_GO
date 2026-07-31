#!/usr/bin/env bash

set -euo pipefail

KAFKA_CONTAINER="${KAFKA_CONTAINER:-mlc-kafka-1}"
KAFKA_BOOTSTRAP_SERVERS="${KAFKA_BOOTSTRAP_SERVERS:-kafka-1:9092,kafka-2:9092,kafka-3:9092}"
KAFKA_TOPIC="${KAFKA_TOPIC:-mlc.domain.events}"
KAFKA_PARTITIONS="${KAFKA_PARTITIONS:-12}"
KAFKA_REPLICATION_FACTOR="${KAFKA_REPLICATION_FACTOR:-3}"
KAFKA_TOPICS_BIN="/opt/kafka/bin/kafka-topics.sh"

if ! docker inspect "$KAFKA_CONTAINER" >/dev/null 2>&1; then
    echo "[ERROR] Kafka 容器不存在: $KAFKA_CONTAINER"
    exit 1
fi

if [ "$(docker inspect -f '{{.State.Running}}' "$KAFKA_CONTAINER")" != "true" ]; then
    echo "[ERROR] Kafka 容器未运行: $KAFKA_CONTAINER"
    exit 1
fi

echo "[INFO] 确保 Kafka topic 存在: $KAFKA_TOPIC"
docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
    --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
    --create \
    --if-not-exists \
    --topic "$KAFKA_TOPIC" \
    --partitions "$KAFKA_PARTITIONS" \
    --replication-factor "$KAFKA_REPLICATION_FACTOR"

docker exec "$KAFKA_CONTAINER" "$KAFKA_TOPICS_BIN" \
    --bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" \
    --describe \
    --topic "$KAFKA_TOPIC"
