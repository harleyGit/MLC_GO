#!/bin/bash

# 这行叫“严格模式”设置。
# set 是 Bash 内置命令，用来打开一些脚本执行选项。
# -e：只要某个命令执行失败，脚本就立刻退出，不再往后跑。
# -u：如果用了一个还没定义的变量，直接报错退出。
# -o pipefail：如果出现 a | b 这样的管道，只要其中任何一段失败，就认为整条失败。
# 这样可以减少“前面已经错了，后面还继续执行”的问题。
set -euo pipefail

# SCRIPT_DIR 表示“当前脚本文件自己所在的目录”。
# dirname "${BASH_SOURCE[0]}" 会先拿到当前脚本路径，再取它的目录部分。
# cd 到这个目录后，再用 pwd 拿到绝对路径。
# 最终得到的通常是：.../MLC_GO/scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# WORKSPACE_DIR 表示项目根目录。
# 因为脚本位于 scripts 目录下，所以 scripts 的上一级就是工程根目录。
WORKSPACE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# TARGET_ENV 表示这次脚本要按哪个环境工作。
# $1 表示脚本收到的第 1 个命令行参数。
# 例如：
#   ./scripts/ensure_debug_deps.sh debug
#   ./scripts/ensure_debug_deps.sh pre
#   ./scripts/ensure_debug_deps.sh prod
# 如果外部没有传参数，就默认按 debug 处理。
TARGET_ENV="${1:-debug}"

# 基础设施连接信息统一来自模块化 YAML，不再读取分环境 .env。
CONFIG_DIR="${WORKSPACE_DIR}/config"
PRE_COMPOSE_FILE="${WORKSPACE_DIR}/config/docker/hg_docker_compose.pre.yml"
KAFKA_COMPOSE_FILE="${WORKSPACE_DIR}/deployments/docker-compose.kafka.yml"
KAFKA_CONTAINERS=("mlc-kafka-1" "mlc-kafka-2" "mlc-kafka-3")

# 下面先定义默认值。
# 如果后面没有读到配置文件，脚本就用这些默认值继续执行。
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
REDIS_HOST="127.0.0.1"
REDIS_PORT="6379"
ALLOW_AUTO_START="true"

# case 是多分支判断。
# 这里根据目标环境，决定：
# 1. 要读取哪一个 env 文件
# 2. 当前环境是否允许“自动启动服务”
#
# 设计约定：
# - debug：允许自动启动本机 MySQL / Redis
# - pre：本地模拟 pre 时允许通过 docker compose 自动启动
# - prod：只检查，不自动启动，避免误操作生产环境
case "${TARGET_ENV}" in
    debug)
        ALLOW_AUTO_START="true"
        ;;
    pre)
        ALLOW_AUTO_START="true"
        ;;
    prod)
        ALLOW_AUTO_START="false"
        ;;
    *)
        echo "[ERROR] 不支持的环境：${TARGET_ENV}。可选值：debug / pre / prod" >&2
        exit 1
        ;;
esac

CONFIG_VALUES=()
while IFS= read -r value; do
    CONFIG_VALUES+=("${value}")
done < <(go run "${WORKSPACE_DIR}/cmd/hg_config_check" --env "${TARGET_ENV}" --config-dir "${CONFIG_DIR}")

if [ "${#CONFIG_VALUES[@]}" -ne 4 ]; then
    echo "[ERROR] 读取 ${TARGET_ENV} 基础设施配置失败" >&2
    exit 1
fi

MYSQL_HOST="${CONFIG_VALUES[0]}"
MYSQL_PORT="${CONFIG_VALUES[1]}"
REDIS_HOST="${CONFIG_VALUES[2]}"
REDIS_PORT="${CONFIG_VALUES[3]}"

# log_info 是一个函数。
# 调用方式例如：log_info "MySQL 已运行"
# $1 表示传给函数的第 1 个参数。
# 这里统一输出普通信息日志，方便在 VS Code task 面板里看执行过程。
log_info() {
    echo "[INFO] $1"
}

