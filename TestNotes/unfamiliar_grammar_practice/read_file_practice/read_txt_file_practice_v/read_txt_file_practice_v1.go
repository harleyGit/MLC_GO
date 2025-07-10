/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 21:49:11
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-21 09:49:26
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/read_file_practice/read_txt_file_practice_v/read_txt_file_practice_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 *
 * title: 读取key=value类型的配置文件
 */
package read_txt_file_practice_v

import (
	"MLC_GO/pkg/logHG"
	"bufio"
	"io"
	"os"
	"strings"
)

type ReadTXTFilePractice struct{}

// 协议
func (readTextPractice *ReadTXTFilePractice) ExecutePracticeNone() {
	logHG.DebugInfo("协议 读取Text文件配置 ReadJSONFilePractice ExecutePracticeNone")
}

func (readJSONPractice *ReadTXTFilePractice) ReadTextFilePractice_v1() {
	config := InitConfig("./conf/mlc_app.txt")
	ip := config["ip"]
	port := config["port"]

	logHG.DebugInfo("text文件读取内容: ip=", string(ip), " port=", string(port))
}

// 读取key=value类型的配置文件
func InitConfig(path string) map[string]string {
	///  1.创建一个 map
	config := make(map[string]string)

	/// 2.打开文件
	// 尝试打开配置文件
	f, err := os.Open(path)
	// 确保函数结束时关闭文件，防止资源泄露
	defer f.Close()
	if err != nil {
		// 如果 os.Open 失败（比如文件不存在），直接 panic(err) 终止程序
		panic(err)
	}

	/// 3. 逐行读取文件
	// 创建 bufio.Reader 逐行读取文件内容
	r := bufio.NewReader(f)
	for {
		// r.ReadLine()：读取一行数据（返回 b []byte）
		b, _, err := r.ReadLine()
		/// 4.处理读取的行
		if err != nil {
			// 逐行读取，直到 EOF
			// io.EOF：表示读取结束，退出循环
			if err == io.EOF {
				break
			}
			// panic(err)：如果遇到其他错误，直接 panic 终止程序
			panic(err)
		}
		/// 5.处理每一行字符串
		// 去掉 首尾空格，防止解析错误
		s := strings.TrimSpace(string(b))
		// 查找 = 的位置，如果找不到（即 index == -1），跳过本行
		index := strings.Index(s, "=")
		if index < 0 {
			continue
		}

		/// 6.提取 key=value
		// 获取 key：
		// 		s[:index] 提取 等号左边的部分（即 key）。
		// 		TrimSpace 去除空格，避免 " key = value " 这种情况出错。
		// 		if len(key) == 0 过滤掉 空的 key，如 =value（没有 key）
		key := strings.TrimSpace(s[:index])
		if len(key) == 0 {
			continue
		}
		// 获取 value：
		// 		s[index+1:] 提取 等号右边的部分（即 value）。
		// 		TrimSpace 去除空格。
		// 		if len(value) == 0 过滤掉 空的 value，如 key=（没有 value）。
		value := strings.TrimSpace(s[index+1:])
		if len(value) == 0 {
			continue
		}
		config[key] = value
	}
	return config
}
