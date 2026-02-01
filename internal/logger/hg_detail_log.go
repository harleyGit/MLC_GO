/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 01:00:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 01:21:37
 * @FilePath: /MLC_GO/internal/pkg/logger/hg_detail_log.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGLoggerPackage

const (
	LoginLogBeforeDesc string = "用户登录,获取SQL数据前"
	LoginLogAfterDesc  string = "用户登录,获取SQL数据后"
)

/**打印范例
HGLoggerPackage.LogInfo(ctx, map[string]any{
		"Tag": HGLoggerPackage.LoginLogBeforeDesc,
		"account": account,
	})
*/
