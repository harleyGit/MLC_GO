package transport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	Config "MLC_GO/internal/pkg/config"
)

// TestHGIMDebugS3PrivateBoundary 仅显式启用时写入本机隔离桶；最小权限身份无删除权，
// 因此留下两个各一字节的随机探针对象，不能将该测试用于共享或生产环境。
func TestHGIMDebugS3PrivateBoundary(t *testing.T) {
	if os.Getenv("MLC_IM_S3_BOUNDARY_INTEGRATION") != "1" {
		t.Skip("需显式启用本机 debug 私有存储实测")
	}
	if os.Getenv("SERVER_ENV") != "debug" {
		t.Fatal("仅允许在 debug 环境写入隔离存储")
	}
	configDir, err := filepath.Abs("../../../../config")
	if err != nil {
		t.Fatal("无法定位 debug 配置")
	}
	t.Setenv("MLC_CONFIG_DIR", configDir)
	if err := Config.InitRuntimeEnv(); err != nil {
		t.Fatal("无法加载本机 debug 配置")
	}
	endpoint, err := hgSDKDebugEndpoint("MLC_IM_S3_ENDPOINT", "8333")
	if err != nil {
		t.Fatal(err)
	}
	region, bucket := os.Getenv("MLC_IM_S3_REGION"), os.Getenv("MLC_IM_S3_BUCKET")
	upload, err := HGNewSeaweedFS(endpoint, region, bucket, os.Getenv("MLC_IM_S3_ACCESS_KEY"), os.Getenv("MLC_IM_S3_SECRET_KEY"))
	if err != nil {
		t.Fatal("上传身份未就绪")
	}
	defer upload.HGClose()
	review, err := HGNewSeaweedFS(endpoint, region, bucket, os.Getenv("MLC_IM_REVIEW_S3_ACCESS_KEY"), os.Getenv("MLC_IM_REVIEW_S3_SECRET_KEY"))
	if err != nil {
		t.Fatal("审核身份未就绪")
	}
	defer review.HGClose()

	id := hgSDKProbeID(t)
	quarantine, sealed := "quarantine/"+id, "sealed/"+id
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	link, err := upload.HGUploadURL(quarantine, "text/plain", 1)
	if err != nil {
		t.Fatal(err)
	}
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, link, bytes.NewReader([]byte("a")))
	if err != nil {
		t.Fatal(err)
	}
	put.ContentLength = 1
	put.Header.Set("Content-Type", "text/plain")
	resp, err := upload.client.Do(put)
	if err != nil {
		t.Fatal("私有上传失败")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("私有上传响应错误: HTTP %d", resp.StatusCode)
	}
	open, err := review.HGReviewOpenQuarantine(ctx, quarantine, 1)
	if err != nil {
		t.Fatal("审核身份无法读取隔离区")
	}
	body, readErr := io.ReadAll(open)
	open.Close()
	if readErr != nil || !bytes.Equal(body, []byte("a")) {
		t.Fatal("隔离区读回内容不一致")
	}
	created, err := review.HGReviewSeal(ctx, sealed, "text/plain", 1, bytes.NewReader([]byte("a")))
	if err != nil || !created {
		t.Fatalf("首次封存失败: %v", err)
	}
	created, err = review.HGReviewSeal(ctx, sealed, "text/plain", 1, bytes.NewReader([]byte("b")))
	if err != nil || created {
		t.Fatal("封存对象不得被覆盖")
	}
	read, err := upload.HGOpen(ctx, sealed, "")
	if err != nil {
		t.Fatal("上传身份无法读取已封存对象")
	}
	sealedBody, readErr := io.ReadAll(read.Body)
	read.Body.Close()
	if readErr != nil || !bytes.Equal(sealedBody, []byte("a")) {
		t.Fatal("封存字节被改写")
	}
	injected, err := http.NewRequestWithContext(ctx, http.MethodPut, link, bytes.NewReader([]byte("b")))
	if err != nil {
		t.Fatal(err)
	}
	injected.ContentLength = 1
	injected.Header.Set("Content-Type", "text/plain")
	injected.Header.Set("x-amz-acl", "public-read")
	resp, err = upload.client.Do(injected)
	if err != nil {
		t.Fatal("验证公开 ACL 注入失败")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("未签名的公开 ACL 必须被拒绝: HTTP %d", resp.StatusCode)
	}
	anonymous, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+bucket+"/"+sealed, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = upload.client.Do(anonymous)
	if err != nil {
		t.Fatal("验证匿名访问失败")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("封存对象不得被匿名访问: HTTP %d", resp.StatusCode)
	}
}
