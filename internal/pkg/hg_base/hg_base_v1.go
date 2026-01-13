/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 22:17:58
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 11:26:06
 * @FilePath: /MLC_GO/pkg/hg_base/hg_base_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package hg_base

type HGPracticerV1 interface {
	ExecutePracticeNone()
}

type HGModulePractice struct {}

func (modulePractice *HGModulePractice) ExecutePracticeNone() {}