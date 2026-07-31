#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
    echo "[FAIL] $1" >&2
    exit 1
}

run_case() {
    local case_name="$1"
    local describe_output="$2"
    local expected_status="$3"
    local temp_dir
    temp_dir="$(mktemp -d)"

    mkdir -p "${temp_dir}/bin"
    printf '%s\n' "${describe_output}" >"${temp_dir}/describe.txt"

    cat >"${temp_dir}/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "inspect" ]; then
    if [ "${2:-}" = "-f" ]; then
        echo "true"
    fi
    exit 0
fi
if [ "${1:-}" = "exec" ] && [[ " $* " = *" --describe "* ]]; then
    cat "${DESCRIBE_OUTPUT_FILE}"
fi
EOF
    chmod +x "${temp_dir}/bin/docker"

    set +e
    DESCRIBE_OUTPUT_FILE="${temp_dir}/describe.txt" \
        PATH="${temp_dir}/bin:${PATH}" \
        KAFKA_PARTITIONS=2 \
        KAFKA_REPLICATION_FACTOR=3 \
        "${SCRIPT_DIR}/kafka_init.sh" >/dev/null 2>&1
    local actual_status=$?
    set -e

    rm -rf "${temp_dir}"

    if [ "${expected_status}" = "success" ] && [ "${actual_status}" -ne 0 ]; then
        fail "${case_name} 应成功，实际退出码为 ${actual_status}"
    fi
    if [ "${expected_status}" = "failure" ] && [ "${actual_status}" -eq 0 ]; then
        fail "${case_name} 应因 Topic 拓扑异常失败"
    fi
}

healthy_topic=$'Topic: mlc.domain.events\tTopicId: id\tPartitionCount: 2\tReplicationFactor: 3\tConfigs:\nTopic: mlc.domain.events\tPartition: 0\tLeader: 1\tReplicas: 1,2,3\tIsr: 1,2,3\nTopic: mlc.domain.events\tPartition: 1\tLeader: 2\tReplicas: 2,3,1\tIsr: 2,3,1'
incomplete_isr=$'Topic: mlc.domain.events\tTopicId: id\tPartitionCount: 2\tReplicationFactor: 3\tConfigs:\nTopic: mlc.domain.events\tPartition: 0\tLeader: 1\tReplicas: 1,2,3\tIsr: 1,2\nTopic: mlc.domain.events\tPartition: 1\tLeader: 2\tReplicas: 2,3,1\tIsr: 2,3,1'

run_case healthy "${healthy_topic}" success
run_case incomplete-isr "${incomplete_isr}" failure

echo "[PASS] Kafka Topic 拓扑校验通过"
