package ConfigPackage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitRuntimeEnvPreservesExternalEnvironment(t *testing.T) {
	t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))
	t.Setenv("SERVER_ENV", "pre")

	if err := InitRuntimeEnv(); err != nil {
		t.Fatalf("InitRuntimeEnv() error = %v", err)
	}
	if got := os.Getenv("SERVER_ENV"); got != "pre" {
		t.Fatalf("SERVER_ENV = %q, want external value pre", got)
	}
}

func TestInfrastructureConfiguredForEveryEnvironment(t *testing.T) {
	type expectedConfig struct {
		mysqlHost     string
		mysqlPort     string
		mysqlUser     string
		mysqlPassword string
		mysqlDatabase string
		redisHost     string
		redisPort     string
		migrate       int
	}

	expectedByEnv := map[string]expectedConfig{
		"debug": {"127.0.0.1", "3306", "root", "hh109", "HG_MLC_DB", "127.0.0.1", "6379", 9},
		"pre":   {"127.0.0.1", "3308", "root", "hh109", "HG_MLC_PRE_DB", "127.0.0.1", "6380", 9},
		"prod":  {"prod-mysql.internal", "3306", "app", "********", "HG_MLC_PROD_DB", "prod-redis.internal", "6379", 9},
	}

	for _, env := range []string{"debug", "pre", "prod"} {
		t.Run(env, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))
			if err := LoadConfig(env); err != nil {
				t.Fatalf("LoadConfig(%q) error = %v", env, err)
			}

			mysqlConfig, err := GetMySQLConfig()
			if err != nil {
				t.Fatalf("GetMySQLConfig() error = %v", err)
			}
			redisConfig, err := GetRedisConfig()
			if err != nil {
				t.Fatalf("GetRedisConfig() error = %v", err)
			}

			expected := expectedByEnv[env]
			if mysqlConfig.Host != expected.mysqlHost || mysqlConfig.Port != expected.mysqlPort || mysqlConfig.User != expected.mysqlUser || mysqlConfig.Password != expected.mysqlPassword || mysqlConfig.Database != expected.mysqlDatabase || mysqlConfig.MigrateExpectVersion != expected.migrate {
				t.Fatalf("mysql config mismatch: host=%q port=%q user=%q database=%q migrate=%d", mysqlConfig.Host, mysqlConfig.Port, mysqlConfig.User, mysqlConfig.Database, mysqlConfig.MigrateExpectVersion)
			}
			if redisConfig.Host != expected.redisHost || redisConfig.Port != expected.redisPort {
				t.Fatalf("redis config = %+v, want host=%q port=%q", redisConfig, expected.redisHost, expected.redisPort)
			}
		})
	}
}

func TestInfrastructureConfigRejectsInvalidValues(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("mysql.host", "127.0.0.1")
	viper.Set("mysql.port", "invalid")
	viper.Set("mysql.user", "root")
	viper.Set("mysql.database", "db")
	viper.Set("mysql.migrate_expect_version", 1)
	viper.Set("redis.host", "127.0.0.1")
	viper.Set("redis.port", "70000")

	if _, err := GetMySQLConfig(); err == nil {
		t.Fatal("expected invalid mysql port to be rejected")
	}
	if _, err := GetRedisConfig(); err == nil {
		t.Fatal("expected invalid redis port to be rejected")
	}
}

func TestInitRuntimeEnvRejectsMissingFile(t *testing.T) {
	t.Setenv("MLC_CONFIG_DIR", filepath.Join(t.TempDir(), "missing"))
	if err := InitRuntimeEnv(); err == nil {
		t.Fatal("expected missing MLC.env to be rejected")
	}
}
