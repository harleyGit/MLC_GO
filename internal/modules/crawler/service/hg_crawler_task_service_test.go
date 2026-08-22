package service

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type hgTaskRepositoryStub struct {
	definition *CrawlerModelPackage.HGTaskDefinition
	runs       []*CrawlerModelPackage.HGTaskRun
	items      []CrawlerPlatformPackage.HGRecommendation
	taskID     uint64
	runID      uint64
	contents   []CrawlerModelPackage.HGTaskExternalContent
}

func (s *hgTaskRepositoryStub) SaveTaskDefinition(_ context.Context, definition *CrawlerModelPackage.HGTaskDefinition) error {
	if definition.ID == 0 {
		definition.ID = 9
		definition.Version = 1
	}
	copy := *definition
	copy.Configuration = append(json.RawMessage(nil), definition.Configuration...)
	s.definition = &copy
	return nil
}

func (s *hgTaskRepositoryStub) GetTaskDefinitionByID(context.Context, uint64) (*CrawlerModelPackage.HGTaskDefinition, error) {
	if s.definition == nil {
		return nil, errors.New("not found")
	}
	copy := *s.definition
	return &copy, nil
}

func (s *hgTaskRepositoryStub) ListTaskDefinitions(context.Context, uint64, int) ([]CrawlerModelPackage.HGTaskDefinition, uint64, bool, error) {
	return nil, 0, false, nil
}

func (s *hgTaskRepositoryStub) ListTaskExternalContents(context.Context, uint64, uint64, int) ([]CrawlerModelPackage.HGTaskExternalContent, uint64, bool, error) {
	return s.contents, 7, true, nil
}

func (s *hgTaskRepositoryStub) CreateTaskRun(_ context.Context, run *CrawlerModelPackage.HGTaskRun) error {
	run.ID = uint64(len(s.runs) + 1)
	copy := *run
	s.runs = append(s.runs, &copy)
	return nil
}

func (s *hgTaskRepositoryStub) CompleteTaskRun(_ context.Context, run *CrawlerModelPackage.HGTaskRun) error {
	copy := *run
	s.runs[len(s.runs)-1] = &copy
	return nil
}

func (s *hgTaskRepositoryStub) UpsertRecommendations(_ context.Context, items []CrawlerPlatformPackage.HGRecommendation) error {
	s.items = append([]CrawlerPlatformPackage.HGRecommendation(nil), items...)
	return nil
}

func (s *hgTaskRepositoryStub) UpsertTaskRecommendations(_ context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) error {
	s.taskID, s.runID = taskID, runID
	s.items = append([]CrawlerPlatformPackage.HGRecommendation(nil), items...)
	return nil
}

type hgTaskHTTPStub struct {
	validationErr error
	result        HGHTTPResult
	results       []HGHTTPResult
	err           error
	executed      int
}

func (s hgTaskHTTPStub) ValidateRequest(CrawlerDtoPackage.HGDebugRequest) error {
	return s.validationErr
}
func (s *hgTaskHTTPStub) Execute(context.Context, CrawlerDtoPackage.HGDebugRequest) (HGHTTPResult, error) {
	if s.err != nil {
		return HGHTTPResult{}, s.err
	}
	if s.executed < len(s.results) {
		result := s.results[s.executed]
		s.executed++
		return result, nil
	}
	s.executed++
	return s.result, nil
}

type hgTaskLeaseStub struct {
	acquired bool
	ttl      time.Duration
}

func (s *hgTaskLeaseStub) Acquire(_ context.Context, _ uint64, ttl time.Duration) (string, bool, error) {
	s.ttl = ttl
	return "owner", s.acquired, nil
}

func (*hgTaskLeaseStub) Release(context.Context, uint64, string) error { return nil }

