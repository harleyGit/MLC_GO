/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-18 10:28:03
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:06:06
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/utils/utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package utils

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
	"encoding/binary"
	"encoding/json"
	"net"
)

// 这里将这些方法关联到结构体中
type Transfer struct {
	Conn net.Conn
	Buf  [8096]byte //传输时，使用缓冲
}

func (this *Transfer) ReadPkg() (mes message.Message, err error) {

	// buf := make([]byte, 8096)
	logHG.DebugInfo("读取客户端发送的数据......")
	//conn.Read 在conn没有关闭的情况下，才会阻塞
	// 如果客户关闭了 conn 则就不会阻塞
	_, err = this.Conn.Read(this.Buf[:4])
	if err != nil {
		return
	}

	// 根据buf[:4]转成一个uint32类型
	var pkgLen uint32
	pkgLen = binary.BigEndian.Uint32(this.Buf[0:4])
	// 根据 pkgLen 读取消息内容
	n, err := this.Conn.Read(this.Buf[:pkgLen])
	if n != int(pkgLen) || err != nil {
		return
	}

	// 把pkgLen 反序列化成 -> message.Message
	// 技术就是一层窗户纸 &么说！！
	err = json.Unmarshal(this.Buf[:pkgLen], &mes)
	if err != nil {
		logHG.ErrInfo("json.Unmarshal err = ", err)
		return
	}
	return
}

func (this *Transfer) WritePkg(data []byte) (err error) {

	// 先发送一个长度给对方
	var pkgLen uint32
	pkgLen = uint32(len(data))
	// var buf [4]byte
	binary.BigEndian.AppendUint32(this.Buf[0:4], pkgLen)
	// 发送长度
	n, err := this.Conn.Write(this.Buf[:4])
	if n != 4 || err != nil {
		logHG.ErrInfo("conn.Write(bytes) fail", err)
		return
	}

	// 发送data本身
	n, err = this.Conn.Write(data)
	if n != int(pkgLen) || err != nil {
		logHG.ErrInfo("conn.Write(bytes) fail", err)
		return
	}
	return
}
