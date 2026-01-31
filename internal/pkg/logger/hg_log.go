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
	Msg  any `json:"msg"`
	TID  string `json:"tid"`
}

// TODO： 日志文件如何切割
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
	logHG.DebugFInfo("【日志】：%s", string(b))
}

 
