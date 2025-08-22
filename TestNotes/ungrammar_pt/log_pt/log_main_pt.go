/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-22 11:00:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-22 11:43:27
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/log_pt/log_main_pt.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package logpt

import logpt00 "MLC_GO/TestNotes/ungrammar_pt/log_pt/log_pt00"

func LogMainPT() {

	//校验 ID：哨兵错误 + 可判定
	logpt00.Log_fmt_Errorf_PT00()

	//加锁失败：保留根因（用 %w 包装）
	logpt00.Log_fmt_Errorf_PT01()

	//errors.As：提取具体错误类型=
	logpt00.Log_errors_As_PT00()
}
