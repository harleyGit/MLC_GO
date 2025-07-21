/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 15:58:03
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-21 18:14:53
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Client/ClientMain/utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE()
 */
package clientmain

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/pkg/logHG"
	"encoding/binary"
	"encoding/json"
	"net"
)
func readPkg(conn net.Conn) (mes message.Message, err error) {
	buf := make([]byte, 8096)
	logHG.DebugInfo("读取客户端发送的数据......")
	// 在conn没有被关闭的情况下， 才会阻塞
	//如果客户端关闭 conn， 就不会阻塞
	_,err = conn.Read(buf[:4])
	if  err != nil {
		return
	}
	//根据buf[:4]转成一个uint32类型
	var pkgLen uint32
	pkgLen = binary.BigEndian.Uint32(buf[0:4])
	// 根据 pkgLen 读取消息内容
	n, err := conn.Read(buf[:pkgLen])
	if n != int(pkgLen) || err != nil {
		return
	}
	//把pkgLen 反序列化成-> message.Message
	err = json.Unmarshal(buf[: pkgLen], &mes)
	if err != nil {
		logHG.ErrInfo("json.Unmarshal err =", err)
		return
	}

	return
} 

func WritePkg(conn net.Conn, data []byte) (err error) {
	//先发送一个长度给对方
	var pkgLen uint32
	pkgLen = uint32(len(data))
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[0:4], pkgLen)
	// 发送长度
	n, err :=  conn.Write(buf[:4])
	if n != 4 || err != nil {
		logHG.ErrInfo("conn.Write(byres) fail: ", err)
		return
	}

	//发送data长度本身
	n, err = conn.Write(data)
	if n != int(pkgLen) || err != nil {
		logHG.ErrInfo("conn.Write(bytes) fail: ", err)
		return
	}

	return
}