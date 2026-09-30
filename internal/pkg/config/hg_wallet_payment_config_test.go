package ConfigPackage

import (
	"github.com/spf13/viper"
	"testing"
)

func TestIsWalletDebugPaymentEnabledRequiresStrictDebugAndExplicitFlag(t *testing.T) {
	hgPrevious := viper.Get(hgLoadedEnvKey)
	t.Cleanup(func() { viper.Set(hgLoadedEnvKey, hgPrevious) })
	viper.Set(hgLoadedEnvKey, "debug")
	for _, hgCase := range []struct {
		env, flag string
		want      bool
	}{
		{"debug", "true", true},
		{"debug", "", false},
		{"debug", "false", false},
		{"debug", "TRUE", false},
		{"debug", "1", false},
		{"pre", "true", false},
		{"prod", "true", false},
		{"", "true", false},
	} {
		t.Run(hgCase.env+"/"+hgCase.flag, func(t *testing.T) {
			t.Setenv("SERVER_ENV", hgCase.env)
			t.Setenv("MLC_WALLET_DEBUG_PAYMENT_ENABLED", hgCase.flag)
			if got := IsWalletDebugPaymentEnabled(); got != hgCase.want {
				t.Fatalf("enabled=%v, want %v", got, hgCase.want)
			}
		})
	}
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("MLC_WALLET_DEBUG_PAYMENT_ENABLED", "true")
	for _, hgLoaded := range []string{"", "pre", "prod"} {
		viper.Set(hgLoadedEnvKey, hgLoaded)
		if IsWalletDebugPaymentEnabled() {
			t.Fatalf("loaded environment %q must deny payment", hgLoaded)
		}
	}
}
