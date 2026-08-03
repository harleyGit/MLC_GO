package HGMiddlewarePackage

import (
	"net/http/httptest"
	"testing"
)

func TestAPIGuardSignsCommentImageRouteAsEmptyBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/video_comments/image?ext=png", nil)
	req.Header.Set("Content-Type", "application/x-custom-image")
	if !hgIsBinarySignedBody(req) {
		t.Fatal("comment image route must use the API Guard empty-body signature convention")
	}
}

func TestAPIGuardDoesNotGloballyExemptImageContentTypes(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/profile/update", nil)
	req.Header.Set("Content-Type", "image/png")
	if hgIsBinarySignedBody(req) {
		t.Fatal("generic image content type must not bypass body signing outside the comment image route")
	}
}

func TestAPIGuardRetainsExistingBinaryContentTypeExemptions(t *testing.T) {
	for _, contentType := range []string{"multipart/form-data; boundary=test", "application/octet-stream"} {
		req := httptest.NewRequest("POST", "/api/v1/other", nil)
		req.Header.Set("Content-Type", contentType)
		if !hgIsBinarySignedBody(req) {
			t.Fatalf("content type %q must retain empty-body signing", contentType)
		}
	}
}
