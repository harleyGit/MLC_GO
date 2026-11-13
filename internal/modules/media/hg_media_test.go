package media

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestHGPolicyRequiresPurposeAndRestrictsType(t *testing.T) {
	valid := HGUploadRequest{Purpose: "user_avatar", MIME: "image/jpeg", Size: 128, SHA256: strings.Repeat("a", 64), IdempotencyKey: "attempt_1"}
	policy, ext, err := hgValidateRequest(valid)
	if err != nil || policy.biz != "user" || ext != "jpg" {
		t.Fatalf("有效头像用途未通过：%v", err)
	}
	for name, change := range map[string]func(*HGUploadRequest){
		"缺少用途":        func(r *HGUploadRequest) { r.Purpose = "" },
		"伪造用途":        func(r *HGUploadRequest) { r.Purpose = "admin" },
		"跨类型":         func(r *HGUploadRequest) { r.MIME = "video/mp4" },
		"超出大小":        func(r *HGUploadRequest) { r.Size = 11 << 20 },
		"错误摘要":        func(r *HGUploadRequest) { r.SHA256 = strings.Repeat("Z", 64) },
		"注入重试键":       func(r *HGUploadRequest) { r.IdempotencyKey = "../../etc" },
		"审核器不支持的 GIF": func(r *HGUploadRequest) { r.Purpose = "comment_image"; r.MIME = "image/gif" },
		"审核器不支持的字幕":   func(r *HGUploadRequest) { r.Purpose = "video_subtitle"; r.MIME = "text/vtt" },
	} {
		t.Run(name, func(t *testing.T) {
			r := valid
			change(&r)
			if _, _, err := hgValidateRequest(r); !errors.Is(err, HGErrInvalid) {
				t.Fatalf("应拒绝不合法请求：%v", err)
			}
		})
	}
}

func TestHGSignUploadDoesNotAllowUnlimitedBody(t *testing.T) {
	store, err := HGNewSeaweedStore("https://upload.example.test", "us-east-1", "mlc-temp", "test-key", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	link, err := store.HGSignUpload(context.Background(), "quarantine/"+strings.ReplaceAll(uuid.NewString(), "-", ""), "image/jpeg", 123)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Query().Get("X-Amz-SignedHeaders"), "content-length") || u.Host != "upload.example.test" || !strings.Contains(u.Path, "mlc-temp/quarantine/") {
		t.Fatalf("预签名未绑定对象长度或目标：%s", u.Path)
	}
}

func TestHGStoreRejectsUntrustedEndpoint(t *testing.T) {
	for _, endpoint := range []string{"https://a.example/?token=secret", "file:///tmp/storage", "https://user:pass@example.org", "http://example.org/prefix"} {
		if _, err := HGNewSeaweedStore(endpoint, "us-east-1", "mlc-temp", "a", "b"); err == nil {
			t.Fatalf("应拒绝不安全端点: %s", endpoint)
		}
	}
}
