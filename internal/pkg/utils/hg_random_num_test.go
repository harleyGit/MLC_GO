package UtilsPackage

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var roleIDPattern = regexp.MustCompile(`^ROL_[0-9A-HJKMNP-TV-Z]{26}$`)
var userIDPattern = regexp.MustCompile(`^HGUSR_[A-Z0-9]{2,8}_[0-9A-HJKMNP-TV-Z]{26}$`)
var userIDTaiwanPattern = regexp.MustCompile(`^HGUSR_TW_[0-9A-HJKMNP-TV-Z]{26}$`)
var userIDHongKongPattern = regexp.MustCompile(`^HGUSR_HK_[0-9A-HJKMNP-TV-Z]{26}$`)

func TestGenerateUserIDUsesRecommendedRegionULIDFormat(t *testing.T) {
	userID := GenerateUserID()

	if !strings.HasPrefix(userID, "HGUSR_") {
		t.Fatalf("userID = %q, want HGUSR_ prefix", userID)
	}
	if !userIDPattern.MatchString(userID) {
		t.Fatalf("userID = %q, want HGUSR_<region>_ plus 26-char ULID", userID)
	}
}

func TestGenerateUserIDUsesEnvironmentRegion(t *testing.T) {
	t.Setenv("HG_REGION", "hk")
	t.Setenv("REGION", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("TZ", "")

	userID := GenerateUserID()

	if !userIDHongKongPattern.MatchString(userID) {
		t.Fatalf("userID = %q, want HGUSR_HK_ plus 26-char ULID", userID)
	}
}

func TestCurrentRegionFallsBackToTimezone(t *testing.T) {
	t.Setenv("HG_REGION", "")
	t.Setenv("REGION", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("TZ", "Asia/Taipei")

	if got := CurrentRegion(); got != "TW" {
		t.Fatalf("CurrentRegion() = %q, want TW", got)
	}
}

func TestRegionFromTimezoneMapsKnownLocations(t *testing.T) {
	cases := map[string]string{
		"Asia/Taipei":      "TW",
		"Asia/Hong_Kong":   "HK",
		"Asia/Shanghai":    "CN",
		"Asia/Tokyo":       "JP",
		"America/New_York": "US",
		"":                 "TW",
	}
	for zone, want := range cases {
		t.Run(zone, func(t *testing.T) {
			if got := regionFromTimezone(zone); got != want {
				t.Fatalf("regionFromTimezone(%q) = %q, want %q", zone, got, want)
			}
		})
	}
}

func TestCurrentRegionMapsCloudRegion(t *testing.T) {
	t.Setenv("HG_REGION", "")
	t.Setenv("REGION", "")
	t.Setenv("AWS_REGION", "ap-east-1")
	t.Setenv("TZ", "")

	if got := CurrentRegion(); got != "HK" {
		t.Fatalf("CurrentRegion() = %q, want HK", got)
	}
}

func TestCurrentRegionUsesTimeLocalWhenEnvIsEmpty(t *testing.T) {
	t.Setenv("HG_REGION", "")
	t.Setenv("REGION", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("TZ", "")
	oldLocal := time.Local
	t.Cleanup(func() { time.Local = oldLocal })
	time.Local = time.FixedZone("Asia/Taipei", 8*60*60)

	if got := CurrentRegion(); got != "TW" {
		t.Fatalf("CurrentRegion() = %q, want TW", got)
	}
}

func TestGenerateUserIDWithRegionNormalizesRegion(t *testing.T) {
	userID := GenerateUserIDWithRegion(" hk ")

	if !userIDHongKongPattern.MatchString(userID) {
		t.Fatalf("userID = %q, want HGUSR_HK_ plus 26-char ULID", userID)
	}
}

func TestGenerateUserIDDoesNotRepeat(t *testing.T) {
	t.Setenv("HG_REGION", "tw")

	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		userID := GenerateUserID()
		if _, ok := seen[userID]; ok {
			t.Fatalf("duplicate userID generated: %s", userID)
		}
		seen[userID] = struct{}{}
	}
}

func TestGenerateRoleIDUsesRecommendedULIDFormat(t *testing.T) {
	roleID := GenerateRoleID()

	if len(roleID) != 30 {
		t.Fatalf("len(roleID) = %d, want 30", len(roleID))
	}
	if !strings.HasPrefix(roleID, "ROL_") {
		t.Fatalf("roleID = %q, want ROL_ prefix", roleID)
	}
	if !roleIDPattern.MatchString(roleID) {
		t.Fatalf("roleID = %q, want ROL_ plus 26-char ULID", roleID)
	}
}

func TestGenerateRoleIDDoesNotRepeat(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		roleID := GenerateRoleID()
		if _, ok := seen[roleID]; ok {
			t.Fatalf("duplicate roleID generated: %s", roleID)
		}
		seen[roleID] = struct{}{}
	}
}

func TestGenerateBilibiliTagIDUsesRecommendedULIDFormat(t *testing.T) {
	tagID := GenerateBilibiliTagID()
	if !strings.HasPrefix(tagID, "BLTAG_") {
		t.Fatalf("tagID = %q, want BLTAG_ prefix", tagID)
	}
	if len(tagID) != len("BLTAG_")+26 {
		t.Fatalf("tagID = %q, want BLTAG_ plus 26-char ULID", tagID)
	}
}

func TestGenerateBusinessIDNormalizesPrefix(t *testing.T) {
	id := GenerateBusinessID("rol")

	if !roleIDPattern.MatchString(id) {
		t.Fatalf("id = %q, want normalized ROL_ plus 26-char ULID", id)
	}
}
