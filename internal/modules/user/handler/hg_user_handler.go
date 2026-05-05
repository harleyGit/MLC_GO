/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:55:15
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-05-05 17:01:49

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
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserCachePackage "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	PkGDevicePackage "MLC_GO/internal/pkg/device"
	"MLC_GO/internal/pkg/logHG"
	PkgMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	utilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 验证码响应结构体
type verifyCodeReqModel struct {
	Account string `json:"account"` // email or phone
}

/* 登录响应结构体 */
type loginReqModel struct {
	Account  string `json: "account"` // email or phone
	Code     string `json:"code"`
	Password string `json:"password"`
}

// UserHandlerDeps 用户处理器依赖
type UserHandlerDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
	SMSSender    HGSMSPackage.HGSender
}

type UserHandler struct {
	redisService *PersistenceRedisPackage.RedisService
	svc          *UserServicePackage.UserService
	tokenService *UserServicePackage.HGAuthService
	smsSender    HGSMSPackage.HGSender
}

var (
	users             = make(map[string]*UserModelsPackage.HGUserModel) // 存储用户账号与密码的映射关系
	verifyCodes       = make(map[string]string)                         // 存储账号与验证码的映射关系【弃用】
	userAutoID  int64 = 1                                               // 模拟自增用户ID
)

// NewUserHandler 创建用户处理器，内部创建所有依赖
func NewUserHandler(deps UserHandlerDeps) *UserHandler {
	// 创建基础设施依赖
	db := deps.SQLManager.GetSQLDB()
	redisClient := UserCachePackage.NewCodeCache(deps.RedisService)
	userCache := UserCachePackage.NewUserCache(deps.RedisService)
	userRepo := UserRepositoryPackage.NewUserRepo(db)

	// 创建 Service 层
	svc := UserServicePackage.NewUserService(userRepo, userCache, deps.RedisService)
	tokenService := UserServicePackage.NewAuthService(userRepo, redisClient)

	// 处理 SMS Sender
	smsSender := deps.SMSSender
	if smsSender == nil {
		smsSender = HGSMSPackage.NewMockSender()
	}

	return &UserHandler{
		redisService: deps.RedisService,
		svc:          svc,
		tokenService: tokenService,
		smsSender:    smsSender,
	}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var d UserDtoPackage.HGCreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.CreateUser(r.Context(), &d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// GET /profile/list?cursor=0&pageSize=20; “我要第一页，从最新的数据开始拿 20 条”。
func (h *UserHandler) GetUserList(w http.ResponseWriter, r *http.Request) {

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	page, _ := strconv.Atoi(r.URL.Query().Get("pageNum"))
	size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if size <= 0 || size > 1000 {
		size = 20
	}

	// 兼容旧参数 pageNum：
	// 新方案优先使用 cursor；若未传 cursor，则默认按首屏查询处理。
	// 深分页请改用上一次返回结果中的 nextCursor。
	if cursor <= 0 && page <= 1 {
		cursor = 0
	}

	resp, err := h.svc.GetUserList(r.Context(), cursor, size)
	if err != nil {
		HGResponsePakcage.FailResult[error](w, r, HGResponsePakcage.UserListFailCode, err.Error())
		return
	}

	//按理说写成HGResponsePakcage.SuccessResult[HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]](w, r, resp)
	// 但是看到第三个参数推断出T就是resp类型，也就是 HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO] 类型了
	HGResponsePakcage.SuccessResult(w, r, resp)
}

func (h *UserHandler) PathUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)

	var d UserDtoPackage.HGCreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := h.svc.PathUser(r.Context(), id, &d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// UpdateProfile 处理用户资料更新，支持单字段或多字段更新。
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}

	var req UserDtoPackage.HGUpdateUserProfileReqDTO
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "请求体格式错误")
		return
	}

	resp, err := h.svc.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			w.WriteHeader(http.StatusNotFound)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
			return
		case errors.Is(err, UserServicePackage.ErrProfileNoField),
			errors.Is(err, UserServicePackage.ErrProfileGenderInvalid),
			errors.Is(err, UserServicePackage.ErrProfileBirthDateInvalid):
			w.WriteHeader(http.StatusBadRequest)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "更新用户资料失败")
			return
		}
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// RegisterHandlerV3 处理用户注册
func (handler *UserHandler) RegisterHandlerV3(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	json.NewDecoder(r.Body).Decode(&req)
	// TODO：防止多次重复注册，注意下
	if err := UserServicePackage.RegisterService(r.Context(), req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserRegisterFail, "注册失败: "+err.Error())
		return
	}
	HGResponsePakcage.SuccessResult(w, r, req)
}

// SendCode 发送验证码
func (h *UserHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	// 从 URL 查询参数中获取 phone
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "缺少 phone 参数")
		return
	}

	// 调用 Service 层
	code, err := h.svc.SendCode(r.Context(), phone)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, err.Error())
		return
	}

	// 发送短信（Mock / 真实）
	if err := h.smsSender.Send(phone, code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "发送短信失败")
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"phone": phone, "message": "验证码已发送", "verifyCode": code})
}

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
	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(req.Account)
	PersistenceRedisPackage.SetToRedis(key, code, 5*time.Minute) // 5分钟过期

	logHG.DebugInfo("验证码发送到 account %s:，验证码： %s， 5分钟过期", req.Account, code)

	PresentersPackage.WriteJSON(w, map[string]string{"message": "验证码已发送"})
}

