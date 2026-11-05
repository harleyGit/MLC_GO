#!/bin/bash
# 只补齐本机 debug 的独立审核器与 crawler；不关闭或重启已有进程。
set -euo pipefail
umask 077

HG_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HG_RUNTIME="${TMPDIR:-/tmp}/mlc-go-debug-aux"
mkdir -p "${HG_RUNTIME}"

if [[ "${SERVER_ENV:-debug}" != "debug" ]]; then
    echo "[ERROR] 独立服务只允许在 debug 环境自动启动" >&2
    exit 1
fi

# 只读取所需的非敏感开关和监听地址，不 source 含凭据的本机配置文件。
hg_setting() {
    local name="$1" file value
    value="${!name:-}"
    if [[ -n "${value}" ]]; then
        printf '%s' "${value}"
        return
    fi
    for file in "${HG_ROOT}/config/MLC.local.env" "${HG_ROOT}/config/MLC.env"; do
        [[ -f "${file}" ]] || continue
        value="$(awk -v key="${name}" 'index($0,key"=")==1 {print substr($0,length(key)+2); exit}' "${file}")"
        if [[ -n "${value}" ]]; then
            value="${value%\"}"; value="${value#\"}"
            value="${value%\'}"; value="${value#\'}"
            printf '%s' "${value}"
            return
        fi
    done
}

# 仅当健康接口确实返回预期协议时复用；端口被其他进程占用则失败关闭。
hg_ready() {
    local kind="$1" url="$2" result
    if [[ "${kind}" == "review" ]]; then
        result="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 2 "${url}" 2>/dev/null)" || return 1
        [[ "${result}" == "204" ]]
    else
        result="$(curl -fsS --max-time 2 "${url}" 2>/dev/null)" || return 1
        [[ "${result}" == *'"status":"healthy"'* ]]
    fi
}

hg_ensure() {
    local kind="$1" port="$2" url="$3" package="$4" binary="$5"
    shift 5
    if hg_ready "${kind}" "${url}"; then
        echo "[INFO] ${kind} 已就绪，复用当前进程"
        return
    fi
    if lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "[ERROR] ${kind} 端口已占用但服务未就绪；不覆盖现有进程" >&2
        return 1
    fi
    echo "[INFO] 启动缺失的 ${kind} 服务"
    go build -o "${HG_RUNTIME}/${binary}" "${package}"
    if [[ "${kind}" == "review" ]]; then
        SERVER_ENV=debug MLC_CONFIG_DIR="${HG_ROOT}/config" nohup "${HG_RUNTIME}/${binary}" >"${HG_RUNTIME}/${kind}.log" 2>&1 </dev/null &
    else
        SERVER_ENV=debug MLC_CONFIG_DIR="${HG_ROOT}/config" CRAWLER_VIDEO_PIPELINE_ENABLED=true nohup "${HG_RUNTIME}/${binary}" "$@" >"${HG_RUNTIME}/${kind}.log" 2>&1 </dev/null &
    fi
    local pid=$!
    local attempt
    for ((attempt=0; attempt<30; attempt++)); do
        if hg_ready "${kind}" "${url}"; then
            echo "[INFO] ${kind} 已就绪"
            return
        fi
        if ! kill -0 "${pid}" 2>/dev/null; then
            echo "[ERROR] ${kind} 启动失败，请查看 ${HG_RUNTIME}/${kind}.log" >&2
            return 1
        fi
        sleep 1
    done
    echo "[ERROR] ${kind} 等待就绪超时，请查看 ${HG_RUNTIME}/${kind}.log" >&2
    return 1
}

hg_media="$(hg_setting MLC_IM_MEDIA_ENABLED)"
case "${hg_media}" in
    true|1)
        hg_listen="$(hg_setting MLC_IM_REVIEW_LISTEN)"
        hg_listen="${hg_listen:-127.0.0.1:8092}"
        if [[ ! "${hg_listen}" =~ ^127\.0\.0\.1:([0-9]{1,5})$ ]]; then
            echo "[ERROR] 自动启动审核器仅支持本机 127.0.0.1 监听" >&2
            exit 1
        fi
        hg_port="${BASH_REMATCH[1]}"
        if (( 10#${hg_port} < 1 || 10#${hg_port} > 65535 )) || [[ "$(hg_setting MLC_IM_REVIEW_URL)" != "http://${hg_listen}/review" ]]; then
            echo "[ERROR] 审核服务监听与 Go 审核地址不一致" >&2
            exit 1
        fi
        hg_ensure review "${hg_port}" "http://${hg_listen}/readyz" ./cmd/hg_im_review hg-im-review-debug
        ;;
    false|0|"") echo "[INFO] IM 媒体未启用，跳过审核器" ;;
    *) echo "[ERROR] IM 媒体开关不合法" >&2; exit 1 ;;
esac

hg_ensure crawler 8090 http://127.0.0.1:8090/healthz ./cmd/hg_crawler hg-crawler-debug --addr=:8090 --env=debug --config-dir="${HG_ROOT}/config"
