//go:build production

package main

// main 只启动生产应用；练习模块由默认入口保留，不进入容器镜像的依赖图。
func main() {
	mlc_main()
}
