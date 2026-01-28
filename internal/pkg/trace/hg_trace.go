/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-28 10:16:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 10:39:22

* @FilePath: /MLC_GO/internal/pkg/trace/hg_trace.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* OpenTelemetry 设计是啥？使用它有啥好处
* 这就是“自研究OTel适配层”，将来可以将这里替换成官方 【go.opentelemetry.io/tel】
* 
*/
package HGTracePackage

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"fmt"
	"time"
)

type HGTraceContext struct {
	TraceD string
	SpanID string
	Name   string
	Start  time.Time
}

type hgTraceKey struct{}

var traceKey = hgTraceKey{}

// 对外暴露安全的读取方法
func GetTraceKey(ctx context.Context) string {
	traceKey, _ := ctx.Value(traceKey).(string)
	return traceKey
}

func NewTrace(ctx context.Context, name string, tid string) context.Context {

	tc := HGTraceContext{
		TraceD: tid,
		SpanID: genSpanID(),
		Name:   name,
		Start:  time.Now(),
	}
	return context.WithValue(ctx, traceKey, tc)
}

func Get(ctx context.Context) *HGTraceContext {

	if v := ctx.Value(traceKey); v != nil {
		return v.(*HGTraceContext)
	}
	return nil
}

func End(tc *HGTraceContext) {
	cost := time.Since(tc.Start).Milliseconds()
	logHG.DebugFInfo(
		`{"otel":"span", "trace_id":"%s","span_id":"%s", "name":"%s", "cost_ms":%d   }`+"\n",
		tc.TraceD,
		tc.SpanID,
		tc.Name,
		cost,
	)
}

func genSpanID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
