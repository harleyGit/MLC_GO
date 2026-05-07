package HGUserModulePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	"testing"
)

// TestNewUserModuleComponents_NilInfrastructurePanics 明确模块装配层对基础设施依赖的要求。
// 生产启动必须先完成 Redis/MySQL 初始化，再进入 user 模块装配；这可以避免 handler 内部偷偷创建依赖。
func TestNewUserModuleComponents_NilInfrastructurePanics(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic when infrastructure deps are nil")
		}
	}()

	_ = NewUserModuleComponents(UserModuleDeps{
		RedisService: (*PersistenceRedisPackage.RedisService)(nil),
		SQLManager:   (*PersistenceSQLPackage.HGSQLManager)(nil),
	})
}
