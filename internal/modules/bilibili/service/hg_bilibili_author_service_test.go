package BilibiliServicePackage

import (
	"errors"
	"testing"
	"time"
)

func TestHGParseAuthorVideoCursor(t *testing.T) {
	t.Parallel()

	parsed, submissionID, err := hgParseAuthorVideoCursor("2026-08-14T12:30:45.123456789Z|sub_001")
	if err != nil {
		t.Fatalf("parse cursor: %v", err)
	}
	if submissionID != "sub_001" {
		t.Fatalf("submission id = %q", submissionID)
	}
	want := time.Date(2026, 8, 14, 12, 30, 45, 123456789, time.UTC)
	if parsed == nil || !parsed.Equal(want) {
		t.Fatalf("parsed time = %v, want %v", parsed, want)
	}
}

func TestHGParseAuthorVideoCursorRejectsMalformedValue(t *testing.T) {
	t.Parallel()

	_, _, err := hgParseAuthorVideoCursor("not-a-cursor")
	if !errors.Is(err, ErrInvalidAuthorRequest) {
		t.Fatalf("err = %v, want ErrInvalidAuthorRequest", err)
	}
}

func TestHGNormalizePageSize(t *testing.T) {
	t.Parallel()

	if got := hgNormalizePageSize(0); got != hgDefaultAuthorVideoPageSize {
		t.Fatalf("default page size = %d", got)
	}
	if got := hgNormalizePageSize(1000); got != hgMaxAuthorVideoPageSize {
		t.Fatalf("max page size = %d", got)
	}
}
