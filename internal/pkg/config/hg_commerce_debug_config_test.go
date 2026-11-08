package ConfigPackage

import (
	"github.com/spf13/viper"
	"testing"
)

func TestHGCommerceDebugTradingRequiresThreeGates(t *testing.T) {
	previous := viper.Get(hgLoadedEnvKey)
	t.Cleanup(func() { viper.Set(hgLoadedEnvKey, previous) })
	for _, test := range []struct {
		process, loaded, flag string
		want                  bool
	}{
		{"debug", "debug", "true", true},
		{"debug", "debug", "", false},
		{"debug", "debug", "TRUE", false},
		{"pre", "debug", "true", false},
		{"prod", "debug", "true", false},
		{"debug", "pre", "true", false},
		{"debug", "prod", "true", false},
	} {
		t.Setenv("SERVER_ENV", test.process)
		t.Setenv("MLC_COMMERCE_DEBUG_TRADING", test.flag)
		viper.Set(hgLoadedEnvKey, test.loaded)
		if got := IsCommerceDebugTradingEnabled(); got != test.want {
			t.Fatalf("process=%q loaded=%q flag=%q: enabled=%t want=%t", test.process, test.loaded, test.flag, got, test.want)
		}
	}
}
