/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 20:02:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 20:02:09
 * @FilePath: /MLC_GO/TestNotes/RedisExample/RedisProtocolExample/protocol/RedisRequest.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package protocol

import (
	"strconv"
	"strings"
)

func GetRequest(args []string) []byte {
	req := []string{
		"*" + strconv.Itoa(len(args)),
	}

	for _, arg := range args {
		req = append(req, "$"+strconv.Itoa(len(arg)))
		req = append(req, arg)
	}

	str := strings.Join(req, "\r\n")
	return []byte(str + "\r\n")
}