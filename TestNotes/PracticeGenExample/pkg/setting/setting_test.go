/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 20:50:34
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-24 20:12:07
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/setting/setting.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package setting

import "testing"

func TestLoadBase(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
		{"huanggang"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LoadBase()
		})
	}
}
