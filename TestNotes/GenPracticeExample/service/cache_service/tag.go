/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 16:34:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 17:13:44
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/service/cache_service/tag.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

// 缓存Key
// 传送门
package cache_service

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"strconv"
	"strings"
)

type Tag struct {
	ID    int
	Name  string
	State int

	PageNum  int
	PageSize int
}

func (t *Tag) GetTagsKey() string {
	keys := []string{
		e.CACHE_TAG,
		"LIST",
	}

	if t.Name != "" {
		keys = append(keys, t.Name)
	}
	if t.State >= 0 {
		keys = append(keys, strconv.Itoa(t.State))
	}
	if t.PageNum > 0 {
		keys = append(keys, strconv.Itoa(t.PageNum))
	}
	if t.PageSize > 0 {
		keys = append(keys, strconv.Itoa(t.PageSize))
	}

	return strings.Join(keys, "_")
}