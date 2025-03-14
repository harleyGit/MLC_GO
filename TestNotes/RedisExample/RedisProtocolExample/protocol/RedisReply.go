/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 19:54:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 09:49:42
 * @FilePath: /MLC_GO/TestNotes/RedisExample/RedisProtocolExample/protocol/RedisReply.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package protocol

import (
	"strconv"
	"strings"
)

const (
	StatusReply = 		'+'
	ErrorReply = 		'-'
	IntegerReply = 		':'
	BulkReply = 		'$'
	MultiBulkReply = 	'*'

	OkReply = 			"OK"
	PongReply = 		"PONG"
)

// 根据回复数据的首字节判断回复的类型，然后调用相应的处理函数将原始数据解析成最终的数据格式。
func GetReply(reply []byte) (interface{}, error) {
	// 取出回复的第一个字节,根据 Redis 协议，不同回复类型的标识符通常为：
	// +：简单状态回复
	// -：错误回复
	// :：整数回复
	// $：Bulk 回复
	// *：Multi-Bulk 回复
	replyType := reply[0]

	switch replyType {
		case StatusReply:
			// reply[1:]，即去除掉首字节后剩余的部分。这表示根据当前逻辑，StatusReply 被当作一个 Multi-Bulk 回复来处理；
			return doStatusReply(reply[1:])
		case ErrorReply:
			return doErrorReply(reply[1:])
		case IntegerReply:
			return doIntegerReply(reply[1:])
		case BulkReply:
			return doBulkReply(reply[1:])
		case MultiBulkReply:
			return doMultiBulkReply(reply[1:])
		default:
			return nil, nil
	}
}

// 解析状态回复。状态回复一般为简单的文本信息，例如 “OK” 或 “PONG”。
func doStatusReply(reply []byte) (string, error) {
	// 如果长度为 3 且字符序列为 “OK”（注意 reply[0] 可能是状态标识符，如 +），则返回预定义的常量 OkReply。
	if len(reply) == 3 && reply[1] == 'O' && reply[2] == 'K' {
		return OkReply, nil
	}

	// 如果长度为 5 且字符序列为 “PONG”，则返回 PongReply。
	if len(reply) == 5 && reply[1] == 'P' && reply[2] == 'O' && reply[3] == 'N' && reply[4] == 'G' {
		return PongReply, nil
	}

	// 如果以上两个条件都不满足，则直接将 reply 转为字符串返回。
	return string(reply), nil
}

// 对错误回复进行处理，直接将错误信息（原始字节数据）转换为字符串返回。
func doErrorReply(reply []byte) (string, error) {
	return string(reply), nil
}

// 解析整数回复。Redis 整数回复一般以 : 开头，后面紧跟一个数字和回车换行。
// 返回转换后的整数或转换错误
func doIntegerReply(reply []byte) (int, error) {
	// 找到回复中第一个回车字符的位置，此位置之前的内容就是整数的字符串表示
	pos := getFlagPos('\r', reply)
	// 使用 strconv.Atoi 将该子串转换为整数。
	result, err := strconv.Atoi(string(reply[:pos]))
	if err != nil {
		return 0, err
	}

	return result, nil
}

// 解析 Bulk 回复。Bulk 回复以 $ 开头，紧跟着一个数字表示数据长度，再换行，最后是实际数据。
func doBulkReply(reply []byte) (interface{}, error) {
	// 定位到第一个回车字符的位置，该位置前的子串（可能以 $ 开头）表示数据长度信息。
	// 如果第一个字符是 $，则将转换起始位置 pstart 设为 1，跳过 $ 符号。
	pos := getFlagPos('\r', reply)
	pstart := 0
	if reply[:pos][0] == '$' {
		pstart = 1
	}

	// 如果第一个字符是 $，则将转换起始位置 pstart 设为 1，跳过 $ 符号。
	// 如果 vlen 等于 -1，表示 Redis 协议中的 NULL 回复，返回 nil。
	vlen, err := strconv.Atoi(string(reply[pstart:pos]))
	if err != nil {
		return nil, err
	}
	if vlen == -1 {
		return nil, nil
	}

	// 数据实际开始的位置为 pos + 2（跳过回车换行符 \r\n），结束位置为 start + vlen
	start := pos + 2
	end := start + vlen
	// 返回从 start 到 end 的子串（转换为字符串）。
	return string(reply[start:end]), nil
}

// 解析 Multi-Bulk 回复，也就是由多个 Bulk 回复组成的数组形式。Multi-Bulk 回复以 * 开头，接着是 Bulk 回复的个数，然后依次列出各个 Bulk 回复。
func doMultiBulkReply(reply []byte) (interface{}, error) {
	// 使用 strings.Split(string(reply), "\r\n") 将整个回复按照回车换行符拆分成字符串数组。
	replyStrs := strings.Split(string(reply), "\r\n")
	replylen := len(replyStrs)
	// 接着对数组进行切片 replyStrs = replyStrs[1 : replylen-1]，去掉第一个元素（通常为 Bulk 回复的数量信息）和最后一个空字符串。
	replyStrs = replyStrs[1 : replylen-1]

	// 初始化一个空的切片 r 用于保存解析后的各个 Bulk 回复。
	r := []interface{}{}
	for i := 0; i < replylen-1; i++ {
		if i%2 == 1 { // 认为每两个元素为一组（例如：第一行是 Bulk 回复的长度信息，第二行是实际数据）。
			rv := strings.Join([]string{ // 通过 strings.Join 将相邻两行拼接成一个完整的 Bulk 回复字符串，再加上结尾的 \r\n。
				replyStrs[i-1],
				replyStrs[i],
			}, "\r\n") + "\r\n"

			// 对拼接后的字符串进行解析，得到实际数据 value。
			value, err := doBulkReply([]byte(rv))
			if err != nil {
				return nil, err
			}

			r = append(r, value)
		}
	}

	return r, nil
}

// 在给定的字节切片 reply 中查找指定字符（例如回车 '\r'）的第一个出现位置。
func getFlagPos(flag byte, reply []byte) int {
	pos := 0
	for _, v := range reply {
		// 遍历 reply 中的每个字节，并计数位置。
		if v == flag {
			break
		}
		pos++
	}

	return pos
}