/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-22 11:14:55
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-22 11:51:47
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/log_pt/log_pt00/log_node_pt.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/// 测试fmt.Errorf的错误与 errors.As区别

package logpt00

import (
	"MLC_GO/pkg/logHG"
	"errors"
	"fmt"
	"os"
)

var errNodeIDRange = errors.New("node-id must be [0, 1024]")

/* 测试fmt.Errorf的错误与 errors.As区别：校验 ID：哨兵错误 + 可判定 */
func Log_fmt_Errorf_PT00() {
	if err := validateNodeID(2048); err != nil {
		// 明确知道是“范围错误”
		// 可以返回 400、提示用户、或走特定分支
		logHG.ErrFInfo("错误测试 err：", err)
	}
	// 记录日志时打印完整文本
	// log.Printf("validate failed: %v", err) // node-id must be [0,1024): got 2048
}

func validateNodeID(id int) error {
	if id < 0 || id >= 1024 {
		return fmt.Errorf("%w: got %d", errNodeIDRange, id)
	}
	return nil
}

//================分隔符====================
/* 加锁失败：保留根因（用 %w 包装） */
func Log_fmt_Errorf_PT01() {

	err := lock("./MLC_GO_REMADE.md")
	if err != nil {
		switch {
		case errors.Is(err, os.ErrExist):
			//目录/锁已存在（比如已有进程占用）
			logHG.DebugFInfo("目录/锁已存在（比如已有进程占用）",err)
		case errors.Is(err, os.ErrPermission):
			// 权限问题
			logHG.DebugInfo("权限问题")

		default:
			// 其他未知问题
			logHG.DebugInfo("其他未知问题")

		}
	}
}

func lock(path string) error {
	// 假设底层返回的是某个具体错误（例如 os.ErrExist）
	if err := doLock(path); err != nil {
		return fmt.Errorf("lock %q: %w", path, err) // 用 %w 才能被 Is/As 判断
	}
	return nil
}

func doLock(path string) error {
	// 仅示意：返回一个具体根因
	return os.ErrExist
}

// ================errors.As：提取具体错误类型====================
type ErrRemote struct {
	Code int
	Msg  string
}

func Log_errors_As_PT00() {
	if err := call(); err != nil {
		var r *ErrRemote
		if errors.As(err, &r) {
			// 拿到结构化信息 r.Code / r.Msg
			logHG.ErrFInfo("拿到结构化信息: %d %v", r.Code, r.Msg)
		}
	}
}

func (e *ErrRemote) Error() string { return fmt.Sprintf("remote: %d %s", e.Code, e.Msg) }

func call() error {
	return fmt.Errorf("rpc failed: %w", &ErrRemote{Code: 502, Msg: "bad gateWay"})
}
