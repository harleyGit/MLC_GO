/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 21:07:44
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 21:39:36
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/read_config_file_practice/read_config_json_practice.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package read_file_practice

import "MLC_GO/TestNotes/unfamiliar_grammar_practice/read_file_practice/read_json_file_practice_v"


func ReadFilePracticeMain() {
	readJSONFile := read_json_file_practice_v.ReadJSONFilePractice{}
	readJSONFile.ExecutePracticeNone()
	readJSONFile.ReadJSONFilePractice_v1()
}