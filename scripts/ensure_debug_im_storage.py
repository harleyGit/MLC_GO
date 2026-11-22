#!/usr/bin/env python3
"""仅恢复本机 debug 的既有 IM S3 容器及其最小权限配置，不创建桶或更改密钥。"""

import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time


HG_ROOT = Path(__file__).resolve().parent.parent
HG_CONTAINER = "mlc-im-debug-seaweed-v4"
HG_IMAGE = "chrislusf/seaweedfs@sha256:f7a49e3f3972ebed5be2ed4befa4a0c68eb7bdddba6e692d42729ceb545134b8"
HG_CLAMAV_IMAGE = "clamav/clamav@sha256:75fb5fd95fcbe1d7e6d240c369c1572b686ee2c95949d1042b5148de8eddebb4"
HG_DIR = Path(os.environ.get("TMPDIR", "/tmp")).resolve() / "opencode/mlc-im-debug"
HG_CONFIG = HG_DIR / "s3.json"
HG_KEYS = (
    "MLC_IM_S3_BUCKET",
    "MLC_IM_S3_ACCESS_KEY",
    "MLC_IM_S3_SECRET_KEY",
    "MLC_IM_REVIEW_S3_ACCESS_KEY",
    "MLC_IM_REVIEW_S3_SECRET_KEY",
)


def hg_setting(name):
    if os.environ.get(name):
        return os.environ[name]
    for file in (HG_ROOT / "config/MLC.local.env", HG_ROOT / "config/MLC.env"):
        if not file.is_file():
            continue
        for line in file.read_text().splitlines():
            if line.startswith(name + "="):
                return line.split("=", 1)[1].strip().strip("'\"")
    return ""


def hg_expected():
    values = {name: hg_setting(name) for name in HG_KEYS}
    bucket = values["MLC_IM_S3_BUCKET"]
    if (not all(values.values())
            or not re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", bucket)
            or values["MLC_IM_S3_ACCESS_KEY"] == values["MLC_IM_REVIEW_S3_ACCESS_KEY"]
            or values["MLC_IM_S3_SECRET_KEY"] == values["MLC_IM_REVIEW_S3_SECRET_KEY"]):
        raise ValueError("本机 IM 存储身份缺失、重复或桶名不合法")
    account = {"id": "mlc-im-debug", "displayName": "mlc-im-debug"}
    return {"identities": [
        {
            "name": "mlc-im-upload",
            "account": account,
            "credentials": [{"accessKey": values["MLC_IM_S3_ACCESS_KEY"],
                             "secretKey": values["MLC_IM_S3_SECRET_KEY"]}],
            # 不授予 WriteAcp：SeaweedFS 4.51 接受 public-read，不能依赖签名方自律。
            "actions": [f"Write:{bucket}/quarantine/*", f"Read:{bucket}/sealed/*"],
        },
        {
            "name": "mlc-im-review",
            "account": account,
            "credentials": [{"accessKey": values["MLC_IM_REVIEW_S3_ACCESS_KEY"],
                             "secretKey": values["MLC_IM_REVIEW_S3_SECRET_KEY"]}],
            "actions": [f"Read:{bucket}/quarantine/*", f"Read:{bucket}/sealed/*",
                        f"Write:{bucket}/sealed/*"],
        },
    ]}


def hg_check_container():
    container = json.loads(subprocess.check_output(
        ["docker", "inspect", HG_CONTAINER], stderr=subprocess.DEVNULL))[0]
    mounts = container["HostConfig"]["Binds"] or []
    expected_bind = f"/host_mnt{HG_CONFIG}:/etc/seaweed/s3.json:ro"
    ports = container["HostConfig"]["PortBindings"]
    if (container["Config"]["Image"] != HG_IMAGE
            or expected_bind not in mounts
            or ports.get("8333/tcp") != [{"HostIp": "127.0.0.1", "HostPort": "8333"}]
            or ports.get("18082/tcp") != [{"HostIp": "127.0.0.1", "HostPort": "18082"}]):
        raise ValueError("现有 IM 存储容器镜像、私有配置挂载或端口与预期不符")
    return container["State"]["Running"]


def hg_ensure_clamav():
    name = "mlc-im-debug-clamav"
    container = json.loads(subprocess.check_output(
        ["docker", "inspect", name], stderr=subprocess.DEVNULL))[0]
    if (container["Config"]["Image"] != HG_CLAMAV_IMAGE
            or container["HostConfig"]["PortBindings"].get("3310/tcp") !=
            [{"HostIp": "127.0.0.1", "HostPort": "3310"}]):
        raise ValueError("现有 ClamAV 容器镜像或监听地址不符合本机 debug 预期")
    if not container["State"]["Running"]:
        subprocess.run(["docker", "start", name], check=True, stdout=subprocess.DEVNULL)
    for _ in range(60):
        status = subprocess.check_output(
            ["docker", "inspect", "--format", "{{.State.Health.Status}}", name],
            text=True, stderr=subprocess.DEVNULL).strip()
        if status == "healthy":
            print("[INFO] ClamAV 已就绪")
            return
        if status == "unhealthy":
            break
        time.sleep(2)
    raise ValueError("ClamAV 未就绪，拒绝启动 IM 审核器")


def hg_run():
    if os.environ.get("SERVER_ENV", "debug") != "debug":
        raise ValueError("只允许在 debug 环境恢复 IM 存储")
    expected = hg_expected()
    hg_ensure_clamav()
    running = hg_check_container()
    if not running:
        HG_DIR.mkdir(mode=0o700, parents=True, exist_ok=True)
    if HG_DIR.is_symlink() or HG_DIR.stat().st_uid != os.getuid():
        raise ValueError("IM 私有配置目录所有者不符")
    HG_DIR.chmod(0o700)
    if HG_CONFIG.exists() or HG_CONFIG.is_symlink():
        mode = HG_CONFIG.stat().st_mode
        if not stat.S_ISREG(mode) or stat.S_IMODE(mode) != 0o600:
            raise ValueError("IM 私有配置文件权限不符合 0600")
        if json.loads(HG_CONFIG.read_text()) != expected:
            raise ValueError("现有 IM 私有配置与最小权限及当前凭据不符，拒绝覆盖")
    else:
        if running:
            raise ValueError("存储容器已运行但私有配置缺失，拒绝复用")
        fd = os.open(HG_CONFIG, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as file:
            json.dump(expected, file)
    if running:
        print("[INFO] IM 存储容器已运行且私有权限配置匹配；不覆盖或重启")
        return
    subprocess.run(["docker", "start", HG_CONTAINER], check=True, stdout=subprocess.DEVNULL)
    print("[INFO] IM 存储容器已启动（仅本机；权限仍须实测）")


if __name__ == "__main__":
    try:
        hg_run()
    except (ValueError, OSError, subprocess.SubprocessError, json.JSONDecodeError) as error:
        print(f"[ERROR] 恢复 debug IM 存储失败: {error}", file=sys.stderr)
        sys.exit(1)
