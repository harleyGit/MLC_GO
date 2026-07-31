#!/usr/bin/env bash

# 这个文件测试 VS Code 前置脚本对 Kafka 的判断逻辑。
# 测试不会真的启动或停止 Docker、MySQL、Redis、Kafka，而是在临时目录中创建同名“假命令”。
# 被测脚本执行 docker、make、go、migrate 时，实际调用的是这些假命令，因此测试安全且可重复。

# 开启严格模式，任何断言失败、变量拼错或管道失败都会立即结束测试。
set -euo pipefail

# SCRIPT_DIR 是当前测试脚本所在的 scripts 目录。
# WORKSPACE_DIR 是 scripts 的上一级，也就是项目根目录，用来拼出预期 compose 文件绝对路径。
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 统一输出失败信息并用退出码 1 结束测试。
# shell 约定退出码 0 表示成功，非 0 表示失败。
fail() {
    echo "[FAIL] $1" >&2
    exit 1
}

# 断言日志文件中必须出现某条完整命令。
# grep -F 按普通字符串查找，不把命令中的符号当正则表达式；-q 表示只返回结果，不打印内容。
assert_contains() {
    local expected="$1"
    local file="$2"

    if ! grep -Fq -- "${expected}" "${file}"; then
        fail "未找到预期命令：${expected}"
    fi
}

# 断言日志文件中不能出现某条命令。
# 主要用于确认 Kafka 已运行时不会重复执行 docker compose up。
assert_not_contains() {
    local unexpected="$1"
    local file="$2"

    if grep -Fq -- "${unexpected}" "${file}"; then
        fail "发现不应执行的命令：${unexpected}"
    fi
}

# run_case 执行一个测试场景。
# 第 1 个参数 case_name 表示场景名称：running 或 stopped。
# 第 2 个参数 stopped_container 可选，用来指定哪个 Kafka 容器应被模拟成“未运行”。
run_case() {
    local case_name="$1"
    local stopped_container="${2:-}"
    local temp_dir
    # mktemp -d 创建只属于当前场景的临时目录，避免不同测试之间相互污染。
    temp_dir="$(mktemp -d)"

    # bin 目录放假命令，commands.log 记录被测脚本实际调用了哪些命令。
    mkdir -p "${temp_dir}/bin"
    # “: >文件”会创建空文件，文件已存在时则清空内容。
    : >"${temp_dir}/commands.log"

    # 下面的 heredoc 会生成一个名为 docker 的假可执行文件。
    # <<'EOF' 使用单引号，表示生成文件时不展开里面的 $变量，等假命令运行时再展开。
    cat >"${temp_dir}/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
# 无论收到什么参数，都把完整 docker 命令写入日志，供后面的断言检查。
echo "docker $*" >>"${COMMAND_LOG}"
if [ "${1:-}" = "inspect" ]; then
    # ${@: -1} 取得最后一个参数，也就是当前被检查的容器名。
    container="${@: -1}"
    # 如果容器名与测试指定的 STOPPED_CONTAINER 相同，就返回失败，模拟容器不存在或未运行。
    if [ -n "${STOPPED_CONTAINER:-}" ] && [ "${container}" = "${STOPPED_CONTAINER}" ]; then
        exit 1
    fi
    # 其他容器输出 true，模拟 docker inspect 返回 Running=true。
    echo "true"
fi
EOF

    # 假 make 不执行真实 Makefile，只记录是否调用了 make kafka-init。
    cat >"${temp_dir}/bin/make" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "make $*" >>"${COMMAND_LOG}"
EOF

    # 被测前置脚本还会用 go run 读取配置、检查 MySQL/Redis 和执行迁移。
    # 读取配置时按真实命令格式返回四行 host/port；检查和迁移命令直接成功且不输出内容。
    cat >"${temp_dir}/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ " $* " != *" --check "* ]] && [[ " $* " != *" --migrations-dir "* ]]; then
    printf '%s\n' 127.0.0.1 3306 127.0.0.1 6379
fi
EOF

    # 前置脚本会用 command -v migrate 检查迁移工具是否存在。
    # 放置这个假命令即可通过存在性检查，真正迁移仍由上面的假 go 命令模拟。
    cat >"${temp_dir}/bin/migrate" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

    # 新建文件默认没有执行权限，chmod +x 后才能像普通命令一样运行。
    chmod +x "${temp_dir}/bin/docker" "${temp_dir}/bin/make" "${temp_dir}/bin/go" "${temp_dir}/bin/migrate"

    # 把临时 bin 放到 PATH 最前面，Shell 会优先找到假命令，而不是系统中的真实命令。
    # COMMAND_LOG 和 STOPPED_CONTAINER 只对本次命令生效，不会修改用户终端的永久环境变量。
    # >/dev/null 隐藏被测脚本普通日志，让测试输出只保留 PASS 或 FAIL。
    COMMAND_LOG="${temp_dir}/commands.log" \
        STOPPED_CONTAINER="${stopped_container}" \
        PATH="${temp_dir}/bin:${PATH}" \
        "${SCRIPT_DIR}/ensure_debug_deps.sh" debug >/dev/null

    # 无论 Kafka 原来是否运行，最终都必须执行幂等 Topic 初始化和拓扑检查。
    assert_contains "make kafka-init" "${temp_dir}/commands.log"

    # running 场景要求跳过 compose；stopped 场景要求只启动三个 Kafka broker。
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

    # 当前场景完成后删除假命令和日志，不在仓库中留下测试产物。
    rm -rf "${temp_dir}"
}

# 场景一：三个容器都返回 Running=true，不能重复执行 docker compose。
run_case running
# 场景二：模拟 mlc-kafka-2 未运行，必须执行限定三个 broker 的 compose 启动命令。
run_case stopped mlc-kafka-2

echo "[PASS] VS Code debug 前置 Kafka 检查通过"
