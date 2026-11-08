package HGResponsePakcage

import (
	"strings"
	"testing"
)

func TestHGPermissionError(t *testing.T) {
	for _, tc := range []struct {
		match, expected string
	}{
		{"all", "alpha:write、beta:run"},
		{"any", "alpha:write 或 beta:run"},
	} {
		got := HGPermissionError("需要有效的资源归属", tc.match, "alpha:write", "beta:run")
		if got.Code != Forbidden.Code || !strings.Contains(got.Message, tc.expected) || !strings.Contains(got.Message, "IAM") || !strings.Contains(got.Message, "资源归属") {
			t.Fatalf("权限错误缺失说明: %+v", got)
		}
	}
}
