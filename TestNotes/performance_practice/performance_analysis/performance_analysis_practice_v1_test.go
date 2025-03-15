/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 17:18:15
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-15 17:18:24
 * @FilePath: /MLC_GO/TestNotes/performance_practice/performance_analysis/performance_analysis_practice_v1_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package performance_analysis

import "testing"

const url = "https://github.com/harleyGit/StudyNotes/tree/master"

func TestAdd(t *testing.T) {
	s := Add(url)
	if s == "" {
		t.Errorf("Test.Add error!")
	}
}

func BenchmarkAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Add(url)
	}
}