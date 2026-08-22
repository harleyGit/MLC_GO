package PersistenceRedisPackage

import "testing"

func TestGetCrawlerTaskLeaseKeyIsPerTaskAndClusterStable(t *testing.T) {
	if got := GetCrawlerTaskLeaseKey(42); got != "crawler:tasks:lease:{42}" {
		t.Fatalf("GetCrawlerTaskLeaseKey(42) = %q", got)
	}
	if GetCrawlerTaskLeaseKey(42) == GetCrawlerTaskLeaseKey(43) {
		t.Fatal("crawler task lease keys collided")
	}
}