# log_error 和 log_info 类似，但它把内容输出到标准错误。
# >&2 的意思是“把 echo 的输出重定向到 stderr（错误输出）”。
# 这样普通日志和错误日志更容易区分。
log_error() {
    echo "[ERROR] $1" >&2
}

# can_auto_start 用来判断当前环境是否允许自动拉起依赖服务。
# 约定上：
# - debug 可以自动启动
# - pre 也可以自动启动，但走的是本地 docker compose 方案
# - prod 默认不自动启动，只做检查
can_auto_start() {
    [ "${ALLOW_AUTO_START}" = "true" ]
}

# is_apple_silicon_mac 用来判断当前是不是 macOS 的 Apple Silicon 机器。
# 这里主要是为了兼容你本地这种 M2 / arm64 场景：
# 有些机器上 brew services start mysql 启动的是 Homebrew 安装的实例，
# 但你实际可用的却是 /usr/local/mysql/support-files/mysql.server 对应的实例。
is_apple_silicon_mac() {
    [ "$(uname -s)" = "Darwin" ] && [ "$(uname -m)" = "arm64" ]
}

# can_prompt_for_sudo 用来判断当前脚本是不是跑在可交互终端里。
# 只有标准输入和标准输出都连接到终端时，sudo 才适合弹出密码输入。
# 如果脚本运行在不能交互的场景，就不要直接执行 sudo mysql.server start，
# 否则任务可能会卡在密码提示上。
can_prompt_for_sudo() {
    [ -t 0 ] && [ -t 1 ]
}

# docker compose 方式更适合本地模拟 pre 环境：
# 因为 pre 需要一整套和 debug 隔离的依赖端口，
# 所以这里用 compose 统一拉起 MySQL / Redis。
start_pre_compose_services() {
    # 只有 pre 环境才需要走这里。
    if [ "${TARGET_ENV}" != "pre" ]; then
        return 1
    fi

    # 如果 compose 文件不存在，就直接失败，避免输出误导信息。
    if [ ! -f "${PRE_COMPOSE_FILE}" ]; then
        log_error "pre 环境 compose 文件不存在：${PRE_COMPOSE_FILE}"
        return 1
    fi

    # 先判断 docker 命令是否存在。
    if ! command -v docker >/dev/null 2>&1; then
        log_error "未找到 docker 命令，无法自动启动 pre 环境依赖"
        return 1
    fi

    # 用 docker compose 同时拉起 mysql 和 redis。
    # 这里用 -d 表示后台运行，这样不会阻塞 VS Code 的 preLaunchTask。
    if docker compose -f "${PRE_COMPOSE_FILE}" up -d mysql redis >/dev/null 2>&1; then
        log_info "已执行 docker compose 启动 pre 环境 MySQL / Redis"
        return 0
    fi

    return 1
}

# wait_for_port 是一个通用等待函数。
# 它负责反复检查某个 host:port 是否已经能连通。
# 参数说明：
#   $1 -> host，例如 127.0.0.1
#   $2 -> port，例如 3306
#   $3 -> service_name，例如 MySQL
wait_for_port() {
    # local 表示这些变量只在当前函数内部有效，
    # 不会污染外部同名变量。
    local host="$1"
    local port="$2"
    local service_name="$3"

    # seq 1 30 会生成 1 到 30。
    # for 循环表示最多重试 30 次，每次间隔 1 秒，也就是最多等 30 秒。
    for _ in $(seq 1 30); do
        # nc -z host port 用来测试端口是否可连接。
        # >/dev/null 2>&1 的意思是：
        #   标准输出不要显示
        #   标准错误也不要显示
        # 这里只关心成功还是失败，不关心具体输出内容。
        if nc -z "${host}" "${port}" >/dev/null 2>&1; then
            log_info "${service_name} 端口 ${host}:${port} 已可用"
            return 0
        fi

        # 如果这次还没连通，就先等 1 秒再继续下一次重试。
        sleep 1
    done

    # return 1 表示函数执行失败。
    # Bash 中通常 return 0 表示成功，非 0 表示失败。
    return 1
}

