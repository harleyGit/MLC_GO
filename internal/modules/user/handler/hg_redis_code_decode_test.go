package UserHandlerPackage

import "testing"

func TestDecodeRedisStringValue_WithJSONString(t *testing.T) {
	got := decodeRedisStringValue(`"654321"`)
	if got != "654321" {
		t.Fatalf("decodeRedisStringValue() = %q, want %q", got, "654321")
	}
}

func TestDecodeRedisStringValue_WithRawString(t *testing.T) {
	got := decodeRedisStringValue("654321")
	if got != "654321" {
		t.Fatalf("decodeRedisStringValue() = %q, want %q", got, "654321")
	}
}

func TestDecodeRedisStringValue_WithInvalidJSON(t *testing.T) {
	raw := "{invalid-json"
	got := decodeRedisStringValue(raw)
	if got != raw {
		t.Fatalf("decodeRedisStringValue() = %q, want %q", got, raw)
	}
}
