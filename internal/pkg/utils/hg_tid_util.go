/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-28 20:16:41
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-29 17:42:52
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_tid_util.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

package UtilsPackage

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

// 包级单例
type HGTidContextKeyType struct{}

// 这是一个包级变量（在函数外声明的）
// 它只会在程序启动时初始化一次
// 之后每次使用都是引用同一个已初始化的实例
var tidContextKey = HGTidContextKeyType{}

/* 随机生成 tid 【用于追踪请求】 工具函数 */
func GenerateTID() string {
	return fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
}
func generateTIDV1() string {
	return fmt.Sprintf(
		"%d-%06d",
		time.Now().UnixNano(),
		rand.Intn(1000000),
	)
}

func generateTIDV2() string {

	b := make([]byte, 16)
	crand.Read(b)
	return hex.EncodeToString(b)
}

func CreateTIDV1() string {
	return generateTIDV1()
}

func CreateTIDV2() string {
	return generateTIDV2()
}

func GetTidKey() HGTidContextKeyType {
	return tidContextKey
}

// 对外暴露安全的读取方法
func GetTID(ctx context.Context) string {
	tid, _ := ctx.Value(tidContextKey).(string)
	return tid
}

func InjectTID(ctx context.Context, tid string) context.Context {
	// 放到 context 里供 handler 使用，不要字符串类型 "tid" 做key，应该用 tidContextKey
	// 原因是 字符串容易被其他的覆盖掉， string 无法表达“只属于这个包”；
	// type tidContextKeyType struct{} 1.只存在于这个 package；2.外部包 无法构造同类型的 key
	return context.WithValue(ctx, tidContextKey, tid)
}

func From(ctx context.Context) string {

	if v := ctx.Value(tidContextKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