# check_mysql 的作用是检查 MySQL 是否真的可用。
# 这里不用单纯的端口探测，而用 mysqladmin ping，
# 因为端口打开不代表 MySQL 已经完全就绪，也不代表认证参数一定正确。
check_mysql() {
    go run "${WORKSPACE_DIR}/cmd/hg_config_check" --env "${TARGET_ENV}" --config-dir "${CONFIG_DIR}" --check mysql >/dev/null 2>&1
}

# start_mysql 的作用是：当 MySQL 没起来时，尝试把它启动起来。
# 启动顺序是：
# 1. pre 环境优先尝试 docker compose
# 2. Apple Silicon 机器优先尝试 mysql.server，因为这类机器上常见的是 sudo mysql.server start
# 3. 其他场景优先尝试 brew services start mysql
# 4. 如果还不行，再尝试 sudo -n mysql.server start
#
# 如果当前终端支持交互，则允许执行 sudo mysql.server start，
# 这样像你当前 M2 机器这种“必须手动输入密码”的场景也能直接拉起。
# 如果当前终端不支持交互，则仍然只尝试 sudo -n，避免后台任务卡死。
start_mysql() {
    log_info "MySQL 未就绪，尝试启动"

    # 如果当前环境策略不允许自动启动，就直接返回失败。
    # 这样 pre / prod 就只会检查，不会擅自拉服务。
    if ! can_auto_start; then
        log_error "当前环境 ${TARGET_ENV} 不允许自动启动 MySQL，请先确认服务已启动"
        return 1
    fi

    # pre 环境优先走 docker compose。
    # 因为 pre 需要和 debug 隔离端口，且 MySQL / Redis 最好一起启动。
    if [ "${TARGET_ENV}" = "pre" ]; then
        if start_pre_compose_services; then
            return 0
        fi
    fi

    # Apple Silicon 机器优先尝试 mysql.server。
    # 这样可以兼容你当前这种需要执行 sudo mysql.server start 的 MySQL 安装方式。
    if is_apple_silicon_mac && command -v mysql.server >/dev/null 2>&1; then
        # 如果当前任务跑在可交互终端里，就直接允许 sudo 弹密码。
        # VS Code integratedTerminal 一般属于这种场景，输入一次密码后即可继续。
        if can_prompt_for_sudo; then
            log_info "检测到 Apple Silicon macOS，优先尝试 sudo mysql.server start"
            if sudo mysql.server start; then
                log_info "已执行 sudo mysql.server start"
                return 0
            fi
        fi

        # 如果当前不是可交互场景，则退回到 sudo -n。
        # 这样不会因为无法输入密码而把任务挂住。
        if sudo -n mysql.server start >/dev/null 2>&1; then
            log_info "已执行 sudo -n mysql.server start"
            return 0
        fi
    fi

    # command -v brew 用来检查 brew 命令是否存在。
    # 如果存在，再尝试用 Homebrew 的服务管理命令启动 MySQL。
    if command -v brew >/dev/null 2>&1; then
        if brew services start mysql >/dev/null 2>&1; then
            log_info "已执行 brew services start mysql"
            return 0
        fi
    fi

    # 如果 brew 方式没有成功，再检查 mysql.server 这个命令是否存在。
    # 一些 MySQL 安装方式会提供 mysql.server 启停命令。
    #
    # 非 Apple Silicon 的场景依然优先保持原来的保守策略：
    # 先尝试非交互 sudo，避免不能输入密码的任务被卡住；
    # 如果当前终端可交互，再允许你手动输入 sudo 密码。
    if command -v mysql.server >/dev/null 2>&1; then
        if sudo -n mysql.server start >/dev/null 2>&1; then
            log_info "已执行 sudo -n mysql.server start"
            return 0
        fi

        if can_prompt_for_sudo; then
            log_info "检测到当前终端可交互，尝试 sudo mysql.server start，可能需要输入密码"
            if sudo mysql.server start; then
                log_info "已执行 sudo mysql.server start"
                return 0
            fi
        fi
    fi

    # 两种自动方式都没成功，就给出更明确的说明。
    # 这样你在 VS Code 中看到报错时，能立刻知道下一步该做什么。
    log_error "MySQL 自动启动失败。"
    log_error "如果你的 MySQL 启动方式需要 sudo 密码，请先在终端手动执行：sudo mysql.server start"
    log_error "手动启动成功后，再回到 VS Code 点击绿色启动按钮。"

    return 1
}

