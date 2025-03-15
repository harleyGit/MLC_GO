/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 15:08:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-15 15:13:59
 * @FilePath: /MLC_GO/TestNotes/performance_practice/performance_analysis/performance_analysis_practice_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package performance_analysis

var datas []string



func Add(str string) string {
	data := []byte(str)
	sData := string(data)
	datas = append(datas, sData)

	return sData
}