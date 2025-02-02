/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-26 14:12:26
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-27 14:21:27
 * @FilePath: /MLC_GO/TestNotes/TestPrepareTest/TestNormalTest/test_normal_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package testnormaltest

import "testing"

func TestHello(t *testing.T){
	result := HelloTest()
	want := "Hello World"
	if result == want {
		t.Logf("🍭 Hello() = %v, want %v", result, want)
	}else {
		t.Errorf("🍭 Hello() = %v, want %v", result, want)
	}
}