# ensure_mysql_ready 表示“确保 MySQL 就绪”。
# 它不是单纯检查一下，而是一个完整流程：
# 1. 先看 MySQL 是不是已经可用了
# 2. 如果没好，就尝试启动
# 3. 启动后等待端口可连接
# 4. 再做一次真正的 mysqladmin ping 校验
ensure_mysql_ready() {
    # 如果 MySQL 本来就已经是好的，就直接返回，不做多余动作。
    if check_mysql; then
        log_info "MySQL 已运行"
        return 0
    fi

    # 如果 MySQL 没启动，先在终端给出手动连接提示，
    # 方便排查账号权限、密码或本机服务异常问题。
    log_info "检测到 MySQL 未启动，可先手动执行：mysql -uroot"

    # 如果启动失败，就输出错误并返回失败。
    if ! start_mysql; then
        log_error "MySQL 启动失败，请先手动启动后再调试"
        return 1
    fi

    # 启动命令执行成功，不代表服务已经瞬间可用，
    # 所以这里还要轮询等待端口真正起来。
    if ! wait_for_port "${MYSQL_HOST}" "${MYSQL_PORT}" "MySQL"; then
        log_error "MySQL 端口未在预期时间内就绪"
        return 1
    fi

    # 端口打开后，再做一次 mysqladmin ping，
    # 防止出现“端口开了但 MySQL 其实还没完全 ready”的情况。
    if ! check_mysql; then
        log_error "MySQL 端口已打开，但认证或服务状态校验失败"
        return 1
    fi

    log_info "MySQL 检查通过"
}

# debug 调试直接依赖仓库最新 schema；依赖探活后执行幂等迁移，避免数据库版本落后导致应用立即退出。
# pre/prod 不在本机调试任务中自动改库，防止对共享或生产数据库产生意外 DDL。
ensure_debug_database_migrated() {
    if [ "${TARGET_ENV}" != "debug" ]; then
        return 0
    fi

    if ! command -v migrate >/dev/null 2>&1; then
        log_error "未找到 migrate 命令，请先安装 golang-migrate"
        return 1
    fi

    log_info "开始执行 debug 数据库迁移"
    go run "${WORKSPACE_DIR}/cmd/hg_config_check" \
        --env "${TARGET_ENV}" \
        --config-dir "${CONFIG_DIR}" \
        --migrations-dir "${WORKSPACE_DIR}/migrations"
    log_info "debug 数据库迁移已完成"
}

# check_redis 的作用是检查 Redis 是否可用。
# redis-cli ping 如果正常，一般会返回 PONG。
# grep -q "PONG" 表示只检查是否包含这个字符串，不把内容打印到屏幕。
check_redis() {
    go run "${WORKSPACE_DIR}/cmd/hg_config_check" --env "${TARGET_ENV}" --config-dir "${CONFIG_DIR}" --check redis >/dev/null 2>&1
}

# start_redis 的作用是尝试启动 Redis。
# 启动顺序是：
# 1. pre 环境优先走 docker compose
# 2. 先尝试直接执行 redis-server
# 3. 如果还不行，再尝试 brew services start redis
#
# 这里之所以不是把 redis-server 直接跑在前台，
# 是因为当前脚本会被 VS Code 的 preLaunchTask 调用。
# 如果 redis-server 一直占住前台，脚本就无法继续往下执行。
# 所以这里采用“后台启动”的方式，既满足执行 redis-server，
# 也避免阻塞后续的依赖检查和调试启动。
start_redis() {
    log_info "Redis 未就绪，尝试启动"

    # 和 MySQL 一样，是否允许自动启动，取决于当前环境策略。
    if ! can_auto_start; then
        log_error "当前环境 ${TARGET_ENV} 不允许自动启动 Redis，请先确认服务已启动"
        return 1
    fi

    # pre 环境下，Redis 也交给 docker compose 统一启动。
    # 如果 MySQL 检查阶段已经拉起过 compose，这里通常很快就会通过。
    if [ "${TARGET_ENV}" = "pre" ]; then
        if start_pre_compose_services; then
            return 0
        fi
    fi

    # 如果本机存在 redis-server，则优先直接用它拉起 Redis。
    # nohup + 后台符号 & 的组合表示：
    # 即使当前启动命令返回后，Redis 进程也继续在后台运行。
    # 日志重定向到 /tmp，避免把 task 面板刷满。
    if command -v redis-server >/dev/null 2>&1; then
        if nohup redis-server >/tmp/mlc_go_redis_server.log 2>&1 & then
            log_info "已执行 redis-server（后台启动）"
            return 0
        fi
    fi

    # 先判断 brew 是否存在，再尝试启动 Redis 服务。
    if command -v brew >/dev/null 2>&1; then
        if brew services start redis >/dev/null 2>&1; then
            log_info "已执行 brew services start redis"
            return 0
        fi
    fi

    # 如果没有 brew 或启动失败，则返回失败。
    return 1
}

