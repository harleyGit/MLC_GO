/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 01:05:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 10:58:04
 * @FilePath: /MLC_GO/internal/logger/hg_log.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGLoggerPackage

import (
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"encoding/json"
	"time"
)

type HGLog struct {
	Time string `json:"time"`
	Msg  any    `json:"msg"`
	TID  string `json:"tid"`
}

func LogInfo(ctx context.Context, msg any) {
	if ctx == nil {
		return
	}
	l := HGLog{
		Time: time.Now().Format(time.RFC3339),
		Msg:  msg,
		TID:  UtilsPackage.From(ctx),
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		logHG.ErrFInfo("json marshal error: %v", err)
		return
	}
	// 控制台
	logHG.DebugFInfo("【日志】：%s", string(b))

	// 文件（自动切割
	GetLogWriter().WriteLog(append(b, '\n'))
}
