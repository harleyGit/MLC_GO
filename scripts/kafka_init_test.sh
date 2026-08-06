#!/usr/bin/env bash

# 这个文件测试 kafka_init.sh 的 Topic 拓扑校验。
# 它用假 docker 返回预先准备的 kafka-topics.sh --describe 文本，
# 因此既能稳定模拟健康 Topic，也能模拟 ISR 缺失，而不需要破坏真实 Kafka 集群。

# 开启严格模式，让测试中的命令失败和未定义变量立即暴露。
set -euo pipefail

# 取得 scripts 目录绝对路径，确保从任意工作目录执行测试都能找到 kafka_init.sh。
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 统一报告测试失败，并返回非 0 退出码。
fail() {
    echo "[FAIL] $1" >&2
    exit 1
}

# run_case 执行一个 Topic 描述校验场景。
# 第 1 个参数 case_name：场景名称，仅用于失败提示。
# 第 2 个参数 describe_output：假 docker 应返回的 Topic 拓扑文本。
# 第 3 个参数 expected_status：期望 kafka_init.sh 成功还是失败。
run_case() {
    local case_name="$1"
    local describe_output="$2"
    local expected_status="$3"
    local temp_dir
    # 每个场景使用独立临时目录，避免并行或重复执行时相互覆盖。
    temp_dir="$(mktemp -d)"

    # 保存假 docker 命令和本场景的 describe 输出。
    mkdir -p "${temp_dir}/bin"
    printf '%s\n' "${describe_output}" >"${temp_dir}/describe.txt"

    # 创建假 docker：
    # 1. docker inspect 始终成功，并在读取 Running 状态时返回 true；
    # 2. docker exec ... --list 和 --create 默认成功；
    # 3. docker exec ... --describe 时输出当前场景准备的拓扑文本。
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
    # DESCRIBE_OUTPUT_FILE 由测试调用处传入，指向当前场景的 describe.txt。
    cat "${DESCRIBE_OUTPUT_FILE}"
fi
EOF
    # 赋予假 docker 执行权限，否则 PATH 找到文件后也无法运行。
    chmod +x "${temp_dir}/bin/docker"

    # 这里需要主动观察被测脚本的退出码，所以暂时关闭 set -e。
    # 否则“预期失败”的 incomplete-isr 场景会让测试脚本直接退出，来不及判断失败是否符合预期。
    set +e
    # 把假 docker 所在目录放到 PATH 最前面，确保不会访问真实 Docker。
    # 测试样例只有 2 个分区，因此通过环境变量覆盖生产默认的 12 个分区；副本数仍为 3。
    DESCRIBE_OUTPUT_FILE="${temp_dir}/describe.txt" \
        PATH="${temp_dir}/bin:${PATH}" \
        KAFKA_PARTITIONS=2 \
        KAFKA_DANMAKU_PARTITIONS=2 \
        KAFKA_REPLICATION_FACTOR=3 \
        "${SCRIPT_DIR}/kafka_init.sh" >/dev/null 2>&1
    # $? 是上一条命令的退出码，必须在执行其他命令前立即保存。
    local actual_status=$?
    # 退出码已经保存，可以重新打开严格模式。
    set -e

    # 删除当前场景创建的临时文件。
    rm -rf "${temp_dir}"

    # 期望成功但实际非 0，说明健康拓扑被错误拒绝。
    if [ "${expected_status}" = "success" ] && [ "${actual_status}" -ne 0 ]; then
        fail "${case_name} 应成功，实际退出码为 ${actual_status}"
    fi
    # 期望失败但实际为 0，说明异常拓扑没有被校验逻辑发现。
    if [ "${expected_status}" = "failure" ] && [ "${actual_status}" -eq 0 ]; then
        fail "${case_name} 应因 Topic 拓扑异常失败"
    fi
}

# $'...' 允许在 Bash 字符串中使用 \t 和 \n，分别表示 Tab 和换行，
# 用来还原 kafka-topics.sh --describe 的真实输出格式。
# healthy_topic：2 个分区都有 3 个 Replicas 和 3 个 ISR，应通过校验。
healthy_topic=$'Topic: mlc.domain.events\tTopicId: id\tPartitionCount: 2\tReplicationFactor: 3\tConfigs:\nTopic: mlc.domain.events\tPartition: 0\tLeader: 1\tReplicas: 1,2,3\tIsr: 1,2,3\nTopic: mlc.domain.events\tPartition: 1\tLeader: 2\tReplicas: 2,3,1\tIsr: 2,3,1'
# incomplete_isr：分区 0 虽配置 3 副本，但 ISR 只有 1、2 两个 broker，应被拒绝。
incomplete_isr=$'Topic: mlc.domain.events\tTopicId: id\tPartitionCount: 2\tReplicationFactor: 3\tConfigs:\nTopic: mlc.domain.events\tPartition: 0\tLeader: 1\tReplicas: 1,2,3\tIsr: 1,2\nTopic: mlc.domain.events\tPartition: 1\tLeader: 2\tReplicas: 2,3,1\tIsr: 2,3,1'

# 先验证健康拓扑正常通过，再验证 ISR 缺失时正确失败。
run_case healthy "${healthy_topic}" success
run_case incomplete-isr "${incomplete_isr}" failure

echo "[PASS] Kafka Topic 拓扑校验通过"
