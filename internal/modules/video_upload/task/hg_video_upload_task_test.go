package VideoUploadTaskPackage

import (
	"context"
	"testing"
)

func TestMemoryPublisherPublishAndClose(t *testing.T) {
	publisher := NewMemoryPublisher()
	defer publisher.Close()

	err := publisher.Publish(context.Background(), Task{
		Type:         TaskTypeAudit,
		UserID:       "user_1",
		SubmissionID: "submission_1",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}
