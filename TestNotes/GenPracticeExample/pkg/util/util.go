/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 19:20:30
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 19:20:33
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/util/util.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package util

import "MLC_GO/TestNotes/GenPracticeExample/pkg/setting"

func Setup() {
	jwtSecret = []byte(setting.AppSetting.JwtSecret)
}