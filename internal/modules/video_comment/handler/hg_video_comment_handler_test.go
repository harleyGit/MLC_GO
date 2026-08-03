package VideoCommentHandlerPackage

import (
	VideoCommentServicePackage "MLC_GO/internal/modules/video_comment/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteRootWithRepliesReturnsConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/video_comments/delete", nil)
	hgWriteError(recorder, request, VideoCommentServicePackage.ErrCommentHasReplies)

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "存在回复的评论不能删除") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
