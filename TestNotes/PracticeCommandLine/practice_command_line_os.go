/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-27 22:19:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-27 22:35:29
 * @FilePath: /MLC_GO/TestNotes/PracticeCommandLine/practice_command_line_os.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"html/template"
	"os"
)

type userInfoOS struct {
	File string `json: "file"`
	Name string `json: "name"`
	Email string `json:"email"`
	Company string `json:"company"`
}

func main(){
	practiceCmdOs()
}

func (u *userInfoOS) practiceTemplate() {
	t := template.New("New Template for book")
	t, _ = t.Parse(`
	An example of os cli.
	Show User Information by template:
		File Name: {{.File}}
		Name: {{.Name}}
		Email: {{.Email}}
		Company: {{.Company}}
	Use "user help <topic>" for more information about that topic.
	`)
	t.Execute(os.Stdout, u)
}

func practiceCmdOs() {
	args := os.Args
	if len(args) != 4 {
		fmt.Println("you need add name, email, company field")
		return
	}
	var oneUserInfoOS userInfoOS
	oneUserInfoOS.File= os.Args[0]
	oneUserInfoOS.Name= os.Args[1]
	oneUserInfoOS.Email = os.Args[2]
	oneUserInfoOS.Company = os.Args[3]
	
	oneUserInfoOS.practiceTemplate()
}