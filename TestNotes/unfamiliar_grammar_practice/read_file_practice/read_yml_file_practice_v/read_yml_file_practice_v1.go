/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-21 09:55:10
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-21 10:46:24
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/read_file_practice/read_yaml_file_practice_v/read_yml_file_practice_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package read_yml_file_practice_v

import (
	"MLC_GO/pkg/hglog"
	"os"

	"gopkg.in/yaml.v3"
)

//解析yml文件
type BaseInfo struct {
	// 结构体标签 yaml:"xxx"：告诉 yaml.Unmarshal 要映射哪个 YAML 字段。
	Port     string `yaml:"port"`
	Ip     	 string `yaml:"ip"`
	Host     string `yaml:"host"`
	// 嵌套结构体：Spring 代表 RedisEntity，用于存储 Redis 相关配置。
	Spring 	 RedisEntity `yaml:"spring"`
}

type RedisEntity struct {
	Redis     RedisData `yaml:"redis"`
}

type RedisData struct {
	Host     	string `yaml:"host"`
	Port     	string `yaml:"port"`
	DataBase    string `yaml:"dataBase"`
	Timeout     string `yaml:"timeout"`
}

type ReadYMLTFilePractice struct {}

// 协议
func (readYMLPractice *ReadYMLTFilePractice) ExecutePracticeNone() {
	hglog.DebugInfo("协议 读取yml文件配置 ReadJSONFilePractice ExecutePracticeNone")
}

func (readJSONPractice *ReadYMLTFilePractice) ReadYMLFilePractice_v1() {
	baseInfo  := BaseInfo{}
	// config := baseInfo.getConf("./conf/mlc_app.yml")
	// hglog.DebugInfo("yml 文件读取内容: ip=",string(config.Host)," port=",string(config.Port))

	baseInfo.getConf_v1("./conf/mlc_app.yml")
	hglog.DebugInfo("yml 文件读取内容: ip=",string(baseInfo.Host)," port=",string(baseInfo.Port))

}

func (c *BaseInfo) getConf(path string) *BaseInfo {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		hglog.ErrInfo("yml读取文件失败: ", err.Error())
	}
	// 使用 yaml.Unmarshal 解析 YAML，必须传入结构体指针 &config
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		hglog.ErrInfo("yml赋值失败: ",err.Error())
	}
	return c
}

func (c *BaseInfo) getConf_v1(path string) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		hglog.ErrInfo("yml读取文件失败: ", err.Error())
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		hglog.ErrInfo("yml赋值失败: ",err.Error())
	}
}