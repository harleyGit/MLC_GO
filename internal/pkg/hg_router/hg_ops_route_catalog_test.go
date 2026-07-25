package HGRouterPackage

import (
	"net/http"
	"testing"
)

func TestOpsRouteCatalogContainsBilibiliTagCRUD(t *testing.T) {
	want := map[string]string{
		"/api/v1/ops/bilibili/tags":        http.MethodPost,
		"/api/v1/ops/bilibili/tags/list":   http.MethodGet,
		"/api/v1/ops/bilibili/tags/update": http.MethodPost,
		"/api/v1/ops/bilibili/tags/delete": http.MethodPost,
	}

	for _, item := range OpsRouteCatalog() {
		if method, ok := want[item.Path]; ok && item.Method == method && item.NeedAuth {
			delete(want, item.Path)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing authenticated bilibili tag routes: %v", want)
	}
}
