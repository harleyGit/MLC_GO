package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestHGMediaMigrationHasBoundedClaimsAndReferenceIndexes(t *testing.T) {
	for file, required := range map[string][]string{
		"000071_media_review_cleanup.up.sql": {"idx_media_review", "idx_media_delete_due", "idx_media_delete_repair", "uk_media_delete_object", "finalized_at", "lease_token"},
		"000072_media_asset_refs.up.sql":     {"uk_media_business_slot", "idx_media_ref_asset", "asset_id", "owner_id"},
	} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range required {
			if !strings.Contains(string(content), item) {
				t.Errorf("%s 缺少 %s", file, item)
			}
		}
		if strings.Contains(strings.ToUpper(string(content)), "USE ") {
			t.Errorf("%s 不得切换数据库", file)
		}
	}
}
