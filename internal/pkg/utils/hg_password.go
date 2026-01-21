/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 11:17:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 14:19:18
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_password.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UtilsPackage

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashPassword(password string, salt string) string {
	//TODO: 真实项目用 bcrypt / argon2
	// 这里使用简单的拼接和哈希作为示例，实际应用中应使用更安全的哈希算法
	hashed := sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(hashed[:])	
}