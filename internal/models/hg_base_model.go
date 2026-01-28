/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-25 17:04:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:05:12
 * @FilePath: /MLC_GO/internal/models/hg_base_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGBaseModelPackage

import (
	"fmt"
	"reflect"
	"strings"
)



// TODO: 将其写入到一个基类model中，最好这个类是一个协议，其他继承这个类。然后共用这个反射映射
// 使用反射（通用但性能稍差）
func ModelToMap(dto interface{}) map[string]string {
	result := make(map[string]string)
	v := reflect.ValueOf(dto)

	// 如果是指针，获取指向的值
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// 获取json标签
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 处理omitempty
		jsonTag = strings.Split(jsonTag, ",")[0]

		// 根据字段类型处理
		switch field.Type.Kind() {
		case reflect.String:
			result[jsonTag] = value.String()

		case reflect.Ptr:
			if !value.IsNil() {
				elem := value.Elem()
				if elem.Kind() == reflect.String {
					result[jsonTag] = elem.String()
				}
			}

		case reflect.Int, reflect.Int64:
			result[jsonTag] = fmt.Sprintf("%d", value.Int())
		}
	}

	return result
}

// 使用
// userDTO := HGCreateUserDTO{
//     Username: &username,
//     Password: "123456",
//     // ...
// }
// result := ModelToMap(userDTO)
