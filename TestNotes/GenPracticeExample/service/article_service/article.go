/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-12 17:40:31
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-12 17:52:40
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/service/article_service/article.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AEac
 */
package article_service

import "MLC_GO/TestNotes/GenPracticeExample/models"

type Article struct {
	ID            int
	TagID         int
	Title         string
	Desc          string
	Content       string
	CoverImageUrl string
	State         int
	CreatedBy     string
	ModifiedBy    string

	PageNum  int
	PageSize int
}

func (a *Article) Add() error {
	article := map[string] interface{} {
		"tag_id":          a.TagID,
		"title":           a.Title,
		"desc":            a.Desc,
		"content":         a.Content,
		"created_by":      a.CreatedBy,
		"cover_image_url": a.CoverImageUrl,
		"state":           a.State,
	}

	if err := models.AddArticle(article); err != nil {
		return err
	}

	return nil
}

func (a *Article) ExistByID() (bool, error) {
	return models.ExistArticleByID(a.ID)
}

func (a *Article) Edit() error {
	return models.EditArticle(a.ID, map[string]interface{}{
		"tag_id":          a.TagID,
		"title":           a.Title,
		"desc":            a.Desc,
		"content":         a.Content,
		"cover_image_url": a.CoverImageUrl,
		"state":           a.State,
		"modified_by":     a.ModifiedBy,
	})
}