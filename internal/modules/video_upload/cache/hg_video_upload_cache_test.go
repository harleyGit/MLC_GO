package VideoUploadCachePackage

import "testing"

func TestVideoUploadCacheKeys(t *testing.T) {
	userID := "user_1"
	submissionID := "submission_1"

	if got := sessionKey(userID, submissionID); got != "video_upload:session:user_1:submission_1" {
		t.Fatalf("sessionKey() = %s", got)
	}
	if got := userRateKey(userID); got != "video_upload:rate:user:user_1" {
		t.Fatalf("userRateKey() = %s", got)
	}
	if got := ipRateKey("127.0.0.1"); got != "video_upload:rate:ip:127.0.0.1" {
		t.Fatalf("ipRateKey() = %s", got)
	}
	if got := submitLockKey(userID, submissionID); got != "video_upload:submit_lock:user_1:submission_1" {
		t.Fatalf("submitLockKey() = %s", got)
	}
	if got := submitResultKey(userID, submissionID); got != "video_upload:submit_result:user_1:submission_1" {
		t.Fatalf("submitResultKey() = %s", got)
	}
}
