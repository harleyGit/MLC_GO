/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 16:46:18
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-03 23:13:51
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/cmd/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cmd

import (
	"MLC_GO/TestNotes/PracticeGRPCExample/server"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/logging"

	"github.com/spf13/cobra"
)

/*
&cobra.Command：

	Use：Command的用法，Use是一个行用法消息
	Short：Short是help命令输出中显示的简短描述
	Run：运行:典型的实际工作功能。大多数命令只会实现这一点；另外还有PreRun、PreRunE、PostRun、PostRunE等等不同时期的运行命令，但比较少用，具体使用时再查看亦可
*/
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the gRPC hello-world server",
	Run: func(cmd *cobra.Command, args []string) {
		defer func() {
			if err := recover(); err != nil {
				logging.Info("Recover error: %v", err)
			}
		}()
		server.Serve()
	},
}

func init() {
	// 定义了一个flag，值存储在&server.ServerPort中，长命令为--port，短命令为-p，，默认值为50052，命令的描述为server port。这一种调用方式成为Local Flags
	// 注意： ./certs/server.pem路径容易错，因为main.go是在PracticeGRPCExample文件夹下，所以路径是相当于它的，不是相对路径是基于MLC_GO文件夹的
	serverCmd.Flags().StringVarP(&server.ServerPort, "port", "p", "50052", "server port")
	serverCmd.Flags().StringVarP(&server.CertPemPath, "cert-pem", "", "./certs/client_server.pem", "cert pem path")
	serverCmd.Flags().StringVarP(&server.CertKeyPath, "cert-key", "", "./certs/client_server.key", "cert key path")
	// 注意⚠️：当时困在这个地方许久，作者博客有问题，第4个参数不是 "grpc server name" 是 "dev"(或者 "localhost")
	serverCmd.Flags().StringVarP(&server.CertName, "cert-name", "", "localhost", "server's hostname")
	// AddCommand向这父命令（rootCmd）添加一个或多个命令
	rootCmd.AddCommand(serverCmd)
}
