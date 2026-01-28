/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-28 20:16:41
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:18:37
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_tid_util.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

package UtilsPackage

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

type HGTidContextKeyType struct{}

var tidContextKey = HGTidContextKeyType{}

/* 随机生成 tid 【用于追踪请求】 工具函数 */
func GenerateTID() string {
	return fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
}
func GenerateTIDV1() string {
	return fmt.Sprintf(
		"%d-%06d",
		time.Now().UnixNano(),
		rand.Intn(1000000),
	)
}

func CreateTID() string {
	return GenerateTIDV1()
}

func GetTidKey() HGTidContextKeyType {
	return tidContextKey
}

// 对外暴露安全的读取方法
func GetTID(ctx context.Context) string {
	tid, _ := ctx.Value(tidContextKey).(string)
	return tid
}
