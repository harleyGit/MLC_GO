/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 15:04:55
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-15 17:18:43
 * @FilePath: /MLC_GO/TestNotes/performance_practice/performance_analysis/performance_analysis_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package performance_practice

import (
	performance_analysis_package "MLC_GO/TestNotes/performance_practice/performance_analysis"
	"log"
	"net/http"
	_ "net/http/pprof"
)

// 性能剖析 PProf测试例子
func Performance_analysis_practice_v1_test() {
	go func() {
		for {
			//https://github.com/harleyGit/StudyNotes/tree/master
			log.Println(performance_analysis_package.Add("https://github.com/harleyGit/StudyNotes/tree/master"))
		}
	}()

	http.ListenAndServe("0.0.0.0:6060", nil)
}
