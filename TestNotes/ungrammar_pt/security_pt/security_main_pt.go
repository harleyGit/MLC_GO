/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-24 08:09:13
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-09-16 10:25:48
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/security_pt/security_main_pt.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package securitypt

import (
	HGSafeV0Pkg "MLC_GO/pkg/hg_safe"
	securityv00 "MLC_GO/pkg/security/security_v00"
	securityV01 "MLC_GO/pkg/security/security_v01"
)


func SecurityPTMain() {
	// 椭圆曲线-共享密钥
	HGSafeV0Pkg.SafeMainPT()
}

/* 升级加密 */
func SecurityV01_mtls_tool() {
	securityV01.SecurityV01Main()
}


/* 生成securityv00证书 */
func SecurityV00_generate_certs() {
	securityv00.Security_v00_Gen_Certs_Main()
}
/*启动服务端 */
func SecurityV00_activate_Server() {
	securityv00.Security_v00_Server_Main()
}
/* 启动客户端 */
func SecurityV00_activate_Client() {
	securityv00.Security_v00_Client_Main()
}