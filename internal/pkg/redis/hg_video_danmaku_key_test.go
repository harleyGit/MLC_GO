package PersistenceRedisPackage

import (
	"strings"
	"testing"
)

func TestVideoDanmakuRoomKeysShareVideoHashTag(t *testing.T) {
	videoID := "video-123"
	for _, key := range []string{
		GetVideoDanmakuBroadcastChannel(videoID),
		GetVideoDanmakuRecentStreamKey(videoID),
		GetVideoDanmakuRecentOffsetKey(videoID),
	} {
		if key == "" || !strings.Contains(key, "{video-123}") {
			t.Fatalf("key %q does not contain video hash tag", key)
		}
	}
}
