/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-18 10:28:03
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-18 10:34:49
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/utils/utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package utils

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"net"
)

// 这里将这些方法关联到结构体中
type Transfer struct {
	Conn net.Conn
	Buf  [8096]byte //传输时，使用缓冲
}

func (this *Transfer) ReadPkg() (mes message.Message, err error) {

	return
}

func (this *Transfer) WritePkg(data []byte) (err error) {

	return
}
