/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 17:34:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 17:34:44
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/export/excel.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AEa
 */
package export

import "MLC_GO/TestNotes/GenPracticeExample/pkg/setting"

const EXT = ".xlsx"

func GetExcelFullUrl(name string) string {
	return setting.AppSetting.PrefixUrl + "/" + GetExcelPath() + name
}

func GetExcelPath() string {
	return setting.AppSetting.ExportSavePath
}

func GetExcelFullPath() string {
	return setting.AppSetting.RuntimeRootPath + GetExcelPath()
}