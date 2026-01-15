/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:55:15
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-15 10:48:30
* @FilePath: /MLC_GO/internal/modules/user/handler/hg_user_handler.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
* 功能：HTTP层
* 注册 / 登录流程，强烈建议使用 Redis原因：
*	1.验证码是短期数据（TTL）
*	2.登录态 / Token 是短期数据
*	3.防刷、限流都依赖 Redis
*	4.不适合放数据库，更不适合内存 Map
*/
package UserHandlerPackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	PresentersPackage "MLC_GO/internal/interfaces/presenters"
	usermodelsPackage "MLC_GO/internal/models/user_models"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"MLC_GO/internal/pkg/logHG"
	PkgMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"encoding/json"
	"net/http"
	"strings"
	"time"
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
	verifyCodes = make(map[string]string) // 存储账号与验证码的映射关系【弃用】
	userAutoID  int64 = 1                             // 模拟自增用户ID
)




/* 处理发送验证码的逻辑
   测试： 
   curl -X POST http://localhost:8080/user/send_verify_code \ 
	-H "Content-Type: application/json" \
	-d '{"account":"test@example.com"}'
*/
/* 发送验证码，用到了redis */
func sendVerifyCodeHandlerV2(w http.ResponseWriter, r *http.Request) {
	
	var req verifyCodeReqModel
	json.NewDecoder(r.Body).Decode(&req)

	if req.Account == "" {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "Account is required"})
		return
	}
	code := utilsPackage.GenerateRandomNum(6)
	key := "verify:"+req.Account
	PersistenceRedisPackage.SetToRedis(key, code, 5 * time.Minute) // 5分钟过期

	logHG.DebugInfo("验证码发送到 account %s:，验证码： %s， 5分钟过期", req.Account, code)

	PresentersPackage.WriteJSON(w, map[string]string{"message": "验证码已发送"})
}
/* 发送验证码V1，用了缓存【弃用】 */
// Deprecated: 使用 sendVerifyCodeHandlerV2 替代
func sendVerifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	
	var req verifyCodeReqModel
	json.NewDecoder(r.Body).Decode(&req)

	if req.Account == "" {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "Account is required"})
		return
	}
	code := utilsPackage.GenerateRandomNum(6)
	verifyCodes[req.Account] = code

	logHG.DebugInfo("验证码发送到 account %s:，验证码： %s", req.Account, code)

	PresentersPackage.WriteJSON(w, map[string]string{"message": "验证码已发送"})
}


/* 注册 
   测试： 
   curl -X POST http://localhost:8080/user/register \
	-H "Content-Type: application/json" \
	-d '{"account":"test@example.com","code":"338122","password":"123456"}'
*/
func registerHandlerV3(w http.ResponseWriter, r *http.Request) {
	var req registerReqModel
	json.NewDecoder(r.Body).Decode(&req)

	if err := UserServicePackage.RegisterService(req.Account, req.Code, req.Password); err != nil {
		http.Error(w, "注册失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Write([]byte("注册成功"))
}
/* 注册V2版本，使用了Redis存储验证码 */
// Deprecated: 使用 registerHandlerV3 替代
func registerHandlerV2(w http.ResponseWriter, r *http.Request) {
	var req registerReqModel
	json.NewDecoder(r.Body).Decode(&req)	

	key := "verify:"+req.Account
	code, err := PersistenceRedisPackage.GetFromRedis(key)
	if err != nil || code != req.Code {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "验证码错误 or 已过期"})
		return
	}

	if _, ok := users[req.Account]; ok {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "账号已存在"})
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
	PersistenceRedisPackage.DeleteFromRedis(key) // rdb中删除验证码

	PresentersPackage.WriteJSON(w, map[string]any{"message": "注册成功", "id": user.UserID})
}
/* 注册V1版本，弃用【只是用到了缓存】 */
// Deprecated: 使用 registerHandlerV2 替代
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req registerReqModel
	json.NewDecoder(r.Body).Decode(&req)	

	if verifyCodes[req.Account] != req.Code {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "验证码错误"})
		return
	}

	if _, ok := users[req.Account]; ok {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "账号已存在"})
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

	PresentersPackage.WriteJSON(w, map[string]any{"message": "注册成功", "id": user.UserID})
}


