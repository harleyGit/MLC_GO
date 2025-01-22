/// 创建文件并写入

package main

import (
	"bufio"
	"fmt"
	"os"
)


func main() {
	testCreateFileAndWrite()
}

// 创建文件并写入
func testCreateFileAndWrite() {
	// 打开文件
	// 0666 表示当前文件没有特殊权限， 任何用户都可以对其执行写入、读取操作
	file, err := os.OpenFile("poetry.txt", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("打开文件失败", err)
	}

	defer file.Close()

	// 使用带缓存的Writer
	write := bufio.NewWriter(file)
	write.WriteString("青海白云暗雪山，")
	write.WriteString("孤城遥望玉门关。")
	write.WriteString("黄沙百战穿金甲，")
	write.WriteString("不破楼兰终不还。")

	// 将缓存的数据写入文件
	write.Flush()

	fmt.Println("诗句以ing写入文件， 请查看")
}