// Login 用户登录（支持验证码和密码两种方式）
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UserServicePackage.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "JSON 解析失败: "+err.Error())
		return
	}

	// 设置设备信息
	req.Device = PkGDevicePackage.Fingerprint(r)

	// 调用 Service 层
	resp, err := h.svc.Login(r.Context(), &req)
	if err != nil {
		switch err {
		case UserServicePackage.ErrUserNotFound:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
		case UserServicePackage.ErrPasswordIncorrect:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "密码不正确")
		case UserServicePackage.ErrCodeInvalid:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "验证码无效或已过期")
		case UserServicePackage.ErrPhoneOrEmailRequired:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "手机号或邮箱必填")
		default:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "登录失败: "+err.Error())
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// decodeRedisStringValue 兼容 Redis 中字符串值被 JSON 序列化后带引号的场景。
func decodeRedisStringValue(v string) string {
	var code string
	if err := json.Unmarshal([]byte(v), &code); err == nil {
		return code
	}
	return v
}

/*
	 登录
		测试： 注意 这个要与 http.HandleFunc("/user/login",loginHandler)第一个参数一致
		curl -X POST http://localhost:8080/user/login \
		-H "Content-Type: application/json" \
		-d '{"account":"test@example.com","password":"123456"}'
*/
// Deprecated: 使用 Login 替代
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

/* Profile 方法（需要中间件） */
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
	if !ok {
		logHG.ErrInfo("用户信息Profile error: %v", ok)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, "unauthorized")
		return
	}

	userDTO, err := h.svc.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		logHG.ErrFInfo("用户信息Profile error: %v", err)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在"+err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, userDTO)
}

// parseUpdateUserID 解析资料更新目标 user_id，优先读取 query 参数，缺失时尝试从 JWT claims 获取。
func parseUpdateUserID(r *http.Request) (string, error) {
	userIDText := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userIDText == "" {
		claims, ok := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
		if ok && claims != nil {
			userIDText = strings.TrimSpace(claims.UserID)
		}
	}
	if userIDText == "" {
		return "", errors.New("缺少 user_id 参数")
	}

	return userIDText, nil
}

/* 受保护接口 */
func profile(w http.ResponseWriter, r *http.Request) {
	PresentersPackage.WriteJSON(w, map[string]string{"message": "已通过认证"})
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

/* 注册V2版本，使用了Redis存储验证码 */
// Deprecated: 使用 RegisterHandlerV3 替代
func registerHandlerV2(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	json.NewDecoder(r.Body).Decode(&req)

	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(req.Account)
	code, err := PersistenceRedisPackage.GetFromRedis(r.Context(), key)
	if err != nil {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "验证码错误 or 已过期"})
		return
	}
	if decodeRedisStringValue(code) != req.Code {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "验证码错误 or 已过期"})
		return
	}

	if _, ok := users[req.Account]; ok {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "账号已存在"})
		return
	}

	salt := utilsPackage.GenerateRandomSalt()
	hash := utilsPackage.HashPassword(req.Password, salt)

	user := &UserModelsPackage.HGUserModel{
		ID:           userAutoID,
		Username:     utilsPackage.StrPtrToNullStr(&req.Account),
		PasswordHash: utilsPackage.StrPtrToNullStr(&hash),
		Salt:         utilsPackage.StrPtrToNullStr(&salt),
	}

	// 检查字符串 req.Acount 中是否包含字符 "@"
	if strings.Contains(req.Account, "@") {
		user.Email = utilsPackage.StrPtrToNullStr(&req.Account)
	} else {
		user.Phone = utilsPackage.StrPtrToNullStr(&req.Account)
	}

	users[req.Account] = user
	userAutoID++
	PersistenceRedisPackage.DeleteFromRedis(key, PersistenceRedisPackage.WithContext(r.Context())) // rdb中删除验证码

	PresentersPackage.WriteJSON(w, map[string]any{"message": "注册成功", "id": user.UserID})
}

/* 注册V1版本，弃用【只是用到了缓存】 */
// Deprecated: 使用 registerHandlerV2 替代
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
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

	user := &UserModelsPackage.HGUserModel{
		ID:           userAutoID,
		Username:     utilsPackage.StrPtrToNullStr(&req.Account),
		PasswordHash: utilsPackage.StrPtrToNullStr(&hash),
		Salt:         utilsPackage.StrPtrToNullStr(&salt),
	}

	// 检查字符串 req.Acount 中是否包含字符 "@"
	if strings.Contains(req.Account, "@") {
		user.Email = utilsPackage.StrPtrToNullStr(&req.Account)
	} else {
		user.Phone = utilsPackage.StrPtrToNullStr(&req.Account)
	}

	users[req.Account] = user
	userAutoID++

	// 从名为 verifyCodes 的 map 中删除键为 req.Account 的键值对。
	delete(verifyCodes, req.Account)

	PresentersPackage.WriteJSON(w, map[string]any{"message": "注册成功", "id": user.UserID})
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

	hash := utilsPackage.HashPassword(req.Password, user.Salt.String)
	if hash != user.PasswordHash.String {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "密码错误"})
		return
	}
	token := utilsPackage.GenerateRandomNum(32)
	PersistenceRedisPackage.SetToRedis("token:"+token, user.UserID, 7*24*time.Hour) // 24小时过期
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

	hash := utilsPackage.HashPassword(req.Password, user.Salt.String)
	if hash != user.PasswordHash.String {
		PresentersPackage.WriteJSON(w, map[string]string{"error": "密码错误"})
		return
	}
	PresentersPackage.WriteJSON(w, map[string]any{"message": "登录成功", "id": user.UserID})
}

func RegisterUserRoutesV3() {
	PersistenceSQLPackage.NewSQLDB()          // 初始化MySQL连接
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
