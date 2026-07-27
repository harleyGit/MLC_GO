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
		mysqlHost      string
		mysqlPort      string
		mysqlUser      string
		mysqlPassword  string
		mysqlDatabase  string
		redisHost      string
		redisPort      string
		migrate        int
		clickHouseHost string
		statGeneration string
	}

	expectedByEnv := map[string]expectedConfig{
		"debug": {"127.0.0.1", "3306", "root", "hh109", "HG_MLC_DB", "127.0.0.1", "6379", 10, "127.0.0.1", "v2"},
		"pre":   {"127.0.0.1", "3308", "root", "hh109", "HG_MLC_PRE_DB", "127.0.0.1", "6380", 10, "pre-clickhouse.internal", "v2"},
		"prod":  {"prod-mysql.internal", "3306", "app", "********", "HG_MLC_PROD_DB", "prod-redis.internal", "6379", 10, "prod-clickhouse.internal", "v2"},
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
			clickHouseConfig, statisticConfig, err := GetStatisticInfrastructureConfig()
			if err != nil {
				t.Fatalf("GetStatisticInfrastructureConfig() error = %v", err)
			}

			expected := expectedByEnv[env]
			if mysqlConfig.Host != expected.mysqlHost || mysqlConfig.Port != expected.mysqlPort || mysqlConfig.User != expected.mysqlUser || mysqlConfig.Password != expected.mysqlPassword || mysqlConfig.Database != expected.mysqlDatabase || mysqlConfig.MigrateExpectVersion != expected.migrate {
				t.Fatalf("mysql config mismatch: host=%q port=%q user=%q database=%q migrate=%d", mysqlConfig.Host, mysqlConfig.Port, mysqlConfig.User, mysqlConfig.Database, mysqlConfig.MigrateExpectVersion)
			}
			if redisConfig.Host != expected.redisHost || redisConfig.Port != expected.redisPort {
				t.Fatalf("redis config = %+v, want host=%q port=%q", redisConfig, expected.redisHost, expected.redisPort)
			}
			if clickHouseConfig.Host != expected.clickHouseHost || statisticConfig.RedisGeneration != expected.statGeneration || statisticConfig.RedisShardCount != 64 {
				t.Fatalf("statistic infrastructure config = clickhouse:%+v statistic:%+v", clickHouseConfig, statisticConfig)
			}
		})
	}
}

func TestStatisticInfrastructureConfigRejectsInvalidGeneration(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("clickhouse.enabled", true)
	viper.Set("clickhouse.scheme", "http")
	viper.Set("clickhouse.host", "127.0.0.1")
	viper.Set("clickhouse.port", "8123")
	viper.Set("clickhouse.database", "mlc")
	viper.Set("clickhouse.user", "app")
	viper.Set("clickhouse.statistic_events_table", "statistic_events")
	viper.Set("clickhouse.statistic_totals_table", "statistic_event_totals")
	viper.Set("clickhouse.write_timeout", "5s")
	viper.Set("clickhouse.query_timeout", "15s")
	viper.Set("statistic.redis_generation", "invalid generation")
	viper.Set("statistic.redis_shard_count", 64)
	viper.Set("statistic.reconcile_interval", "5m")
	viper.Set("statistic.reconcile_timeout", "20s")

	if _, _, err := GetStatisticInfrastructureConfig(); err == nil {
		t.Fatal("expected invalid statistic generation to be rejected")
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