/* 登录 
	测试： 注意 这个要与 http.HandleFunc("/user/login",loginHandler)第一个参数一致
	curl -X POST http://localhost:8080/user/login \
	-H "Content-Type: application/json" \
	-d '{"account":"test@example.com","password":"123456"}' 
*/
func loginHandlerV3(w http.ResponseWriter, r *http.Request) {
	var req loginReqModel
	json.NewDecoder(r.Body).Decode(&req)

	token, err := UserServicePackage.LoginService(req.Account, req.Password)
	if err != nil {
		http.Error(w, "登录失败: "+err.Error(), http.StatusUnauthorized)
		return
	}
	PresentersPackage.WriteJSON(w, map[string]any{"message": "登录成功", "token": token})
}
// Deprecated: 使用 loginHandlerV3 替代
func loginHandlerV2(w http.ResponseWriter, r *http.Request) {
	var req loginReqModel
	json.NewDecoder(r.Body).Decode(&req)

	user, ok := users[req.Account]
	if !ok {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "账号不存在"})
		return
	}

	hash := utilsPackage.HashPassword(req.Password, user.Salt)
	if hash != user.PasswordHash {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "密码错误"})
		return
	}
	token := utilsPackage.GenerateRandomNum(32)
	PersistenceRedisPackage.SetToRedis("token:"+token, user.UserID, 7*24 * time.Hour) // 24小时过期
	PresentersPackage.WriteJSON(w, map[string]any{"message": "登录成功", "id": user.UserID, "token": token})	
}
/* 登录版本V1，用缓存【弃用】 */
// Deprecated: 使用 loginHandlerV2 替代
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginReqModel
	json.NewDecoder(r.Body).Decode(&req)

	user, ok := users[req.Account]
	if !ok {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "账号不存在"})
		return
	}

	hash := utilsPackage.HashPassword(req.Password, user.Salt)
	if hash != user.PasswordHash {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "密码错误"})
		return
	}
	PresentersPackage.WriteJSON(w, map[string]any{"message": "登录成功", "id": user.UserID})
}

/* 受保护接口 */
func profile(w http.ResponseWriter, r *http.Request){
	PresentersPackage.WriteJSON(w, map[string]string{"message": "已通过认证"})
}


// Deprecated: 使用 RegisterUserRoutesV3 替代
func RegisterUserRoutesV3() {
	PersistenceSQLPackage.NewSQLDB()   // 初始化MySQL连接
	PersistenceRedisPackage.NewRedisService() // 初始化Redis连接
	
	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandlerV2)
	http.HandleFunc("/user/register", registerHandlerV2)
	http.HandleFunc("/user/login", loginHandlerV2)
	http.HandleFunc("/user/profile", PkgMiddlewarePackage.TokenAuthMiddleware(profile)) // 受保护接口
}

/* 路由注册 */
// Deprecated: 使用 RegisterUserRoutesV3 替代
func RegisterUserRoutesV2() {
	PersistenceRedisPackage.NewRedisService() // 初始化Redis连接
	
	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandlerV2)
	http.HandleFunc("/user/register", registerHandlerV2)
	http.HandleFunc("/user/login", loginHandlerV2)
	http.HandleFunc("/user/profile", PkgMiddlewarePackage.TokenAuthMiddleware(profile)) // 受保护接口
}

/* 路由注册 */
// TODO: 弃用，用的是缓存
// Deprecated: 使用 RegisterUserRoutesV2 替代
func RegisterUserRoutes() {
	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandler)
	http.HandleFunc("/user/register", registerHandler)
	http.HandleFunc("/user/login", loginHandler)
	http.HandleFunc("/user/profile", PkgMiddlewarePackage.TokenAuthMiddleware(profile)) // 受保护接口
}
