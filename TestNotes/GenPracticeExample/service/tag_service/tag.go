/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-12 17:11:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-12 17:12:04
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/service/tag_service/tag.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package tag_service

import "MLC_GO/TestNotes/GenPracticeExample/models"

type Tag struct {
	ID         int
	Name       string
	CreatedBy  string
	ModifiedBy string
	State      int

	PageNum  int
	PageSize int
}

func (t *Tag)ExistByID() (bool, error) {
	return models.ExistTagByID(t.ID)
}