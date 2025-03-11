/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-11 19:30:52
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-11 19:34:27
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/util/md5.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package util

import (
	"crypto/md5"
	"encoding/hex"
)

func EncodeMD5(value string) string {
	// md5.New() 来自 crypto/md5 包，它返回一个 hash.Hash 对象，用于计算 MD5 哈希值。
	m := md5.New()
	// 将数据添加到哈希计算中。
	// []byte(value)：将字符串 value 转换为 []byte，因为 Write() 需要字节切片作为输入。
	m.Write([]byte(value))

	// m.Sum(nil) 计算当前数据的 MD5 哈希值，并返回 一个 16 字节的 []byte 切片（128 位）。
	// hex.EncodeToString(m.Sum(nil))：将 二进制的 MD5 值 转换为 十六进制字符串。
	return hex.EncodeToString(m.Sum(nil))
}