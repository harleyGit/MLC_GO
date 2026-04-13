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

# 这里先定义一个默认的环境文件路径。
# 后面会根据 TARGET_ENV 再切换成对应环境自己的 env 文件。
ENV_FILE="${WORKSPACE_DIR}/config/env_configs/hg_debug.env"
PRE_COMPOSE_FILE="${WORKSPACE_DIR}/config/docker/hg_docker_compose.pre.yml"

# 下面先定义默认值。
# 如果后面没有读到配置文件，脚本就用这些默认值继续执行。
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_USER="root"
MYSQL_PASSWORD=""
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
        ENV_FILE="${WORKSPACE_DIR}/config/env_configs/hg_debug.env"
        ALLOW_AUTO_START="true"
        ;;
    pre)
        ENV_FILE="${WORKSPACE_DIR}/config/env_configs/hg_pre.env"
        ALLOW_AUTO_START="true"
        ;;
    prod)
        ENV_FILE="${WORKSPACE_DIR}/config/env_configs/hg_prod.env"
        ALLOW_AUTO_START="false"
        ;;
    *)
        echo "[ERROR] 不支持的环境：${TARGET_ENV}。可选值：debug / pre / prod" >&2
        exit 1
        ;;
esac

# if [ -f 文件路径 ] 的意思是：
# “如果这个路径存在，并且它是一个普通文件，就执行 then 里的逻辑”。
# 这里是在判断当前目标环境对应的 env 文件是否存在。
if [ -f "${ENV_FILE}" ]; then
    # while IFS='=' read -r key value 的作用是：
    # 按行读取文件内容，并且按 = 拆成左边 key、右边 value。
    # 比如 MYSQL_HOST=127.0.0.1
    # 就会拆成：
    #   key   -> MYSQL_HOST
    #   value -> 127.0.0.1
    while IFS='=' read -r key value; do
        # case 是 Bash 的多分支判断，类似很多语言里的 switch。
        # 根据 key 的名字，把对应的 value 赋值给本脚本变量。
        case "${key}" in
            MYSQL_HOST) MYSQL_HOST="${value}" ;;
            MYSQL_PORT) MYSQL_PORT="${value}" ;;
            MYSQL_USER) MYSQL_USER="${value}" ;;
            MYSQL_PASSWORD) MYSQL_PASSWORD="${value}" ;;
            REDIS_HOST) REDIS_HOST="${value}" ;;
            REDIS_PORT) REDIS_PORT="${value}" ;;
        esac
    # done < <(...) 叫“进程替换”。
    # 可以理解为：把括号里命令的输出，当作 while 循环的输入。
    #
    # grep -v '^[[:space:]]*#'：去掉注释行
    # grep '='：只保留包含等号的配置行
    #
    # 这里没有直接 source 整个 env 文件，是为了只取这个脚本真正需要的几个配置，
    # 减少额外变量带来的副作用。
    done < <(grep -v '^[[:space:]]*#' "${ENV_FILE}" | grep '=')
fi

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
    # 先定义一个数组，把 mysqladmin 需要的参数放进去。
    # 数组写法：
    # local mysqladmin_args=( 参数1 参数2 参数3 )
    local mysqladmin_args=(
        -h"${MYSQL_HOST}"
        -P"${MYSQL_PORT}"
        -u"${MYSQL_USER}"
    )

    # [ -n "${MYSQL_PASSWORD}" ] 表示“如果字符串非空”。
    # 只有当密码不为空时，才把 -p密码 追加进去。
    if [ -n "${MYSQL_PASSWORD}" ]; then
        mysqladmin_args+=(-p"${MYSQL_PASSWORD}")
    fi

    # "${mysqladmin_args[@]}" 表示把数组中的每个参数原样展开。
    # --silent 表示静默模式。
    # 最终只通过退出码判断是否成功，不打印多余信息。
    mysqladmin ping "${mysqladmin_args[@]}" --silent >/dev/null 2>&1
}

# start_mysql 的作用是：当 MySQL 没起来时，尝试把它启动起来。
# 启动顺序是：
# 1. 优先尝试 brew services start mysql
# 2. 如果不行，再尝试 mysql.server start
# 这样兼容本机常见的两种 MySQL 安装方式。
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
    if command -v mysql.server >/dev/null 2>&1; then
        if mysql.server start >/dev/null 2>&1; then
            log_info "已执行 mysql.server start"
            return 0
        fi
    fi

    # 两种方式都没成功，就返回失败。
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

# check_redis 的作用是检查 Redis 是否可用。
# redis-cli ping 如果正常，一般会返回 PONG。
# grep -q "PONG" 表示只检查是否包含这个字符串，不把内容打印到屏幕。
check_redis() {
    redis-cli -h "${REDIS_HOST}" -p "${REDIS_PORT}" ping 2>/dev/null | grep -q "PONG"
}

# start_redis 的作用是尝试启动 Redis。
# 当前这里只走 brew services start redis，
# 因为你这个项目目前 Redis 默认就是连接本机 6379，
# 本地最常见的 Redis 启动方式也是 Homebrew。
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

# main 是脚本主入口函数。
# 这里先确保 MySQL 就绪，再确保 Redis 就绪。
# 只要其中任何一步失败，整个脚本就会失败。
# 而这个脚本是 VS Code debug 的 preLaunchTask，
# 所以一旦脚本失败，调试就会被中断，不会继续启动 Go 程序。
main() {
    log_info "开始检查 ${TARGET_ENV} 环境依赖服务"
    log_info "当前使用环境文件：${ENV_FILE}"
    ensure_mysql_ready
    ensure_redis_ready
    log_info "${TARGET_ENV} 环境依赖服务已就绪"
}

# 这一行是真正执行 main 函数。
# 当前脚本已经在开头自己读取了第 1 个参数作为环境名，
# 所以这里直接调用 main 即可。
main
