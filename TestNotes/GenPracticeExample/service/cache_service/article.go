/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 16:33:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 17:14:32
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/service/cache_service/article.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// 缓存Key
package cache_service

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"strconv"
	"strings"
)

type Article struct {
	ID int
	TagID int
	State int

	PageNum int
	PageSize int
}


func (a *Article) GetArticleKey() string {
	// strconv.Itoa(a.ID)：将文章的整数 ID 转换为字符串
	return e.CACHE_ARTICLE + "_" + strconv.Itoa(a.ID)
}

// 一个定义在 Article 结构体上的方法，通过指针接收器 a *Article 可以访问当前文章对象的字段。
func (a *Article) GetArticlesKey() string {
	// 创建一个字符串切片 keys
	keys := []string{
		e.CACHE_ARTICLE,
		"LIST",
	}

	// 如果文章的 ID 大于 0，则说明该文章有有效的标识符，将 ID 转换为字符串后追加到 keys 切片中
	// 这样可以让缓存键中包含具体文章的标识，例如 "ARTICLE_LIST_123"
	if a.ID > 0 {
		keys = append(keys, strconv.Itoa(a.ID))
	}
	if a.TagID > 0 {
		keys = append(keys, strconv.Itoa(a.TagID))
	}
	if a.State >= 0 {
		keys = append(keys, strconv.Itoa(a.State))
	}
	if a.PageNum > 0 {
		keys = append(keys, strconv.Itoa(a.PageNum))
	}
	if a.PageSize > 0 {
		keys = append(keys, strconv.Itoa(a.PageSize))
	}

	return strings.Join(keys, "_")
}