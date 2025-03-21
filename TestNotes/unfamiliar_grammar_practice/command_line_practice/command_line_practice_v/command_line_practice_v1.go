/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-21 16:07:13
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-21 16:25:24
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/command_line_practice/command_line_practice_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package command_line_practice_v

import (
	"MLC_GO/pkg/hg_setup_config"
	"MLC_GO/pkg/hglog"
)

type CommandLinePracticeV1 struct {}

// 协议
func (commandLinePracticeV1 *CommandLinePracticeV1) ExecutePracticeNone() {
	hglog.DebugInfo("V1命令加载项目配置文件CommandLinePracticeV1  ExecutePracticeNone")
}

/*
如何命令加载项目配置文件呢?

第一种: 编译文件读取工程配置文件
	1.确保在项目根目录: cd ~/MLC_GO
	2.编译你的 Go 项目（如果尚未编译）:	go build -o app
	3.3️⃣ 运行 app 并指定配置文件: ./app -c /etc/myconfig.yaml

第二种: 直接通过Go代码运行
	1. go run main.go -c config.yaml
	或者: go run main.go -c ./conf/mlc_app.yml 
*/
/// 加载测试的yaml文件
func (commandLinePracticeV1 *CommandLinePracticeV1) CommandLinePracticeV1_v1() {
	configer := hg_setup_config.HGSetupConfig{}
	configer.HGSetupConfig()
}