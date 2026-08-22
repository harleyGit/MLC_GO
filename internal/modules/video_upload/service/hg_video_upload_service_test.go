package VideoUploadServicePackage

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	VideoUploadRepositoryPackage "MLC_GO/internal/modules/video_upload/repository"
	"context"
	"testing"
)

func TestVideoListCounterDelta(t *testing.T) {
	tests := []struct {
		name      string
		oldStatus string
		newStatus string
		want      int64
	}{
		{name: "draft to reviewing increments", oldStatus: "draft", newStatus: "reviewing", want: 1},
		{name: "reviewing to reviewing unchanged", oldStatus: "reviewing", newStatus: "reviewing", want: 0},
		{name: "reviewing to draft decrements", oldStatus: "reviewing", newStatus: "draft", want: -1},
		{name: "published to reviewing unchanged", oldStatus: "published", newStatus: "reviewing", want: 0},
		{name: "missing to published increments", oldStatus: "", newStatus: "published", want: 1},
		{name: "missing to draft unchanged", oldStatus: "", newStatus: "draft", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := videoListCounterDelta(tt.oldStatus, tt.newStatus); got != tt.want {
				t.Fatalf("videoListCounterDelta(%q, %q) = %d, want %d", tt.oldStatus, tt.newStatus, got, tt.want)
			}
		})
	}
}

func TestVideoListCounterUpdate(t *testing.T) {
	tests := []struct {
		name       string
		oldStatus  string
		newStatus  string
		wantStatus string
		wantDelta  int64
	}{
		{name: "enter reviewing increments reviewing", oldStatus: "draft", newStatus: "reviewing", wantStatus: "reviewing", wantDelta: 1},
		{name: "leave reviewing decrements reviewing", oldStatus: "reviewing", newStatus: "draft", wantStatus: "reviewing", wantDelta: -1},
		{name: "reviewing to reviewing unchanged", oldStatus: "reviewing", newStatus: "reviewing", wantStatus: "", wantDelta: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotDelta := videoListCounterUpdate(tt.oldStatus, tt.newStatus)
			if gotStatus != tt.wantStatus || gotDelta != tt.wantDelta {
				t.Fatalf("videoListCounterUpdate(%q, %q) = (%q, %d), want (%q, %d)", tt.oldStatus, tt.newStatus, gotStatus, gotDelta, tt.wantStatus, tt.wantDelta)
			}
		})
	}
}

func TestSubmissionEventsEmitsPublishedTransition(t *testing.T) {
	service := &Service{}
	events := service.submissionEvents(context.Background(), "user-1", VideoUploadDtoPackage.SaveSubmissionRequest{SubmissionID: "submission-1", Status: "published"})
	if len(events) != 1 || events[0].EventName() != VideoEventsPackage.VideoPublishedEventName {
		t.Fatalf("events = %#v, want one video.published event", events)
	}
}

func TestVideoStatusCounterSyncInterval(t *testing.T) {
	if videoStatusCounterSyncInterval <= 0 {
		t.Fatalf("videoStatusCounterSyncInterval must be positive")
	}
}

func TestGetVideoListReturnsCachedPageWithoutRepositoryQuery(t *testing.T) {
	cached := &VideoUploadDtoPackage.GetVideoListResponse{
		Total:    2,
		PageSize: 20,
		HasMore:  false,
		Videos: []VideoUploadDtoPackage.VideoListItem{
			{SubmissionID: "submission_cached", SubmitTime: "2026-07-04T10:00:00Z"},
		},
	}
	svc := &Service{
		repo:  &fakeVideoListRepository{},
		cache: &fakeVideoListCache{cachedPage: cached},
	}

	got, err := svc.GetVideoList(context.Background(), "", 20, "MMD·3D")
	if err != nil {
		t.Fatalf("GetVideoList() error = %v", err)
	}
	if got == nil || len(got.Videos) != 1 || got.Videos[0].SubmissionID != "submission_cached" {
		t.Fatalf("GetVideoList() = %#v, want cached response", got)
	}
	if svc.repo.(*fakeVideoListRepository).listCalls != 0 {
		t.Fatalf("repository list calls = %d, want 0", svc.repo.(*fakeVideoListRepository).listCalls)
	}
}

