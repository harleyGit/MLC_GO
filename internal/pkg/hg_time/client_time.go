/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-30 17:10:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-30 17:32:30
 * @FilePath: /MLC_GO/internal/pkg/hg_time/client_time.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package hg_time

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ClientTime 显式标记客户端时间的格式和时区。
// 前端必须声明 format，后端不再猜测。
type ClientTime struct {
	// Value 时间字符串。
	Value string `json:"value"`
	// Format 输入格式：rfc3339 / datetime-local / unix / date / year-month。
	Format string `json:"format"`
	// Timezone IANA 时区名，如 "Asia/Shanghai"。rfc3339 / unix / date / year-month 时可省略。
	Timezone string `json:"timezone,omitempty"`
}

// ParseClientTime 根据显式标记解析客户端时间，返回带正确时区的 time.Time。
// datetime-local 必须携带 timezone，否则返回错误——不再 fallback 到服务器本地时区。
func ParseClientTime(ct ClientTime) (time.Time, error) {
	switch ct.Format {
	case "rfc3339":
		/*
			解析标准 ISO 时间：
				2026-05-30T10:15:30Z
				2026-05-30T10:15:30+08:00

			支持时区
					Z → UTC
					+08:00 → 东八区
					解析结果

			Go 会自动转换成内部时间（UTC 存储）
		*/
		return time.Parse(time.RFC3339, ct.Value)
	case "datetime-local":
		if ct.Timezone == "" {
			return time.Time{}, errors.New("timezone required for datetime-local format")
		}
		/*
			加载一个时区对象（location），比如： loc, err := time.LoadLocation("Asia/Shanghai")
		*/
		loc, err := time.LoadLocation(ct.Timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timezone %q: %w", ct.Timezone, err)
		}
		return time.ParseInLocation("2006-01-02T15:04", ct.Value, loc)
	case "unix":
		ts, err := strconv.ParseInt(ct.Value, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid unix timestamp: %w", err)
		}
		return time.Unix(ts, 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported time format: %q", ct.Format)
	}
}

// NormalizeBirthDate 将显式标记的日期字符串格式化为 DB 标准格式 YYYY-MM-DD。
// format="date" → 直接校验并返回 YYYY-MM-DD。
// format="year-month" → 校验后补齐为当月 1 日 YYYY-MM-01。
func NormalizeBirthDate(ct ClientTime) (string, error) {
	switch ct.Format {
	case "date":
		/*
			ct.Value = "2026-05-30" 解析： t, _ := time.Parse("2006-01-02", ct.Value)
			返回的是UTC时间：2026-05-30 00:00:00 +0000 UTC


		*/
		t, err := time.Parse("2006-01-02", ct.Value)
		if err != nil {
			return "", fmt.Errorf("invalid date %q: %w", ct.Value, err)
		}
		return t.Format("2006-01-02"), nil
	case "year-month":
		t, err := time.Parse("2006-01", ct.Value)
		if err != nil {
			return "", fmt.Errorf("invalid year-month %q: %w", ct.Value, err)
		}
		/* 把 time.Time 格式化成字符串
					t := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
					fmt.Println(t.Format("2006-01"))
					输出：2026-05

		特点
			只保留 年 + 月
			日、时、分全部丢弃
		*/
		return t.Format("2006-01") + "-01", nil
	default:
		return "", fmt.Errorf("birth_date format must be \"date\" or \"year-month\", got %q", ct.Format)
	}
}
