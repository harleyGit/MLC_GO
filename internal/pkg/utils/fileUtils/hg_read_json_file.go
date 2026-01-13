/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 21:09:55
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 21:12:59
 * @FilePath: /MLC_GO/pkg/utils/fileUtils/hg_read_json_file.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package file_utils

type FileUtiler interface {
	// 配置文件路径
	configPath() string
}