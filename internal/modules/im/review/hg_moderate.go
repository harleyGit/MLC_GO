package review

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// hgModerate 使用隔离的本机模型检测裸露；它不覆盖涉政、暴恐、版权等其他类别。
// 退出码 3 是明确违规，依赖失败或超时必须重试，不能据此返回 ready。
func (s *HGService) hgModerate(ctx context.Context, path, mime string, durationMS int64) error {
	work, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(work, s.cfg.Python, s.cfg.Moderator, path, mime, fmt.Sprint(durationMS), s.cfg.FFmpeg)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 3 {
			return hgRejected
		}
		return fmt.Errorf("本地内容检测不可用")
	}
	return nil
}
