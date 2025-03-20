/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 21:14:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 21:46:14
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/read_file_practice/read_json_file_practice_v/read_json_file_practice_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 *
 * title: JSON文件读取
 */
package read_json_file_practice_v

import (
	"MLC_GO/pkg/hglog"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// 定义了一个新的类型 Configs，它是 map[string]json.RawMessage 的别名
// 	json.RawMessage 是 []byte 的别名，用于存储未解析的 JSON 数据
// 	它本质上是一个 map，键（string）表示配置项名称，值（json.RawMessage）存储的是JSON 格式的原始数据。
type Configs map[string]json.RawMessage
var configPath string = "./conf/mlc_app.json"

type ReadJSONFilePractice struct {

}


type MainConfig struct {
	Port string `json:"port"`
	Address string `json:"address"`
}


var confModel *MainConfig
var confMap Configs

var instanceOnce sync.Once

// 协议
func (readJSONPractice *ReadJSONFilePractice) ExecutePracticeNone() {
	hglog.DebugInfo("协议 读取JSON文件配置 ReadJSONFilePractice ExecutePracticeNone")
}

func (readJSONPractice *ReadJSONFilePractice) ReadJSONFilePractice_v1() {
	path := ConfigPath()
	hglog.DebugInfo("文件路径 path: ",path)
	
	Init(path)
	value := confMap["port"]
	hglog.DebugInfo("端口号 port: ",string(value))
}


//从配置文件中载入json字符串
func LoadConfig(path string) (Configs, *MainConfig) {
	// 读取文件内容
	buf, err := os.ReadFile(path)
	if err != nil {
		hglog.ErrInfo("load config conf failed: ", err)
	}
	mainConfig := &MainConfig{}
	// 用于将 JSON 字节数据 转换为 Go 结构体或 map。
	err = json.Unmarshal(buf, mainConfig)
	if err != nil {
		hglog.ErrInfo("decode config file failed:", string(buf), err)
	}
	// 创建一个 Configs 映射
	allConfigs := make(Configs)
	err = json.Unmarshal(buf, &allConfigs)
	if err != nil {
		hglog.ErrInfo("decode config file failed:", string(buf), err)
	}

	return allConfigs, mainConfig
}

//初始化 可以运行多次
func SetConfig(path string) {
	allConfigs, mainConfig := LoadConfig(path)
	configPath = path
	confModel = mainConfig
	confMap = allConfigs
}

// 初始化，只能运行一次
func Init(path string) *MainConfig {
	if confModel != nil && path != configPath {
		hglog.ErrInfo("the config is already initialized, oldPath=", configPath, "path= ",  path)
	}
	instanceOnce.Do(func() {
		allConfigs, mainConfig := LoadConfig(path)
		configPath = path
		confModel = mainConfig
		confMap = allConfigs
	})

	return confModel
}

//初始化配置文件 为 struct 格式
func Instance() *MainConfig {
	if confModel == nil {
		Init(configPath)
	}
	return confModel
}


//初始化配置文件 为 map格式
func AllConfig() Configs {
	if confModel == nil {
		Init(configPath)
	}
	return confMap
}

//获取配置文件路径
func ConfigPath() string {
	return configPath
}

//根据key获取对应的值，如果值为struct，则继续反序列化
func (cfg Configs) GetConfig(key string, config interface{}) error {
	c, ok := cfg[key]
	if ok {
		return json.Unmarshal(c, config)
	} else {
		return fmt.Errorf("fail to get cfg with key: %s", key)
	}
}


