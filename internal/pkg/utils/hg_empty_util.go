/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-20 21:25:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 16:08:06
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_empty_util.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UtilsPackage

import "database/sql"

/* 无效字符串（包含空、和nil）转化为空字符串 */
func NullStrToStr(ns sql.NullString) string {
	if ns.Valid {
		return  ns.String
	}
	return  ""
}

func StrPtrToNullStr(s *string) sql.NullString {
	if s == nil {
		return  sql.NullString{Valid: false}
	}

	return sql.NullString{
		String:  *s,
		Valid: true,
	}
}

/* 空指针字符串转化为nil字符串 */
func NullStrToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return  nil
	}
	v := ns.String
	return  &v
}