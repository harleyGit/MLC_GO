/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 11:43:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-20 21:26:03
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_random_num.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UtilsPackage

import (
	"crypto/rand"
	"encoding/hex"
)


func GenerateRandomNum(n int) string {
	bytes := make([]byte, n) // byte切片,长度为n
	rand.Read(bytes)
	for i := range bytes {//遍历切片
		bytes[i] = '0' + bytes[i]%10 //取余然后转换为数字字符
	}
	return string(bytes)//转换为字符串返回
}

func GenerateRandomSalt() string {
	bytes := make([]byte, 16) // byte切片,长度为n
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

