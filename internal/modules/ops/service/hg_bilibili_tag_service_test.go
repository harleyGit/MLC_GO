package OpsServicePackage

import "testing"

func TestNormalizeBilibiliTagName(t *testing.T) {
	got, err := normalizeBilibiliTagName("  MMD·3D  ")
	if err != nil {
		t.Fatalf("normalizeBilibiliTagName() error = %v", err)
	}
	if got != "MMD·3D" {
		t.Fatalf("normalizeBilibiliTagName() = %q, want %q", got, "MMD·3D")
	}
}

func TestNormalizeBilibiliTagNameRejectsReservedRecommendation(t *testing.T) {
	if _, err := normalizeBilibiliTagName("推荐"); err == nil {
		t.Fatal("normalizeBilibiliTagName() error = nil, want reserved-name error")
	}
}

func TestNormalizeBilibiliTagStatus(t *testing.T) {
	if got, err := normalizeBilibiliTagStatus(0); err != nil || got != 1 {
		t.Fatalf("normalizeBilibiliTagStatus(0) = (%d, %v), want (1, nil)", got, err)
	}
	if _, err := normalizeBilibiliTagStatus(3); err == nil {
		t.Fatal("normalizeBilibiliTagStatus(3) error = nil, want validation error")
	}
}

func TestNormalizeBilibiliTagID(t *testing.T) {
	got, err := normalizeBilibiliTagID("  BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P  ")
	if err != nil {
		t.Fatalf("normalizeBilibiliTagID() error = %v", err)
	}
	if got != "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P" {
		t.Fatalf("normalizeBilibiliTagID() = %q", got)
	}
	if _, err := normalizeBilibiliTagID("101"); err == nil {
		t.Fatal("normalizeBilibiliTagID(101) error = nil, want invalid tagId error")
	}
}
