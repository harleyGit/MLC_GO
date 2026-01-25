/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-20 21:25:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-25 14:28:05
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_empty_util.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UtilsPackage

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

/* 无效字符串（包含空、和nil）转化为空字符串 */
func NullStrToStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func StrPtrToNullStr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}

	return sql.NullString{
		String: *s,
		Valid:  true,
	}
}

/* 空指针字符串转化为nil字符串 */
func NullStrToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

// =================== 空值判断处理Start ===================>
// StringEmptyCheckOption 字符串空值检查选项
type StringEmptyCheckOption struct {
	// TrimSpaces 是否修剪空白字符（默认true）
	TrimSpaces bool

	// IncludeUnicodeSpaces 是否包含所有Unicode空白字符（默认false，只包含ASCII空白）
	IncludeUnicodeSpaces bool

	// TreatNilAsEmpty 对于指针，nil是否视为空（默认true）
	TreatNilAsEmpty bool

	// TrimFunc 自定义修剪函数，如果设置则覆盖 TrimSpaces
	TrimFunc func(rune) bool
}

// DefaultOption 默认选项
var DefaultOption = &StringEmptyCheckOption{
	TrimSpaces:           true,
	IncludeUnicodeSpaces: false,
	TreatNilAsEmpty:      true,
}

// IsEmpty 通用方法：检查字符串是否为空
// 参数说明：
//   - value: 可以是 string, *string, []byte, 或 interface{}
//   - options: 可选的配置，使用 nil 则使用 DefaultOption
func IsEmpty(value interface{}, options ...*StringEmptyCheckOption) bool {
	opt := getOption(options)

	switch v := value.(type) {
	case string:
		return isEmptyString(v, opt)
	case *string:
		if v == nil {
			return opt.TreatNilAsEmpty
		}
		return isEmptyString(*v, opt)
	case []byte:
		if v == nil {
			return opt.TreatNilAsEmpty
		}
		return isEmptyString(string(v), opt)
	case nil:
		return opt.TreatNilAsEmpty
	default:
		// 尝试转换为字符串
		if str, ok := v.(fmt.Stringer); ok {
			return isEmptyString(str.String(), opt)
		}
		// 对于其他类型，通过 fmt.Sprintf 转换为字符串
		str := fmt.Sprintf("%v", v)
		return isEmptyString(str, opt)
	}
}

// isEmptyString 内部方法：处理字符串的具体逻辑
func isEmptyString(s string, opt *StringEmptyCheckOption) bool {
	if opt.TrimFunc != nil {
		return len(strings.TrimFunc(s, opt.TrimFunc)) == 0
	}

	if opt.TrimSpaces {
		if opt.IncludeUnicodeSpaces {
			// 使用 unicode.IsSpace 处理所有Unicode空白字符
			for _, r := range s {
				if !unicode.IsSpace(r) {
					return false
				}
			}
			return true
		}
		// 只处理ASCII空白字符
		return len(strings.TrimSpace(s)) == 0
	}

	// 不修剪空白字符，直接判断长度
	return len(s) == 0
}

// getOption 获取选项
func getOption(options []*StringEmptyCheckOption) *StringEmptyCheckOption {
	if len(options) > 0 && options[0] != nil {
		return options[0]
	}
	return DefaultOption
}

// 预设的常用选项
var (
	// StrictOption 严格模式：不修剪任何字符
	StrictOption = &StringEmptyCheckOption{
		TrimSpaces:           false,
		IncludeUnicodeSpaces: false,
		TreatNilAsEmpty:      true,
	}

	// UnicodeOption 支持Unicode空白字符
	UnicodeOption = &StringEmptyCheckOption{
		TrimSpaces:           true,
		IncludeUnicodeSpaces: true,
		TreatNilAsEmpty:      true,
	}

	// LenientOption 宽松模式：nil不算空
	LenientOption = &StringEmptyCheckOption{
		TrimSpaces:           true,
		IncludeUnicodeSpaces: false,
		TreatNilAsEmpty:      false,
	}
)

/* 空值各种判断处理 
TODO：了解这个判空方法
func mainTest() {
	// 示例1：基本使用
	fmt.Println("基本使用：")
	fmt.Printf("空字符串: %v\n", stringutils.IsEmpty(""))                 // true
	fmt.Printf("只有空格: %v\n", stringutils.IsEmpty("   "))              // true
	fmt.Printf("正常字符串: %v\n", stringutils.IsEmpty("hello"))          // false

	// 示例2：不同选项
	fmt.Println("\n不同选项：")
	fmt.Printf("严格模式（不修剪）: %v\n",
		stringutils.IsEmpty("   ", stringutils.StrictOption))          // false

	fmt.Printf("宽松模式（nil不算空）: %v\n",
		stringutils.IsEmpty(nil, stringutils.LenientOption))           // false

	// 示例3：多种类型
	fmt.Println("\n多种类型支持：")
	var strPtr *string
	fmt.Printf("nil指针: %v\n", stringutils.IsEmpty(strPtr))           // true

	name := "张三"
	fmt.Printf("字符串指针: %v\n", stringutils.IsEmpty(&name))          // false

	emptyStr := ""
	fmt.Printf("空字符串指针: %v\n", stringutils.IsEmpty(&emptyStr))     // true

	// 示例4：字节数组
	bytes := []byte("   ")
	fmt.Printf("字节数组: %v\n", stringutils.IsEmpty(bytes))            // true

	// 示例5：自定义选项
	customOpt := &stringutils.StringEmptyCheckOption{
		TrimSpaces:           true,
		IncludeUnicodeSpaces: true,
		TreatNilAsEmpty:      true,
	}
	fmt.Printf("\n自定义选项: %v\n",
		stringutils.IsEmpty("\u200B\u200B", customOpt))                // true (零宽空格)

	// 示例6：自定义修剪函数
	noDigitOpt := &stringutils.StringEmptyCheckOption{
		TrimFunc: func(r rune) bool {
			return unicode.IsDigit(r) || unicode.IsSpace(r)
		},
	}
	fmt.Printf("\n自定义修剪（去掉数字和空格）: %v\n",
		stringutils.IsEmpty(" 123 ", noDigitOpt))                      // true
	fmt.Printf("自定义修剪（去掉数字和空格）: %v\n",
		stringutils.IsEmpty(" abc ", noDigitOpt))                      // false
}
*/
// <=================== 空值判断处理End ===================
