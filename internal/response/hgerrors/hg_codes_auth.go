package hgerrors

const (
	HGAuthRequestInvalid    Code = 1101006
	HGAuthTokenMissing      Code = 1101001
	HGAuthTokenExpired      Code = 1101002
	HGAuthTokenInvalid      Code = 1101003
	HGAuthRefreshExpired    Code = 1101004
	HGAuthSessionRevoked    Code = 1101005
	HGAuthUserNotFound      Code = 1102001
	HGAuthAccountDisabled   Code = 1102002
	HGAuthCredentialInvalid Code = 1102003
	HGAuthAccountLocked     Code = 1102004
	HGAuthUserAlreadyExists Code = 1102005
	HGAuthSMSCodeInvalid    Code = 1103001
	HGAuthSMSCodeExpired    Code = 1103002
	HGAuthMFARequired       Code = 1104001
	HGAuthMFACodeInvalid    Code = 1104002
	HGAuthMFAAlreadyEnabled Code = 1104003
	HGAuthMFANotEnrolled    Code = 1104004
	HGAuthMFANotEnabled     Code = 1104005
	HGAuthMFACodeUsed       Code = 1104006
	HGAuthMFARateLimited    Code = 1104007
)

var hgAuth = []Definition{
	{HGAuthRequestInvalid, "AUTH_REQUEST_INVALID", "请求身份校验失败", 401, false},
	{HGAuthTokenMissing, "AUTH_TOKEN_MISSING", "未携带 Token", 401, false},
	{HGAuthTokenExpired, "AUTH_TOKEN_EXPIRED", "Access Token 过期", 401, false},
	{HGAuthTokenInvalid, "AUTH_TOKEN_INVALID", "Token 无效", 401, false},
	{HGAuthRefreshExpired, "AUTH_REFRESH_EXPIRED", "Refresh Token 过期", 401, false},
	{HGAuthSessionRevoked, "AUTH_SESSION_REVOKED", "登录会话已撤销", 401, false},
	{HGAuthUserNotFound, "AUTH_USER_NOT_FOUND", "用户不存在", 404, false},
	{HGAuthAccountDisabled, "AUTH_ACCOUNT_DISABLED", "账号被禁用", 403, false},
	{HGAuthCredentialInvalid, "AUTH_CREDENTIAL_INVALID", "账号或密码错误", 401, false},
	{HGAuthAccountLocked, "AUTH_ACCOUNT_LOCKED", "账号已锁定", 403, false},
	{HGAuthUserAlreadyExists, "AUTH_USER_ALREADY_EXISTS", "用户已存在", 409, false},
	{HGAuthSMSCodeInvalid, "AUTH_SMS_CODE_INVALID", "验证码错误", 400, false},
	{HGAuthSMSCodeExpired, "AUTH_SMS_CODE_EXPIRED", "验证码过期", 400, false},
	{HGAuthMFARequired, "AUTH_MFA_REQUIRED", "需要 MFA 验证", 403, false},
	{HGAuthMFACodeInvalid, "AUTH_MFA_CODE_INVALID", "MFA 动态码错误", 400, false},
	{HGAuthMFAAlreadyEnabled, "AUTH_MFA_ALREADY_ENABLED", "当前账号已启用 MFA", 409, false},
	{HGAuthMFANotEnrolled, "AUTH_MFA_NOT_ENROLLED", "当前账号未绑定 MFA", 409, false},
	{HGAuthMFANotEnabled, "AUTH_MFA_NOT_ENABLED", "当前账号未启用 MFA", 409, false},
	{HGAuthMFACodeUsed, "AUTH_MFA_CODE_USED", "动态码已使用", 409, false},
	{HGAuthMFARateLimited, "AUTH_MFA_RATE_LIMITED", "MFA 尝试次数过多", 429, true},
}
