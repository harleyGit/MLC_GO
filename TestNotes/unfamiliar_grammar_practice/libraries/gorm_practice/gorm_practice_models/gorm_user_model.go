/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 16:34:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-19 18:14:23
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_models/gorm_user_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_models

import "time"

type GormUser struct {
	Id int64 `json:"id" gorm:"primary_key"`
	Name string `json:"name"`
	Age int32 `json:"age"`
	Sex int8 `json:"sex"`
	Phone string `json:"phone`
	Birthday time.Time `gorm:"column:day_of_the_beast"` // 将列名设为 `day_of_the_beast`
	CreatedAt time.Time // 在创建时，如果该字段值为零值，则使用当前时间填充
	UpdatedAt int       // 在创建时该字段值为零值或者在更新时，使用当前时间戳秒数填充
}