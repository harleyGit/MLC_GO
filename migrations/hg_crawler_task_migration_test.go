package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestCrawlerTaskMigrationHasJSONOptimisticVersionAndCursorIndexes(t *testing.T) {
	content, err := os.ReadFile("000029_create_crawler_tasks.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(content)
	for _, required := range []string{
		"CREATE TABLE `crawler_task_definitions`",
		"CREATE TABLE `crawler_task_runs`",
		"`configuration` JSON NOT NULL",
		"`version` BIGINT UNSIGNED NOT NULL DEFAULT 1",
		"idx_crawler_task_definitions_enabled_id",
		"(`enabled`, `id`)",
		"idx_crawler_task_runs_definition_id",
		"(`task_definition_id`, `id`)",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	upper := strings.ToUpper(sqlText)
	if strings.Contains(upper, "FOREIGN KEY (`") || strings.Contains(upper, "\nUSE ") {
		t.Fatal("crawler task migration must not contain foreign keys or select a database")
	}
	down, err := os.ReadFile("000029_create_crawler_tasks.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `crawler_task_runs`;") ||
		!strings.Contains(string(down), "DROP TABLE IF EXISTS `crawler_task_definitions`;") {
		t.Fatal("down migration must drop runs before definitions")
	}
}
