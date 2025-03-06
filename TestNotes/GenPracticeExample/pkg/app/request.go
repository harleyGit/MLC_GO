/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-04 21:01:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 20:01:28
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/app/request.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package app

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"

	"github.com/astaxie/beego/validation"
)

// MarkErrors logs error logs
 func MarkErrors(errors []*validation.Error) {
	 for _, err := range errors {
		 logging.DebugInfo(err.Key, err.Message)
	 }
 
	 return
 }