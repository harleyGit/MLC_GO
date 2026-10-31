"""本机 NudeNet 裸露检测；只覆盖显式裸露类别，不等同于全类别内容合规。"""

import os
import math
import subprocess
import sys
import tempfile

from nudenet import NudeDetector


HG_EXPOSED = {
    "FEMALE_GENITALIA_EXPOSED",
    "MALE_GENITALIA_EXPOSED",
    "ANUS_EXPOSED",
    "FEMALE_BREAST_EXPOSED",
    "BUTTOCKS_EXPOSED",
}


def hg_main():
    # 每个请求独立加载本地固定模型，故障不回退到仅看 MIME。
    detector = NudeDetector(providers=["CPUExecutionProvider"])
    if sys.argv[1:] == ["--check"]:
        return 0
    if len(sys.argv) != 5:
        return 2
    path, mime, duration, ffmpeg = sys.argv[1:]
    if mime.startswith("image/"):
        frames = [path]
        for frame in frames:
            if any(item["class"] in HG_EXPOSED and item["score"] >= 0.4 for item in detector.detect(frame)):
                return 3
        return 0
    if not mime.startswith("video/"):
        return 2
    # 固定上限覆盖整个时间轴；仅检测抽样点，点间内容仍可能漏检。
    duration_ms = int(duration)
    if duration_ms <= 0:
        return 2
    count = min(16, max(1, math.ceil(duration_ms / 500)))
    with tempfile.TemporaryDirectory(prefix="hg-im-frames-") as directory:
        for index in range(count):
            frame = os.path.join(directory, f"{index:02d}.jpg")
            # 输入前 seek 避免把整部视频解码到尾部；每个区间取中点，末尾留出一帧余量。
            position_ms = min((2 * index + 1) * duration_ms // (2 * count), max(0, duration_ms - 500))
            subprocess.run(
                [ffmpeg, "-nostdin", "-v", "error", "-ss", f"{position_ms / 1000:.3f}", "-i", path,
                 "-an", "-sn", "-dn", "-vf", "scale=640:640:force_original_aspect_ratio=decrease",
                 "-frames:v", "1", "-q:v", "3", frame],
                check=True,
                timeout=5,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            if not os.path.isfile(frame) or os.path.getsize(frame) == 0:
                return 2
            if any(item["class"] in HG_EXPOSED and item["score"] >= 0.4 for item in detector.detect(frame)):
                return 3
    return 0


if __name__ == "__main__":
    try:
        sys.exit(hg_main())
    except Exception:
        # 不输出用户文件路径、模型细节或原始内容。
        sys.exit(2)
