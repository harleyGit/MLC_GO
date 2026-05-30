package UserServicePackage

import "testing"

func TestDecodeRedisStringValue_WithJSONString(t *testing.T) {
	got := DecodeRedisStringValue(`"123456"`)
	if got != "123456" {
		t.Fatalf("DecodeRedisStringValue() = %q, want %q", got, "123456")
	}
}

func TestDecodeRedisStringValue_WithRawString(t *testing.T) {
	got := DecodeRedisStringValue("123456")
	if got != "123456" {
		t.Fatalf("DecodeRedisStringValue() = %q, want %q", got, "123456")
	}
}

func TestDecodeRedisStringValue_WithInvalidJSON(t *testing.T) {
	raw := "{invalid-json"
	got := DecodeRedisStringValue(raw)
	if got != raw {
		t.Fatalf("DecodeRedisStringValue() = %q, want %q", got, raw)
	}
}
