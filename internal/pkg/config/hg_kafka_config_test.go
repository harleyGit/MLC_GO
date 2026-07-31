package ConfigPackage

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGetKafkaConfigRejectsEmptyBusinessBrokers(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	_, enabled, err := GetKafkaConfig()
	if err == nil {
		t.Fatal("expected empty business kafka brokers to be rejected")
	}
	if enabled {
		t.Fatal("expected kafka disabled when required business brokers are invalid")
	}
	if !strings.Contains(err.Error(), "business.brokers") {
		t.Fatalf("expected missing business.brokers error, got %v", err)
	}
}

func TestKafkaConfiguredForEveryEnvironment(t *testing.T) {
	type expectedConfig struct {
		serverPort     int
		logLevel       string
		businessRetry  int
		businessClient string
		logRetry       int
		logClient      string
	}

	expectedByEnv := map[string]expectedConfig{
		"debug": {
			serverPort:     8080,
			logLevel:       "debug",
			businessRetry:  3,
			businessClient: "mlc-go-debug-business",
			logRetry:       1,
			logClient:      "mlc-go-debug-log",
		},
		"pre": {
			serverPort:     8080,
			logLevel:       "info",
			businessRetry:  3,
			businessClient: "mlc-go-pre-business",
			logRetry:       1,
			logClient:      "mlc-go-pre-log",
		},
		"prod": {
			serverPort:     80,
			logLevel:       "info",
			businessRetry:  5,
			businessClient: "mlc-go-prod-business",
			logRetry:       3,
			logClient:      "mlc-go-prod-log",
		},
	}

	configDir := projectConfigDir(t)
	for _, env := range []string{"debug", "pre", "prod"} {
		t.Run(env, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			t.Setenv("MLC_CONFIG_DIR", configDir)

			if err := LoadConfig(env); err != nil {
				t.Fatalf("load %s config: %v", env, err)
			}

			expected := expectedByEnv[env]
			if got := viper.GetInt("server.port"); got != expected.serverPort {
				t.Fatalf("server.port = %d, want %d", got, expected.serverPort)
			}
			if got := viper.GetString("log.level"); got != expected.logLevel {
				t.Fatalf("log.level = %q, want %q", got, expected.logLevel)
			}
			cfg, enabled, err := GetKafkaConfig()
			if err != nil {
				t.Fatalf("get %s kafka config: %v", env, err)
			}
			if !enabled {
				t.Fatalf("expected %s kafka config to be enabled", env)
			}
			if cfg.Business.Retry != expected.businessRetry || cfg.Business.ClientID != expected.businessClient {
				t.Fatalf("business config = retry %d client_id %q, want retry %d client_id %q", cfg.Business.Retry, cfg.Business.ClientID, expected.businessRetry, expected.businessClient)
			}
			if !reflect.DeepEqual(cfg.Business.Topics, []string{"mlc.domain.events"}) {
				t.Fatalf("business topics = %v, want [mlc.domain.events]", cfg.Business.Topics)
			}
			groups := cfg.Business.Consumers
			if groups.Feed.GroupID == "" || groups.Search.GroupID == "" || groups.Statistic.GroupID == "" || groups.Audit.GroupID == "" || groups.Interaction.GroupID == "" {
				t.Fatalf("consumer groups must be configured: %+v", groups)
			}
			if !groups.Feed.Enabled {
				t.Fatalf("feed consumer must be enabled after its idempotent read model is implemented: %+v", groups)
			}
			if groups.Search.Enabled || groups.Statistic.Enabled || groups.Audit.Enabled {
				t.Fatalf("consumers without complete production dependencies must remain disabled: %+v", groups)
			}
			if !groups.Interaction.Enabled {
				t.Fatalf("interaction consumer must be enabled for asynchronous persistence: %+v", groups)
			}
			if cfg.Log.Retry != expected.logRetry || cfg.Log.ClientID != expected.logClient {
				t.Fatalf("log config = retry %d client_id %q, want retry %d client_id %q", cfg.Log.Retry, cfg.Log.ClientID, expected.logRetry, expected.logClient)
			}
			if len(cfg.Business.Brokers) != 3 || len(cfg.Log.Brokers) != 3 {
				t.Fatalf("expected environment broker lists to replace base values, got business=%v log=%v", cfg.Business.Brokers, cfg.Log.Brokers)
			}
		})
	}
}

func TestLoadConfigRejectsUnsupportedEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))

	err := LoadConfig("production")
	if err == nil {
		t.Fatal("expected unsupported environment to be rejected")
	}
	if !strings.Contains(err.Error(), "不支持的运行环境") {
		t.Fatalf("expected unsupported environment error, got %v", err)
	}
}

func TestLoadConfigDoesNotRetainPreviousEnvironmentValues(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))

	if err := LoadConfig("debug"); err != nil {
		t.Fatalf("load debug config: %v", err)
	}
	if err := LoadConfig("prod"); err != nil {
		t.Fatalf("load prod config: %v", err)
	}

	if got := viper.GetString("kafka.business.client_id"); got != "mlc-go-prod-business" {
		t.Fatalf("kafka.business.client_id = %q, want prod value", got)
	}
	if got := viper.GetString("log.level"); got != "info" {
		t.Fatalf("log.level = %q, want prod value", got)
	}
}

func TestGetServerPortUsesEnvironmentOverrideThenLoadedConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))
	t.Setenv("SERVER_PORT", "")

	if err := LoadConfig("prod"); err != nil {
		t.Fatalf("load prod config: %v", err)
	}
	if got := GetServerPort(); got != "80" {
		t.Fatalf("GetServerPort() = %q, want loaded prod port 80", got)
	}

	t.Setenv("SERVER_PORT", "9090")
	if got := GetServerPort(); got != "9090" {
		t.Fatalf("GetServerPort() = %q, want environment override 9090", got)
	}
}

func TestGetManagementPortUsesEnvironmentOverrideThenLoadedConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))
	t.Setenv("MANAGEMENT_PORT", "")

	if err := LoadConfig("prod"); err != nil {
		t.Fatalf("load prod config: %v", err)
	}
	if got := GetManagementPort(); got != "9091" {
		t.Fatalf("GetManagementPort() = %q, want 9091", got)
	}

	t.Setenv("MANAGEMENT_PORT", "19091")
	if got := GetManagementPort(); got != "19091" {
		t.Fatalf("GetManagementPort() = %q, want environment override 19091", got)
	}
}

func TestGetManagementHostDefaultsToLoopbackAndAllowsEnvironmentOverride(t *testing.T) {
	t.Setenv("MANAGEMENT_HOST", "")
	viper.Set("management.host", "")
	if got := GetManagementHost(); got != "127.0.0.1" {
		t.Fatalf("GetManagementHost() = %q, want 127.0.0.1", got)
	}
	t.Setenv("MANAGEMENT_HOST", "0.0.0.0")
	if got := GetManagementHost(); got != "0.0.0.0" {
		t.Fatalf("GetManagementHost() = %q, want environment override", got)
	}
}

func projectConfigDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "config")
}
