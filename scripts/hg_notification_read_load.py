#!/usr/bin/env python3
"""仅对独享隔离环境做有界、只读通知接口压测；绝不隐式启动或连接默认环境。"""

import argparse
import os
import queue
import statistics
import threading
import time
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    """禁止跨主机重定向转发隔离环境 Authorization 头。"""

    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        return None


def main():
    parser = argparse.ArgumentParser(description="隔离环境通知未读读压测；需先人工确认独享依赖")
    parser.add_argument("--base-url", required=True, help="明确的 HTTPS 隔离环境 API 根地址")
    parser.add_argument("--expected-host", required=True, help="人工核对的隔离环境主机名")
    parser.add_argument("--qps", required=True, type=int)
    parser.add_argument("--requests", required=True, type=int)
    parser.add_argument("--workers", required=True, type=int)
    parser.add_argument("--confirm-isolated", required=True, choices=["ISOLATED_AND_AUTHORIZED"])
    args = parser.parse_args()
    parsed = urllib.parse.urlparse(args.base_url)
    if (parsed.scheme != "https" or parsed.hostname != args.expected_host or not parsed.hostname
            or parsed.username or parsed.password or parsed.path not in ("", "/") or parsed.query
            or not (1 <= args.qps <= 100) or not (1 <= args.requests <= 5000)
            or not (1 <= args.workers <= 32)):
        parser.error("只允许显式 HTTPS 主机与有界 QPS/请求数/并发，不允许 URL 凭据或路径")
    token = os.environ.get("MLC_LOAD_JWT", "")
    if not token or "\n" in token or "\r" in token:
        parser.error("需从独享环境的临时凭据设置 MLC_LOAD_JWT，不要在命令行传 Token")

    work = queue.Queue(maxsize=args.workers * 2)
    durations = []
    failures = 0
    lock = threading.Lock()
    url = args.base_url.rstrip("/") + "/api/v1/notifications/unread"
    opener = urllib.request.build_opener(NoRedirect)

    def worker():
        nonlocal failures
        while True:
            item = work.get()
            if item is None:
                work.task_done()
                return
            began = time.monotonic()
            request = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
            ok = False
            try:
                with opener.open(request, timeout=5) as response:
                    response.read(4096)
                    ok = response.status == 200
            except (urllib.error.URLError, TimeoutError, ValueError):
                pass
            with lock:
                durations.append((time.monotonic() - began) * 1000)
                if not ok:
                    failures += 1
            work.task_done()

    threads = [threading.Thread(target=worker, daemon=True) for _ in range(args.workers)]
    for thread in threads:
        thread.start()
    start = time.monotonic()
    for index in range(args.requests):
        remaining = start + index / args.qps - time.monotonic()
        if remaining > 0:
            time.sleep(remaining)
        work.put(index)
    for _ in threads:
        work.put(None)
    work.join()
    for thread in threads:
        thread.join()
    durations.sort()
    elapsed = time.monotonic() - start

    def percentile(p):
        return round(durations[min(len(durations) - 1, int((len(durations) - 1) * p))], 2)

    print({"requests": len(durations), "errors": failures, "elapsed_seconds": round(elapsed, 2),
           "observed_qps": round(len(durations) / elapsed, 2), "p50_ms": percentile(.5),
           "p95_ms": percentile(.95), "p99_ms": percentile(.99),
           "mean_ms": round(statistics.fmean(durations), 2)})


if __name__ == "__main__":
    main()
