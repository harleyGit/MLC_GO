package UserServicePackage

import "testing"

func TestDecodeRedisStringValue_WithJSONString(t *testing.T) {
	got := decodeRedisStringValue(`"123456"`)
	if got != "123456" {
		t.Fatalf("decodeRedisStringValue() = %q, want %q", got, "123456")
	}
}

func TestDecodeRedisStringValue_WithRawString(t *testing.T) {
	got := decodeRedisStringValue("123456")
	if got != "123456" {
		t.Fatalf("decodeRedisStringValue() = %q, want %q", got, "123456")
	}
}

func TestDecodeRedisStringValue_WithInvalidJSON(t *testing.T) {
	raw := "{invalid-json"
	got := decodeRedisStringValue(raw)
	if got != raw {
		t.Fatalf("decodeRedisStringValue() = %q, want %q", got, raw)
	}
}
