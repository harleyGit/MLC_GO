/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 10:55:15
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 21:22:41
 * @FilePath: /MLC_GO/internal/modules/user/handler/hg_user_handler.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserhandlerPackage

import (
	usermodelsPackage "MLC_GO/internal/models/user_models"
	"MLC_GO/internal/pkg/logHG"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"encoding/json"
	"net/http"
	"strings"
)

// 验证码响应结构体
type verifyCodeReqModel struct {
	Account string `json:"account"` // email or phone
}

/* 注册响应结构体 */
type registerReqModel struct{
	Account string `json: "account"`// email or phone
	Code string `json:"code"`
	Password string `json:"password"`
}

/* 登录响应结构体 */
type loginReqModel struct{
	Account string `json: "account"`// email or phone
	Code string `json:"code"`
	Password string `json:"password"`
}

var (
	users = make(map[string]*usermodelsPackage.HGUserModel)      // 存储用户账号与密码的映射关系
	verifyCodes = make(map[string]string) // 存储账号与验证码的映射关系
	userAutoID  int64 = 1                             // 模拟自增用户ID
)


// 辅助函数：将响应写为 JSON 格式
// v any：Go 1.18+ 引入的泛型语法，any 是 interface{} 的别名，表示可以传入任意类型的值（比如 struct、map、slice 等
func writeJSON(w http.ResponseWriter, v any)  {
	w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w) 创建一个将数据编码为 JSON 并直接写入 w（即 HTTP 响应流）的编码器。
	// .Encode(v) 将变量 v 序列化为 JSON，并写入响应
	json.NewEncoder(w).Encode(v)
}

/* 处理发送验证码的逻辑
   测试： 
   curl -X POST http://localhost:8080/user/send_verify_code \ 
	-H "Content-Type: application/json" \
	-d '{"account":"test@example.com"}'
*/
func sendVerifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	
	var req verifyCodeReqModel
	json.NewDecoder(r.Body).Decode(&req)

	if req.Account == "" {
		writeJSON(w, map[string]string{"error": "Account is required"})
		return
	}
	code := utilsPackage.GenerateRandomNum(6)
	verifyCodes[req.Account] = code

	logHG.DebugInfo("验证码发送到 account %s:，验证码： %s", req.Account, code)

	writeJSON(w, map[string]string{"message": "验证码已发送"})
}


/* 注册 
   测试： 
   curl -X POST http://localhost:8080/user/register \
	-H "Content-Type: application/json" \
	-d '{"account":"test@example.com","code":"338122","password":"123456"}'
*/
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req registerReqModel
	json.NewDecoder(r.Body).Decode(&req)	

	if verifyCodes[req.Account] != req.Code {
		writeJSON(w, map[string]string{"error": "验证码错误"})
		return
	}

	if _, ok := users[req.Account]; ok {
		writeJSON(w, map[string]string{"error": "账号已存在"})
		return
	}

	salt := utilsPackage.GenerateRandomSalt()
	hash := utilsPackage.HashPassword(req.Password, salt)

	user := &usermodelsPackage.HGUserModel{
		UserID:       userAutoID,
		Username:     req.Account,
		PasswordHash: hash,
		Salt:         salt,
	}

	// 检查字符串 req.Acount 中是否包含字符 "@"
	if strings.Contains(req.Account, "@") {
		user.Email = req.Account
	} else {
		user.Phone = req.Account
	}

	users[req.Account] = user
	userAutoID++

	// 从名为 verifyCodes 的 map 中删除键为 req.Account 的键值对。
	delete(verifyCodes, req.Account)

	writeJSON(w, map[string]any{"message": "注册成功", "id": user.UserID})
}

/* 登录 
	测试：
	curl -X POST http://localhost:8080/user/login \
	-H "Content-Type: application/json" \
	-d '{"account":"test@example.com","password":"123456"}' 
*/
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginReqModel
	json.NewDecoder(r.Body).Decode(&req)

	user, ok := users[req.Account]
	if !ok {
		writeJSON(w, map[string]string{"error": "账号不存在"})
		return
	}

	hash := utilsPackage.HashPassword(req.Password, user.Salt)
	if hash != user.PasswordHash {
		writeJSON(w, map[string]string{"error": "密码错误"})
		return
	}
	writeJSON(w, map[string]any{"message": "登录成功", "id": user.UserID})
}

/* 路由注册 */
func RegisterUserRoutes() {
	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandler)
	http.HandleFunc("/user/register", registerHandler)
	http.HandleFunc("/user/login", loginHandler)
}
