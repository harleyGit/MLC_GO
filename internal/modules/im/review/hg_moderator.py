"""本机 NudeNet 裸露检测；只覆盖显式裸露类别，不等同于全类别内容合规。"""

import os
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
    # 本地 CPU 预算只覆盖短视频；超长视频明确拒绝，不以稀疏抽帧宣称全片合规。
    if not 0 < int(duration) <= 12000:
        return 3
    with tempfile.TemporaryDirectory(prefix="hg-im-frames-") as directory:
        subprocess.run(
            [ffmpeg, "-nostdin", "-v", "error", "-i", path, "-an", "-vf", "fps=2,scale=640:640:force_original_aspect_ratio=decrease", "-frames:v", "24", "-q:v", "3", os.path.join(directory, "%03d.jpg")],
            check=True,
            timeout=6,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        frames = sorted(os.path.join(directory, name) for name in os.listdir(directory))
        if not frames:
            return 2
        for frame in frames:
            if any(item["class"] in HG_EXPOSED and item["score"] >= 0.4 for item in detector.detect(frame)):
                return 3
    return 0


if __name__ == "__main__":
    try:
        sys.exit(hg_main())
    except Exception:
        # 不输出用户文件路径、模型细节或原始内容。
        sys.exit(2)
