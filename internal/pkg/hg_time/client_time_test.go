package hg_time

import (
	"testing"
	"time"
)

func TestParseClientTime_RFC3339(t *testing.T) {
	ct := ClientTime{Value: "2026-06-01T10:00:00+08:00", Format: "rfc3339"}
	got, err := ParseClientTime(ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UTC().Hour() != 2 {
		t.Fatalf("expected UTC hour 2, got %d", got.UTC().Hour())
	}
}

func TestParseClientTime_RFC3339_UTC(t *testing.T) {
	ct := ClientTime{Value: "2026-06-01T10:00:00Z", Format: "rfc3339"}
	got, err := ParseClientTime(ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UTC().Hour() != 10 {
		t.Fatalf("expected UTC hour 10, got %d", got.UTC().Hour())
	}
}

func TestParseClientTime_DateTimeLocal(t *testing.T) {
	ct := ClientTime{Value: "2026-06-01T10:00", Format: "datetime-local", Timezone: "Asia/Shanghai"}
	got, err := ParseClientTime(ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UTC().Hour() != 2 {
		t.Fatalf("expected UTC hour 2 (Shanghai is UTC+8), got %d", got.UTC().Hour())
	}
}

func TestParseClientTime_DateTimeLocal_NoTimezone(t *testing.T) {
	ct := ClientTime{Value: "2026-06-01T10:00", Format: "datetime-local"}
	_, err := ParseClientTime(ct)
	if err == nil {
		t.Fatal("expected error for datetime-local without timezone")
	}
}

func TestParseClientTime_DateTimeLocal_InvalidTimezone(t *testing.T) {
	ct := ClientTime{Value: "2026-06-01T10:00", Format: "datetime-local", Timezone: "Invalid/Zone"}
	_, err := ParseClientTime(ct)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestParseClientTime_Unix(t *testing.T) {
	ct := ClientTime{Value: "1748745600", Format: "unix"}
	got, err := ParseClientTime(ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Unix(1748745600, 0).UTC()
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestParseClientTime_Unix_Invalid(t *testing.T) {
	ct := ClientTime{Value: "not_a_number", Format: "unix"}
	_, err := ParseClientTime(ct)
	if err == nil {
		t.Fatal("expected error for invalid unix timestamp")
	}
}

func TestParseClientTime_UnsupportedFormat(t *testing.T) {
	ct := ClientTime{Value: "2026-06-01", Format: "unknown"}
	_, err := ParseClientTime(ct)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestNormalizeBirthDate_FullDate(t *testing.T) {
	ct := ClientTime{Value: "2020-08-16", Format: "date"}
	got, err := NormalizeBirthDate(ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2020-08-16" {
		t.Fatalf("expected 2020-08-16, got %s", got)
	}
}

func TestNormalizeBirthDate_YearMonth(t *testing.T) {
	ct := ClientTime{Value: "2020-08", Format: "year-month"}
	got, err := NormalizeBirthDate(ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2020-08-01" {
		t.Fatalf("expected 2020-08-01, got %s", got)
	}
}

func TestNormalizeBirthDate_InvalidDateFormat(t *testing.T) {
	ct := ClientTime{Value: "2020/08/16", Format: "date"}
	_, err := NormalizeBirthDate(ct)
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
}

func TestNormalizeBirthDate_InvalidYearMonthFormat(t *testing.T) {
	ct := ClientTime{Value: "2020/08", Format: "year-month"}
	_, err := NormalizeBirthDate(ct)
	if err == nil {
		t.Fatal("expected error for invalid year-month format")
	}
}

func TestNormalizeBirthDate_UnsupportedFormat(t *testing.T) {
	ct := ClientTime{Value: "2020-08-16T10:00:00Z", Format: "rfc3339"}
	_, err := NormalizeBirthDate(ct)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
