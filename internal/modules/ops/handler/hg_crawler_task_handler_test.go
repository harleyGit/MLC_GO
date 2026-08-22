package OpsHandlerPackage

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerLeasePackage "MLC_GO/internal/modules/crawler/lease"
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type hgCrawlerTaskHandlerRepositoryStub struct{}

func (hgCrawlerTaskHandlerRepositoryStub) SaveTaskDefinition(context.Context, *CrawlerModelPackage.HGTaskDefinition) error {
	return nil
}
func (hgCrawlerTaskHandlerRepositoryStub) GetTaskDefinitionByID(context.Context, uint64) (*CrawlerModelPackage.HGTaskDefinition, error) {
	return nil, nil
}
func (hgCrawlerTaskHandlerRepositoryStub) ListTaskDefinitions(context.Context, uint64, int) ([]CrawlerModelPackage.HGTaskDefinition, uint64, bool, error) {
	return nil, 0, false, nil
}
func (hgCrawlerTaskHandlerRepositoryStub) ListTaskExternalContents(context.Context, uint64, uint64, int) ([]CrawlerModelPackage.HGTaskExternalContent, uint64, bool, error) {
	return nil, 0, false, nil
}
func (hgCrawlerTaskHandlerRepositoryStub) CreateTaskRun(context.Context, *CrawlerModelPackage.HGTaskRun) error {
	return nil
}
func (hgCrawlerTaskHandlerRepositoryStub) CompleteTaskRun(context.Context, *CrawlerModelPackage.HGTaskRun) error {
	return nil
}
func (hgCrawlerTaskHandlerRepositoryStub) UpsertRecommendations(context.Context, []CrawlerPlatformPackage.HGRecommendation) error {
	return nil
}
func (hgCrawlerTaskHandlerRepositoryStub) UpsertTaskRecommendations(context.Context, uint64, uint64, []CrawlerPlatformPackage.HGRecommendation) error {
	return nil
}

type hgCrawlerTaskHandlerHTTPStub struct{}

func (hgCrawlerTaskHandlerHTTPStub) ValidateRequest(CrawlerDtoPackage.HGDebugRequest) error {
	return nil
}
func (hgCrawlerTaskHandlerHTTPStub) Execute(context.Context, CrawlerDtoPackage.HGDebugRequest) (CrawlerServicePackage.HGHTTPResult, error) {
	return CrawlerServicePackage.HGHTTPResult{}, nil
}

type hgCrawlerTaskHandlerLeaseStub struct{}

func (hgCrawlerTaskHandlerLeaseStub) Acquire(context.Context, uint64, time.Duration) (string, bool, error) {
	return "owner", true, nil
}
func (hgCrawlerTaskHandlerLeaseStub) Release(context.Context, uint64, string) error { return nil }

type hgCrawlerTaskHandlerAuthorizerStub struct{}

func (hgCrawlerTaskHandlerAuthorizerStub) HasAssetPermission(context.Context, string, string) (bool, error) {
	return true, nil
}

func TestListCrawlerTaskContentsRequiresTaskID(t *testing.T) {
	repository := hgCrawlerTaskHandlerRepositoryStub{}
	service, err := CrawlerServicePackage.NewHGTaskService(repository, hgCrawlerTaskHandlerHTTPStub{}, hgCrawlerTaskHandlerLeaseStub{}, time.Second, repository)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(nil).WithCrawlerTasks(service, hgCrawlerTaskHandlerAuthorizerStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/crawler/tasks/contents", nil)
	claims := &UserServicePackage.HGClaims{UserID: "admin"}
	req = req.WithContext(context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, claims))
	recorder := httptest.NewRecorder()

	handler.ListCrawlerTaskContents(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

var _ CrawlerLeasePackage.HGTaskLease = hgCrawlerTaskHandlerLeaseStub{}
