package HGRouterPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"net/http"
)

// authRoutes 返回 auth 模块完整路由定义。
// 路由清单按模块拆分，避免 hg_route_groups.go 聚合过多配置后难以维护。
func authRoutes(userHandler *UserHandlerPackage.HGUserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", nil),
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_email_code", false, "发送邮箱验证码", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register_with_email", false, "邮箱注册", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/refresh", false, "刷新 Token", nil),
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_reset_code", false, "发送忘记密码验证码", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/reset_password", false, "忘记密码重置", nil),
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/click_captcha", false, "获取点选验证码", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/verify_click_captcha", false, "验证点选验证码", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", userHandler.SendCode),
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_email_code", false, "发送邮箱验证码", userHandler.SendEmailCode),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", userHandler.RegisterHandlerV3),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register_with_email", false, "邮箱注册", userHandler.RegisterWithEmail),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", userHandler.Login),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/refresh", false, "刷新 Token", userHandler.RefreshToken),
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_reset_code", false, "发送忘记密码验证码", userHandler.SendResetPasswordCode),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/reset_password", false, "忘记密码重置", userHandler.ResetPassword),
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/click_captcha", false, "获取点选验证码", userHandler.GetClickCaptcha),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/verify_click_captcha", false, "验证点选验证码", userHandler.VerifyClickCaptcha),
	}
}
