package module

import (
	kafka "MLC_GO/internal/pkg/kafka"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 单独运行 IAM 时无需创建聊天消费者、RPC 服务或后台设备任务。
func TestHGIAMOnlyRuntimeDoesNotStartChat(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := HGConfig{
		Realtime: true, IAMOnly: true,
		centrifugoURL: "https://gateway.example.com",
		centrifugoKey: strings.Repeat("a", 24), centrifugoSecret: strings.Repeat("b", 48),
	}
	runtime, err := HGRegisterConfigured(db, "mlc.business.v1", cfg, kafka.HGClusterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if runtime.gateway == nil || runtime.client != nil || runtime.realtime != nil || runtime.rpc != nil || runtime.media != nil || runtime.calls != nil {
		t.Fatal("IAM 独立模式错误启动了聊天、Kafka 或媒体能力")
	}
}
