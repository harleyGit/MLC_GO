/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-06 21:24:47
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 21:59:01
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/vendor/github.com/huanggang/gorm/utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm

import "reflect"


// 判断该字段的值是否为空
// 用于 判断一个 reflect.Value 是否为空。
// 在 GORM（或其他反射相关的场景）中，这个函数通常用于 检查结构体字段是否为默认值，然后决定是否赋值。
func isBlank(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return value.IsNil()
	}

	// 若为空则 field.Set 用于给该字段设置值，参数为 interface{}
	/*
		对于无法匹配的类型，使用 reflect.DeepEqual() 进行通用判断
		reflect.Zero(value.Type()) 返回该类型的“零值”，然后与 value.Interface() 进行比较。
		如果相等，则 value 是默认值（空值）。
	*/ 

	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}