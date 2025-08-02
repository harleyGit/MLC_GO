/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 21:07:44
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-02 17:38:08
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/read_config_file_practice/read_config_json_practice.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package read_file_practice

import (
	"MLC_GO/TestNotes/ungrammar_pt/read_file_practice/read_json_file_practice_v"
	"MLC_GO/TestNotes/ungrammar_pt/read_file_practice/read_txt_file_practice_v"
	"MLC_GO/TestNotes/ungrammar_pt/read_file_practice/read_yml_file_practice_v"
)


func ReadFilePracticeMain() {
	readYMLFile := read_yml_file_practice_v.ReadYMLTFilePractice{}
	readYMLFile.ExecutePracticeNone()
	// 读取yaml文件
	readYMLFile.ReadYMLFilePractice_v1()

	return

	readTxtFile := read_txt_file_practice_v.ReadTXTFilePractice{}
	readTxtFile.ExecutePracticeNone()
	//读取txt文件
	readTxtFile.ReadTextFilePractice_v1()

	return
	readJSONFile := read_json_file_practice_v.ReadJSONFilePractice{}
	readJSONFile.ExecutePracticeNone()
	// json文件读取
	readJSONFile.ReadJSONFilePractice_v1()
}