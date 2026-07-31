#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

fail() {
    echo "[FAIL] $1" >&2
    exit 1
}

assert_contains() {
    local expected="$1"
    local file="$2"

    if ! grep -Fq -- "${expected}" "${file}"; then
        fail "未找到预期命令：${expected}"
    fi
}

assert_not_contains() {
    local unexpected="$1"
    local file="$2"

    if grep -Fq -- "${unexpected}" "${file}"; then
        fail "发现不应执行的命令：${unexpected}"
    fi
}

run_case() {
    local case_name="$1"
    local stopped_container="${2:-}"
    local temp_dir
    temp_dir="$(mktemp -d)"

    mkdir -p "${temp_dir}/bin"
    : >"${temp_dir}/commands.log"

    cat >"${temp_dir}/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "docker $*" >>"${COMMAND_LOG}"
if [ "${1:-}" = "inspect" ]; then
    container="${@: -1}"
    if [ -n "${STOPPED_CONTAINER:-}" ] && [ "${container}" = "${STOPPED_CONTAINER}" ]; then
        exit 1
    fi
    echo "true"
fi
EOF

    cat >"${temp_dir}/bin/make" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "make $*" >>"${COMMAND_LOG}"
EOF

    cat >"${temp_dir}/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ " $* " != *" --check "* ]] && [[ " $* " != *" --migrations-dir "* ]]; then
    printf '%s\n' 127.0.0.1 3306 127.0.0.1 6379
fi
EOF

    cat >"${temp_dir}/bin/migrate" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

    chmod +x "${temp_dir}/bin/docker" "${temp_dir}/bin/make" "${temp_dir}/bin/go" "${temp_dir}/bin/migrate"

    COMMAND_LOG="${temp_dir}/commands.log" \
        STOPPED_CONTAINER="${stopped_container}" \
        PATH="${temp_dir}/bin:${PATH}" \
        "${SCRIPT_DIR}/ensure_debug_deps.sh" debug >/dev/null

    assert_contains "make kafka-init" "${temp_dir}/commands.log"

    case "${case_name}" in
        running)
            assert_not_contains "docker compose" "${temp_dir}/commands.log"
            ;;
        stopped)
            assert_contains "docker compose -f ${WORKSPACE_DIR}/deployments/docker-compose.kafka.yml up -d kafka-1 kafka-2 kafka-3" "${temp_dir}/commands.log"
            ;;
        *)
            fail "未知测试场景：${case_name}"
            ;;
    esac

    rm -rf "${temp_dir}"
}

run_case running
run_case stopped mlc-kafka-2

echo "[PASS] VS Code debug 前置 Kafka 检查通过"
