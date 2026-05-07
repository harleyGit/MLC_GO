package UserServicePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	usercache "MLC_GO/internal/modules/user/cache"
	userrepository "MLC_GO/internal/modules/user/repository"
	"errors"
)

// UserService 是用户域业务服务聚合器。
// 按大厂常见写法，service 持有 repository/cache/基础服务依赖，具体业务方法按能力拆到 auth/profile/query 文件。
type UserService struct {
	repo         *userrepository.UserRepo              // repo 只负责 SQL 访问，不承载 HTTP 语义。
	userCache    *usercache.HGUserCache                // userCache 只负责用户列表等 Redis 缓存读写。
	redisService *PersistenceRedisPackage.RedisService // redisService 用于验证码、token 等需要 Redis 原生命令的业务能力。
}

var (
	// ErrProfileNoField 表示更新资料请求未包含任何可更新字段。
	ErrProfileNoField = errors.New("至少更新一个资料字段")
	// ErrProfileGenderInvalid 表示性别字段超出允许范围。
	ErrProfileGenderInvalid = errors.New("gender 仅支持 0/1/2")
	// ErrProfileBirthDateInvalid 表示出生日期格式不符合约定。
	ErrProfileBirthDateInvalid = errors.New("birth_date 仅支持 YYYY-MM-DD 或 YYYY-MM")
	// ErrUserNotFound 表示登录账号没有匹配用户。
	ErrUserNotFound = errors.New("用户不存在")
	// ErrPasswordIncorrect 表示密码登录时密码为空或哈希不匹配。
	ErrPasswordIncorrect = errors.New("密码不正确")
	// ErrCodeInvalid 表示验证码不存在、过期或与用户提交值不一致。
	ErrCodeInvalid = errors.New("验证码无效或已过期")
	// ErrPhoneOrEmailRequired 表示登录请求缺少手机号和邮箱。
	ErrPhoneOrEmailRequired = errors.New("手机号或邮箱必填")
)

// NewUserService 创建用户业务服务。
// 构造函数只保存依赖，不做网络访问和副作用，便于 module 装配和单元测试替换依赖。
func NewUserService(
	repo *userrepository.UserRepo,
	userCache *usercache.HGUserCache,
	redisService *PersistenceRedisPackage.RedisService,
) *UserService {
	return &UserService{
		repo:         repo,
		userCache:    userCache,
		redisService: redisService,
	}
}
