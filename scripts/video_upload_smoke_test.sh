#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
TOKEN="${TOKEN:-}"
VIDEO_FILE="${VIDEO_FILE:-}"
CONCURRENCY="${CONCURRENCY:-1}"

if [[ -z "${TOKEN}" ]]; then
  echo "TOKEN is required" >&2
  exit 1
fi

if [[ -z "${VIDEO_FILE}" || ! -f "${VIDEO_FILE}" ]]; then
  echo "VIDEO_FILE must point to an existing file" >&2
  exit 1
fi

upload_once() {
  curl -sS \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-API-Version: v1" \
    -H "X-Device-ID: smoke-device" \
    -H "X-Client-Type: web" \
    -H "X-Client-Version: smoke" \
    -H "X-Language: zh-CN" \
    -H "X-Request-ID: smoke-${RANDOM}-${RANDOM}" \
    -H "X-Timestamp: $(date +%s)" \
    -F "partNumber=1" \
    -F "file=@${VIDEO_FILE}" \
    "${BASE_URL}/api/v1/video_upload/upload" >/dev/null
}

for ((i = 0; i < CONCURRENCY; i++)); do
  upload_once &
done

wait
echo "video upload smoke test completed: concurrency=${CONCURRENCY}"