# ensure_redis_ready 和 ensure_mysql_ready 思路一样：
# 1. 先检查 Redis 是否已运行
# 2. 没运行就尝试启动
# 3. 启动后等端口 ready
# 4. 再用 redis-cli ping 做最终确认
ensure_redis_ready() {
    if check_redis; then
        log_info "Redis 已运行"
        return 0
    fi

    if ! start_redis; then
        log_error "Redis 启动失败，请先手动启动后再调试"
        return 1
    fi

    if ! wait_for_port "${REDIS_HOST}" "${REDIS_PORT}" "Redis"; then
        log_error "Redis 端口未在预期时间内就绪"
        return 1
    fi

    if ! check_redis; then
        log_error "Redis 端口已打开，但服务状态校验失败"
        return 1
    fi

    log_info "Redis 检查通过"
}

# 本地 debug 依赖固定先保证三节点 Kafka 已运行，再显式初始化业务 Topic。
# pre/prod 的 Topic 必须由发布系统或 Kafka IaC 管理，本脚本不自动操作共享环境。
ensure_debug_kafka_ready() {
    if [ "${TARGET_ENV}" != "debug" ]; then
        return 0
    fi

    if ! command -v docker >/dev/null 2>&1; then
        log_error "未找到 docker 命令，无法检查本地 Kafka"
        return 1
    fi

    local container
    local kafka_running="true"
    for container in "${KAFKA_CONTAINERS[@]}"; do
        if [ "$(docker inspect -f '{{.State.Running}}' "${container}" 2>/dev/null || true)" != "true" ]; then
            kafka_running="false"
            break
        fi
    done

    if [ "${kafka_running}" = "true" ]; then
        log_info "Kafka 三节点集群已运行，无需重复启动"
    else
        log_info "Kafka 未完整运行，执行本地 compose 启动"
        # compose 文件还包含监控和统计依赖；这里只启动 broker，避免无关端口冲突。
        docker compose -f "${KAFKA_COMPOSE_FILE}" up -d kafka-1 kafka-2 kafka-3
    fi

    # kafka-init 会等待 broker 就绪，并校验 Topic 的分区数、副本数和 ISR。
    make kafka-init
    log_info "Kafka 业务 Topic 已就绪"
}

# main 是脚本主入口函数。
# 这里依次确保 MySQL、Redis 和本地 debug Kafka 就绪。
# 只要其中任何一步失败，整个脚本就会失败。
# 而这个脚本是 VS Code debug 的 preLaunchTask，
# 所以一旦脚本失败，调试就会被中断，不会继续启动 Go 程序。
main() {
    log_info "开始检查 ${TARGET_ENV} 环境依赖服务"
    log_info "当前使用模块配置：${CONFIG_DIR}/${TARGET_ENV}/mysql.yaml、redis.yaml"
    ensure_mysql_ready
    ensure_debug_database_migrated
    ensure_redis_ready
    ensure_debug_kafka_ready
    log_info "${TARGET_ENV} 环境依赖服务已就绪"
}

# 这一行是真正执行 main 函数。
# 当前脚本已经在开头自己读取了第 1 个参数作为环境名，
# 所以这里直接调用 main 即可。
main
