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

func TestAPIGuardSignsMallImageRouteAsEmptyBody(t *testing.T) {
	for _, path := range []string{"/products/image?ext=png", "/api/v1/mall/products/image?ext=png"} {
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set("Content-Type", "image/png")
		if !hgIsBinarySignedBody(req) {
			t.Fatalf("商品图片 %q 必须按空 body 签名，与前端二进制上传一致", path)
		}
	}
}

func TestAPIGuardDoesNotGloballyExemptImageContentTypes(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/profile/update", nil)
	req.Header.Set("Content-Type", "image/png")
	if hgIsBinarySignedBody(req) {
		t.Fatal("普通 image/* 请求不得绕过请求体验签")
	}
	mallJSON := httptest.NewRequest("POST", "/api/v1/mall/products/publish", nil)
	mallJSON.Header.Set("Content-Type", "image/png")
	if hgIsBinarySignedBody(mallJSON) {
		t.Fatal("商城发布接口不得因 image/png 头部绕过请求体验签")
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
