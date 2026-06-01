package UserHandlerPackage

import (
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserServicePackage "MLC_GO/internal/modules/user/service"
)

// HGUserHandlerDeps 声明 HTTP 处理层依赖，由 module 装配层统一注入。
// 大厂项目通常不会在 handler 内部创建 repo/cache/db，因为这会让 HTTP 层和数据层耦合，降低可测试性。
type HGUserHandlerDeps struct {
	UserService     *UserServicePackage.UserService
	TokenService    *UserServicePackage.HGAuthService
	AvatarSvc       *UserServicePackage.AvatarService
	SMSSender       HGSMSPackage.HGSender
	ClickCaptchaSvc *UserServicePackage.ClickCaptchaService
}

// HGUserHandler 是 user 模块 HTTP 入口聚合器。
// 该结构只持有 service 依赖；具体方法按业务能力拆到 hg_auth_handler.go、hg_profile_handler.go、hg_avatar_handler.go。
type HGUserHandler struct {
	svc             *UserServicePackage.UserService
	tokenService    *UserServicePackage.HGAuthService
	smsSender       HGSMSPackage.HGSender
	avatarSvc       *UserServicePackage.AvatarService
	clickCaptchaSvc *UserServicePackage.ClickCaptchaService
}

// NewUserHandler 创建用户处理器。
// handler 只负责 HTTP 入参、错误码和响应，不负责 new repository/cache/database。
func NewUserHandler(deps HGUserHandlerDeps) *HGUserHandler {
	smsSender := deps.SMSSender
	if smsSender == nil {
		// 本地开发或测试未注入真实短信网关时使用 mock，避免启动阶段因为外部供应商缺失失败。
		smsSender = HGSMSPackage.NewMockSender()
	}

	return &HGUserHandler{
		svc:             deps.UserService,
		tokenService:    deps.TokenService,
		smsSender:       smsSender,
		avatarSvc:       deps.AvatarSvc,
		clickCaptchaSvc: deps.ClickCaptchaSvc,
	}
}