func TestHGTaskServiceRejectsInvalidCronAndRequestPolicy(t *testing.T) {
	repository := &hgTaskRepositoryStub{}
	lease := &hgTaskLeaseStub{acquired: true}
	http := &hgTaskHTTPStub{validationErr: ErrHGCrawlerUnsafeTarget}
	service, err := NewHGTaskService(repository, http, lease, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	req := hgValidTaskSaveRequest()
	req.Cron = "invalid"
	if _, err := service.Save(context.Background(), req, "admin"); !errors.Is(err, ErrHGTaskInvalidDefinition) {
		t.Fatalf("invalid cron error = %v", err)
	}
	req.Cron = "0 * * * * *"
	if _, err := service.Save(context.Background(), req, "admin"); !errors.Is(err, ErrHGTaskInvalidDefinition) {
		t.Fatalf("unsafe request error = %v", err)
	}
}

func TestHGTaskServiceSaveAndRunCompletesFailureWithoutRemovingDefinition(t *testing.T) {
	repository := &hgTaskRepositoryStub{}
	lease := &hgTaskLeaseStub{acquired: true}
	http := &hgTaskHTTPStub{err: errors.New("upstream unavailable")}
	service, err := NewHGTaskService(repository, http, lease, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	definition, run, err := service.SaveAndRun(context.Background(), hgValidTaskSaveRequest(), "admin")
	if err == nil || definition == nil || definition.ID != 9 || repository.definition == nil {
		t.Fatalf("SaveAndRun() definition=%#v run=%#v error=%v", definition, run, err)
	}
	if run == nil || run.Status != "failed" || len(repository.runs) != 1 || repository.runs[0].Status != "failed" {
		t.Fatalf("completed run = %#v, stored runs = %#v", run, repository.runs)
	}
	if lease.ttl != 15*time.Second {
		t.Fatalf("lease TTL = %v, want 15s", lease.ttl)
	}
}

func TestHGTaskServiceRunParsesLimitsAndUpserts(t *testing.T) {
	repository := &hgTaskRepositoryStub{}
	lease := &hgTaskLeaseStub{acquired: true}
	httpResult := HGHTTPResult{URL: "https://api.example.com/feed", StatusCode: 200, Body: []byte(`{"items":[{"id":"one","title":"First","url":"/watch/one"}]}`)}
	http := &hgTaskHTTPStub{result: httpResult}
	service, err := NewHGTaskService(repository, http, lease, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := service.Save(context.Background(), hgValidTaskSaveRequest(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.RunByID(context.Background(), definition.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "succeeded" || run.ItemCount != 1 || len(repository.items) != 1 || repository.items[0].ContentID != "one" {
		t.Fatalf("run=%#v items=%#v", run, repository.items)
	}
	if repository.taskID != definition.ID || repository.runID != run.ID {
		t.Fatalf("association identity task=%d run=%d", repository.taskID, repository.runID)
	}
}

func TestHGTaskServiceListContentsRequiresTaskIDAndReturnsCursorPage(t *testing.T) {
	repository := &hgTaskRepositoryStub{contents: []CrawlerModelPackage.HGTaskExternalContent{{AssociationID: 8, TaskDefinitionID: 9}}}
	service, err := NewHGTaskService(repository, &hgTaskHTTPStub{}, &hgTaskLeaseStub{acquired: true}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListContents(context.Background(), CrawlerDtoPackage.HGTaskExternalContentListRequest{}); !errors.Is(err, ErrHGTaskInvalidDefinition) {
		t.Fatalf("missing task id error = %v", err)
	}
	result, err := service.ListContents(context.Background(), CrawlerDtoPackage.HGTaskExternalContentListRequest{TaskID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.NextCursor != 7 || !result.HasMore {
		t.Fatalf("result = %#v", result)
	}
}

func TestHGTaskServiceRunsBoundedURLList(t *testing.T) {
	repository := &hgTaskRepositoryStub{}
	http := &hgTaskHTTPStub{results: []HGHTTPResult{
		{URL: "https://api.example.com/one", StatusCode: 200, Body: []byte(`{"items":[{"id":"one","title":"One","url":"/one"}]}`)},
		{URL: "https://api.example.com/two", StatusCode: 200, Body: []byte(`{"items":[{"id":"two","title":"Two","url":"/two"}]}`)},
	}}
	service, err := NewHGTaskService(repository, http, &hgTaskLeaseStub{acquired: true}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	req := hgValidTaskSaveRequest()
	req.Configuration = json.RawMessage(`{
		"request":{"method":"GET","timeoutMs":10000},
		"collectType":"url_list",
		"urls":["https://api.example.com/one","https://api.example.com/two"],
		"parser":{"type":"restricted_jsonpath","platform":"example","itemSelector":"$.items[*]","fields":{"contentId":{"selector":"$.id"},"title":{"selector":"$.title"},"targetUrl":{"selector":"$.url"}}}
	}`)
	definition, err := service.Save(context.Background(), req, "admin")
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.RunByID(context.Background(), definition.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.ItemCount != 2 || len(repository.items) != 2 || http.executed != 2 {
		t.Fatalf("run=%#v items=%#v requests=%d", run, repository.items, http.executed)
	}
}

func TestHGTaskServiceExpandsSitemapURLs(t *testing.T) {
	repository := &hgTaskRepositoryStub{}
	http := &hgTaskHTTPStub{results: []HGHTTPResult{
		{URL: "https://api.example.com/sitemap.xml", StatusCode: 200, Body: []byte(`<urlset><url><loc>https://api.example.com/one</loc></url></urlset>`)},
		{URL: "https://api.example.com/one", StatusCode: 200, Body: []byte(`{"items":[{"id":"one","title":"One","url":"/one"}]}`)},
	}}
	service, err := NewHGTaskService(repository, http, &hgTaskLeaseStub{acquired: true}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	req := hgValidTaskSaveRequest()
	req.Configuration = json.RawMessage(`{
		"request":{"url":"https://api.example.com/sitemap.xml","method":"GET","timeoutMs":10000},
		"collectType":"sitemap",
		"parser":{"type":"restricted_jsonpath","platform":"example","itemSelector":"$.items[*]","fields":{"contentId":{"selector":"$.id"},"title":{"selector":"$.title"},"targetUrl":{"selector":"$.url"}}}
	}`)
	definition, err := service.Save(context.Background(), req, "admin")
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.RunByID(context.Background(), definition.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.ItemCount != 1 || len(repository.items) != 1 || http.executed != 2 {
		t.Fatalf("run=%#v items=%#v requests=%d", run, repository.items, http.executed)
	}
}

func hgValidTaskSaveRequest() CrawlerDtoPackage.HGTaskDefinitionSaveRequest {
	return CrawlerDtoPackage.HGTaskDefinitionSaveRequest{
		Name: "feed", Platform: "example", Enabled: true, Cron: "0 * * * * *",
		ParserType: "restricted_jsonpath", ItemPath: "$.items[*]", MaxItems: 10,
		Configuration: json.RawMessage(`{
            "request":{"url":"https://api.example.com/feed","method":"GET","timeoutMs":10000},
            "parser":{"type":"restricted_jsonpath","platform":"example","itemSelector":"$.items[*]","fields":{"contentId":{"selector":"$.id"},"title":{"selector":"$.title"},"targetUrl":{"selector":"$.url"}}}
        }`),
	}
}
