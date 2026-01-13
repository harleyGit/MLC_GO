/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-26 17:14:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:14:18
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice/nsq_practice_v1/nsq_relect_PT.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package nsq_practice_v1

import (
	"MLC_GO/internal/pkg/logHG"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
)

type NSQDOptions struct {
	Version  bool   `flag:"version" desc:"显示版本号"`
	Port     int    `flag:"port" desc:"HTTP 服务监听端口"`
	DataPath string `flag:"data-path" desc:"数据文件路径"`
	LogLevel string `flag:"log-level" desc:"日志等级"`
	TCPPort  int    `flag:"tcp-port" desc:"TCP 监听端口"`
	Verbose  bool   `flag:"verbose" desc:"是否输出详细日志"`
}

/* 命令行参数解析 */
func PT_NSQReflect00() {

	opts := &NSQDOptions{}

	// 自定义 FlagSet，避免未知参数导致退出
	// 或者直接定义 fs := flag.CommandLine,不能避免未知参数会导致退出
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	// 使用默认的 flag.CommandLine
	bindFlags(opts, fs)

	// 解析命令行参数
	// flag.Parse()
	// 你可以忽略 fs.Parse 错误来避免调试器注入的参数导致退出
	fs.Parse(os.Args[1:])

	logHG.DebugFInfo("配置结果:\n%+v\n", opts, "Port:", (*opts).Port)
}

func bindFlags(options interface{}, flagSet *flag.FlagSet) {

	/* reflect.ValueOf(options).Elem()
	options 是一个结构体指针，比如 &NSQDOptions{}
	reflect.ValueOf(options) 得到的是 *NSQDOptions 的 Value
	.Elem() 取得指针指向的值，也就是结构体本身 MyOptions。
	*/
	val := reflect.ValueOf(options)
	logHG.DebugFInfo("val值为：%+v", val) 
	//2025/07/27 14:12:28 🔥 val值为：&{Version:false Port:0 DataPath: LogLevel: TCPPort:0 Verbose:false}
	
	logHG.DebugFInfo("val值为：%v, 反射类型为：%T", val, val) 
	//2025/07/27 14:13:54 🔥 val类型为：&{false 0   0 false}, 反射类型为：reflect.Value

	valType := reflect.TypeOf(options)
	logHG.DebugFInfo("val的类型为：%v, 反射类型为：%T", valType, valType) 
	//2025/07/27 15:56:15 🔥 val的类型为：*nsq_practice_v1.NSQDOptions, 反射类型为：*reflect.rtype


	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		panic("options 必须是一个指向结构体的指针")
	}
	// .Elem() 取得指针指向的值，也就是结构体本身 NSQDOptions
	val = val.Elem()
	// 获取结构体类型（reflect.Type），用于访问字段元信息
	typ := val.Type()
	logHG.DebugFInfo("结构体本身：%+v 结构体类型：%+v", val, typ)
	// 🔥 结构体本身：{Version:false Port:0 DataPath: LogLevel: TCPPort:0 Verbose:false} 结构体类型：nsq_practice_v1.NSQDOptions
	// 记录合法的 flag 名称集合
	knownFlags := make(map[string]bool)

	// NumField() 获取字段数量
	for i := 0; i < typ.NumField(); i++ {
		// 得到每个字段的 reflect.StructField，包括字段名、tag、类型等。
		field := typ.Field(i)
		fieldVal := val.Field(i)
		logHG.DebugFInfo("field: %+v, fieldVal: %+v", field, fieldVal)
		// 比如结构体Port字段： 🔥 field: {Name:Port PkgPath: Type:int Tag:flag:"port" desc:"HTTP 服务监听端口" Offset:8 Index:[1] Anonymous:false}, fieldVal: 0
		/*
			// 示例：设置字段值（v 是提前准备好的值）
			fieldVal := val.FieldByName(field.Name)

			val.FieldByName(...) 获取具体字段的值（reflect.Value）
			field.Name: 字段名
			.Addr() 获取该字段的指针，以便后面可以修改它。
		*/
		_ = val.FieldByName(field.Name).Addr()
		logHG.DebugFInfo("val.FieldByName(field.Name): %+v, field.Name: %+v", val.FieldByName(field.Name), field.Name)
		//🔥 val.FieldByName(field.Name): false, field.Name: Version
		
		// 跳过非导出字段（如小写字母开头）
		if !fieldVal.CanSet() {
			continue
		}

		// 获取 flag 名称（必须）
		flagName := field.Tag.Get("flag")
		logHG.DebugFInfo("flagName: %+v", flagName) //🔥 flagName: version
		if flagName == "" {
			continue // 如果没有 tag 就跳过
		}
		// 可选：获取帮助信息
		desc := field.Tag.Get("desc")// desc 标签显示
		knownFlags["-"+flagName] = true // 记录合法 flag

		// 绑定不同类型的参数
		switch fieldVal.Kind() {
		case reflect.String:
			/*
				flagSet.StringVar()	将字段的地址和 flag 名字绑定起来
				fieldVal.Addr().Interface().(*string) 将字段指针强转为对应类型，供 flagSet 使用
			*/
			flagSet.StringVar(fieldVal.Addr().Interface().(*string), flagName, fieldVal.String(), desc)
		case reflect.Int:
			flagSet.IntVar(fieldVal.Addr().Interface().(*int), flagName, int(fieldVal.Int()), desc)
		case reflect.Bool:
			flagSet.BoolVar(fieldVal.Addr().Interface().(*bool), flagName, fieldVal.Bool(), desc)
		default:
			fmt.Fprintf(os.Stderr, "不支持 flag 类型：%s\n", fieldVal.Kind())
		}
	}

	// 过滤合法参数
	validArgs := filterKnownArgs(os.Args[1:], knownFlags)
	logHG.DebugFInfo("解析参数：%+v, \nknownFlags: %+v, \n有效参数：%+v", os.Args[1:], knownFlags, validArgs)
	//🔥 解析参数：[--version=true --port=8081 --data-path=/data/nsq --log-level=debug --tcp-port=4150 --verbose], knownFlags: map[-data-path:true -log-level:true -port:true -tcp-port:true -verbose:true -version:true], 有效参数：[]
	
	// 解析过滤后的参数
	_ = flagSet.Parse(validArgs)
}

// 过滤未定义的 flag 参数
func filterKnownArgs(args []string, known map[string]bool) []string {

	result := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// 如果是合法 flag 或其参数（不以 - 开头），就保留
		if strings.HasPrefix(arg, "-") {
			eqIndex := strings.Index(arg, "=")
			flagName := arg
			if eqIndex > 0 {
				flagName = arg[:eqIndex]
			}

			if known[flagName] {
				result = append(result, arg)
				// 如果没有 '=', 下一个是值，也要保留
				if eqIndex < 0 && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					result = append(result, args[i])
				}
			}
		}
	}
	return result
}