func TestGetVideoListTotalRefreshesLegacyCounterWithoutExternalField(t *testing.T) {
	repo := &fakeVideoListRepository{statusCounts: map[string]int64{"reviewing": 1, "published": 1, "external": 12}}
	cache := &fakeVideoListCache{statusCounters: map[string]int64{"reviewing": 1, "published": 1}}
	svc := &Service{repo: repo, cache: cache}

	total, err := svc.getVideoListTotal(context.Background())
	if err != nil {
		t.Fatalf("getVideoListTotal() error = %v", err)
	}
	if total != 14 {
		t.Fatalf("getVideoListTotal() = %d, want 14", total)
	}
	if repo.statusCountCalls != 1 || cache.setCounterCalls != 1 {
		t.Fatalf("refresh calls = repo:%d cache:%d, want 1/1", repo.statusCountCalls, cache.setCounterCalls)
	}
}

type fakeVideoListRepository struct {
	listCalls        int
	tagName          string
	statusCountCalls int
	statusCounts     map[string]int64
}

func (f *fakeVideoListRepository) EnsureVideoListIndex(ctx context.Context) error { return nil }
func (f *fakeVideoListRepository) CreateUploadedVideo(ctx context.Context, video VideoUploadRepositoryPackage.UploadedVideo) error {
	return nil
}
func (f *fakeVideoListRepository) SaveSubmission(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) error {
	return nil
}
func (f *fakeVideoListRepository) SaveSubmissionWithEvents(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest, domainEvents ...events.DomainEvent) error {
	return nil
}
func (f *fakeVideoListRepository) GetSubmissionStatus(ctx context.Context, submissionID string, userID string) (string, bool, error) {
	return "", false, nil
}
func (f *fakeVideoListRepository) GetVideoListByCursor(ctx context.Context, cursor string, limit int, tagName string) ([]VideoUploadDtoPackage.VideoListItem, error) {
	f.listCalls++
	f.tagName = tagName
	return nil, nil
}
func (f *fakeVideoListRepository) GetVideoListItemByID(ctx context.Context, contentID string) (VideoUploadDtoPackage.VideoListItem, bool, error) {
	return VideoUploadDtoPackage.VideoListItem{}, false, nil
}
func (f *fakeVideoListRepository) GetVideoStatusCounts(ctx context.Context) (map[string]int64, error) {
	f.statusCountCalls++
	if f.statusCounts != nil {
		return f.statusCounts, nil
	}
	return map[string]int64{"reviewing": 1, "published": 1}, nil
}

type fakeVideoListCache struct {
	cachedPage      *VideoUploadDtoPackage.GetVideoListResponse
	statusCounters  map[string]int64
	setCounterCalls int
}

func (f *fakeVideoListCache) TouchUploadSession(ctx context.Context, userID string, submissionID string) error {
	return nil
}
func (f *fakeVideoListCache) SaveUploadSession(ctx context.Context, userID string, submissionID string) error {
	return nil
}
func (f *fakeVideoListCache) CheckUploadRateLimit(ctx context.Context, userID string, ip string) error {
	return nil
}
func (f *fakeVideoListCache) AcquireSubmitLock(ctx context.Context, userID string, submissionID string) (string, error) {
	return "", nil
}
func (f *fakeVideoListCache) ReleaseSubmitLock(ctx context.Context, userID string, submissionID string, lockValue string) error {
	return nil
}
func (f *fakeVideoListCache) SaveSubmitResult(ctx context.Context, userID string, submissionID string, status string) error {
	return nil
}
func (f *fakeVideoListCache) IncrementVideoStatusCounter(ctx context.Context, status string, delta int64) error {
	return nil
}
func (f *fakeVideoListCache) GetVideoStatusCounters(ctx context.Context) (map[string]int64, bool, error) {
	if f.statusCounters != nil {
		return f.statusCounters, true, nil
	}
	return map[string]int64{"reviewing": 1, "published": 1}, true, nil
}
func (f *fakeVideoListCache) SetVideoStatusCounters(ctx context.Context, counters map[string]int64) error {
	f.setCounterCalls++
	return nil
}
func (f *fakeVideoListCache) GetVideoListPage(ctx context.Context, cursor string, pageSize int, tagName string) (*VideoUploadDtoPackage.GetVideoListResponse, bool, error) {
	return f.cachedPage, f.cachedPage != nil, nil
}
func (f *fakeVideoListCache) SetVideoListPage(ctx context.Context, cursor string, pageSize int, tagName string, resp *VideoUploadDtoPackage.GetVideoListResponse) error {
	return nil
}
func (f *fakeVideoListCache) InvalidateVideoListPages(ctx context.Context) error { return nil }
