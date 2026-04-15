/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:55:15
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-04-15 21:30:59

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
	BaseModelsPackage "MLC_GO/internal/models"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserCachePackage "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserMapperPackage "MLC_GO/internal/modules/user/mapper"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	PkGDevicePackage "MLC_GO/internal/pkg/device"
	"MLC_GO/internal/pkg/logHG"
	PkgMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	utilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

func NewUserHandler(redisService *PersistenceRedisPackage.RedisService,
	sqlManager *PersistenceSQLPackage.HGSQLManager,
	smsSender HGSMSPackage.HGSender,
) *UserHandler {
	db := sqlManager.GetSQLDB()
	redisClient := UserCachePackage.NewCodeCache(redisService)
	userCahce := UserCachePackage.NewUserCache(redisService)

	userRepo := UserRepositoryPackage.NewUserRepo(db)
	svc := UserServicePackage.NewUserService(userRepo, userCahce)
	tokenService := UserServicePackage.NewAuthService(userRepo, redisClient)

	return &UserHandler{redisService: redisService, svc: svc,
		tokenService: tokenService, smsSender: smsSender}
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

func (h *UserHandler) SendCode(w http.ResponseWriter, r *http.Request) {

	// 从 URL 查询参数中获取 phone
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		http.Error(w, "缺少 phone 参数", http.StatusBadRequest)
		return
	}
	req := UserDtoPackage.HGCreateUserDTO{
		Phone: &phone, // 假设结构体中有 Phone 字段
	}

	ctx := r.Context()
	code := utilsPackage.GenerateRandomNum(6)

	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(*req.Phone)
	// Redis：存验证码（5 分钟）
	err := h.redisService.SetToRedisV2(key, code, 1*time.Minute, ctx)
	if err != nil {
		http.Error(w, "redis error", 500)
		return
	}

	// 发送短信（Mock / 真实）
	if err := h.smsSender.Send(*req.Phone, code); err != nil {
		http.Error(w, "send sms failed", 500)
		return
	}
	req.Code = &code
	logHG.DebugFInfo("验证码发送到 account %s:，验证码： %s， 5分钟过期", *req.Phone, code)

	HGResponsePakcage.SuccessResult(w, r, req)
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

/*
	 注册
	   测试：
	   curl -X POST http://localhost:8080/user/register \
		-H "Content-Type: application/json" \
		-d '{"account":"test@example.com","code":"338122","password":"123456"}'
*/
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

/* 注册V2版本，使用了Redis存储验证码 */
// Deprecated: 使用  替代
func registerHandlerV2(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	json.NewDecoder(r.Body).Decode(&req)

	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(req.Account)
	code, err := PersistenceRedisPackage.GetFromRedis(r.Context(), key)
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

/* Login 方法（验证码 + JWT） */
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var uid int64 = 1
	var userModel *UserModelsPackage.HGUserModel
	var err error
	var cacheKey string

	var req UserDtoPackage.HGCreateUserDTO
	bodyV := r.Body
	jsonV := json.NewDecoder(bodyV)
	if err := jsonV.Decode(&req); err != nil {
		http.Error(w, "JSON 解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	device := PkGDevicePackage.Fingerprint(r)
	jti := uuid.NewString()
	// phone := r.FormValue("phone")
	// email := r.FormValue("email")
	// code := r.FormValue("code")
	// password := r.FormValue("password")

	if !UtilsPackage.IsEmpty(req.Phone) {
		// TODO: key最好放入某个文件中，太分散了
		cacheKey = PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.AuthLoginVerifyCodekKey, *req.Phone)
		userModel, err = PersistenceSQLPackage.GetUserByEmail(ctx, *req.Phone)
	} else if !UtilsPackage.IsEmpty(*req.Email) {
		cacheKey = PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.AuthLoginVerifyCodekKey, *req.Email)
		userModel, err = PersistenceSQLPackage.GetUserByEmail(ctx, *req.Email)
	} else {
		http.Error(w, "phone/code required", http.StatusBadRequest)
		return
	}
	if err != nil {
		// TODO: 这些错误可以常量化，用标准字符串表示
		http.Error(w, "用户不存在", http.StatusBadRequest)
		return
	}

	if UtilsPackage.IsEmpty(req.Code) { // 使用密码
		hashedPassword := utilsPackage.HashPassword(req.Password, userModel.Salt.String)
		if hashedPassword != userModel.PasswordHash.String {
			http.Error(w, "密码不正确", http.StatusBadRequest)
			return
		}
	} else { // 若是使用验证码
		// 校验验证码
		val, err := h.redisService.GetFromRedisV2(cacheKey, ctx)
		if err != nil || val != *req.Code {
			http.Error(w, "invalid code", http.StatusUnauthorized)
			return
		}
		// 删除验证码（一次性）
		h.redisService.DeleteFromRedis(cacheKey, ctx)
	}

	now := time.Now().UTC()
	userDto := UserMapperPackage.UserModelToDTO(userModel)
	claims := &UserServicePackage.HGClaims{
		UserID:  *userDto.UserID,
		Device:  device,
		JTI:     jti,
		TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * 60 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "mlc-go",
			Subject:   "user-token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(UserServicePackage.Secret))
	userMap := BaseModelsPackage.ModelToMap(userDto)
	userMap["access_token"] = signed

	// 🌟🌟🌟 关键点：写入 Redis 多端设备登录控制 // TODO: 简化下，不要这个地方太多套了过多层调用
	h.tokenService.Store(ctx,
		uid,
		device,
		jti,
		15*time.Minute)

	/** 刷新token生成
	        refreshClaims := &UserServicePackage.HGClaims{
			UserID:  1,
			Device:  PkGDevicePackage.Fingerprint(r), //r.UserAgent(),
			JTI:     uuid.NewString(),                //TODO:看看怎么产生的 比如：
			TokenTp: "refresh",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Issuer:    "mlc-go",
				Subject:   "user-token",
			},
		}

			  refreshToken, err := jwt.NewWithClaims(
		        jwt.SigningMethodHS256,
		        refreshClaims,
		    ).SignedString(s.secret)
		    if err != nil {
		        return nil, err
		    }

		    // -------- Refresh Token 状态入 Redis --------
		    key := "refresh:" + refreshJTI

		    if err := s.rdb.Set(
		        ctx,
		        key,
		        userID,
		        s.refreshTTL,
		    ).Err(); err != nil {
		        return nil, err
		    }
	*/

	// 设置 Content-Type 废弃
	// w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// 使用 json.MarshalIndent 生成格式化 JSON
	userDto.Token = &signed
	// 废弃
	// jsonBytes, err := json.MarshalIndent(userDto, "", "  ") // "" = 前缀，"  " = 每级缩进两个空格
	// if err != nil {
	// 	http.Error(w, "JSON 编码失败", http.StatusInternalServerError)
	// 	return
	// }

	// HGResponsePakcage.WriteJSON(w, r, userDto) // TODO:后面用下面的这个
	HGResponsePakcage.SuccessResult(w, r, userDto)
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

/* Profile 方法（需要中间件） */
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := map[string]any{
		"user_id": claims.UserID,
		"device":  claims.Device,
	}

	json.NewEncoder(w).Encode(resp)
}

/* 受保护接口 */
func profile(w http.ResponseWriter, r *http.Request) {
	PresentersPackage.WriteJSON(w, map[string]string{"message": "已通过认证"})
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
