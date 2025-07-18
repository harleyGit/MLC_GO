/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-13 11:20:13
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-13 11:22:22
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/model/error.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package model

import "errors"

// 根据业务逻辑需要，自定义一些错误
var (
	ERROR_USER_NOTEXISTS = errors.New("用户不存在....")
	ERROR_USER_EXISTS = errors.New("用户已经存在......")
	ERROR_USER_PWD = errors.New("密码不正确")
)