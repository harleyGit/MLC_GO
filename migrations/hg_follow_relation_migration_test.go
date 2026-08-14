package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestFollowRelationBusinessIDMigrationUsesCaseSensitiveChar(t *testing.T) {
	content, err := os.ReadFile("000025_add_follow_relation_business_id.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)
	for _, required := range []string{
		"`relation_id` CHAR(12)",
		"CHARACTER SET ascii COLLATE ascii_bin NULL",
		"UNIQUE KEY `uk_follow_relation_id` (`relation_id`)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}
