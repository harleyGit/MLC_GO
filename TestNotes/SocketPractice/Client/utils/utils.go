/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 19:40:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-21 20:00:35
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Client/utils/utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package utils

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/pkg/logHG"
	"encoding/binary"
	"encoding/json"
	"net"
)

// 这里将这些方法关联到结构体中
type Transfer struct {
	//分析它应该有哪些字段
	Conn net.Conn
	Buf  [8096]byte //传输时，使用缓冲
}

func (this *Transfer) ReadPkg() (mes message.Message, err error) {

	logHG.DebugInfo("读取客户端发送的数据....")
	// conn.Read 在conn没有被关闭的情况下，才会阻塞
	// 如果客户端关闭了 conn 则就不会阻塞
	_, err = this.Conn.Read(this.Buf[:4])
	if err != nil {
		return
	}
	// 根据buf[:4]转成一个uint32类型
	var pkgLen uint32
	pkgLen = binary.BigEndian.Uint32(this.Buf[0:4])
	//根据pkgLen读取消息内容
	n, err := this.Conn.Read(this.Buf[:pkgLen])
	if n != int(pkgLen) || err != nil {
		return
	}
	// 把pkgLen反序列化成 -> message.Message
	err = json.Unmarshal(this.Buf[:pkgLen], &mes)
	if err != nil {
		logHG.ErrInfo("json.Unmarsha err: ", err)
		return
	}
	return
}

func (this *Transfer) WritePkg(data []byte) (err error) {

	// 先发送一个长度给对方
	var pkgLen uint32
	pkgLen = uint32(len(data))
	binary.BigEndian.PutUint32(this.Buf[0:4], pkgLen)
	// 发送长度
	n, err := this.Conn.Write(this.Buf[:4])
	if n != 4 || err != nil {
		logHG.ErrInfo("conn.Write(bytes) fail: ", err)
		return
	}
	// 发送data本身
	n, err = this.Conn.Write(data)
	if n != int(pkgLen) || err != nil {
		logHG.ErrInfo("conn.Write（bytes）fail: ", err)
		return
	}
	return
}
