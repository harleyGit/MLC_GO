/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 17:30:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 17:30:10
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_config/gorm_practice_global_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_config

import "gorm.io/gorm"

var (
	GormDB *gorm.DB
)