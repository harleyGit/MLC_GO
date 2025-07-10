/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 17:20:40
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 18:57:44
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_service/gorm_practice_user_service.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_service

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_models"
	"MLC_GO/pkg/logHG"
)

// 插入一条用户数据
func AddNewUser(user gorm_practice_models.GormUser) (err error) {
	id, err := gorm_practice_models.InsertOneUser(user)
	if err != nil {
		return err
	}
	logHG.DebugInfo("gorm 新增用户Id", id)
	return nil
}

// 根据uid 查询用户
func QueryUserByUid(uid int64) gorm_practice_models.GormUser {
	return gorm_practice_models.QueryUserByUid(uid)
}
