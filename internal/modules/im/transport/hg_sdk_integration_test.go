package transport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	Config "MLC_GO/internal/pkg/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// hgSDKDebugEndpoint 限定已核对的本机 debug 容器端口，禁止测试误写远程服务。
func hgSDKDebugEndpoint(name, port string) (string, error) {
	raw := os.Getenv(name)
	u, err := url.Parse(raw)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.Port() != port || u.Scheme != "http" || u.Path != "" || u.RawQuery != "" || u.User != nil {
		return "", fmt.Errorf("%s 必须指向已核对的本机 debug 端口", name)
	}
	return raw, nil
}

func hgSDKProbeID(t *testing.T) string {
	t.Helper()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal("生成临时测试标识失败")
	}
	return hex.EncodeToString(id[:])
}

// TestHGSDKDebugProtocols 只在显式 opt-in 时访问本机 debug 外部服务，不触发迁移或主服务启动。
func TestHGSDKDebugProtocols(t *testing.T) {
	if os.Getenv("MLC_IM_SDK_INTEGRATION") != "1" {
		t.Skip("需显式启用本机 debug SDK 协议实测")
	}
	if os.Getenv("SERVER_ENV") != "debug" {
		t.Fatal("SDK 协议实测仅允许 SERVER_ENV=debug")
	}
	configDir, err := filepath.Abs("../../../config")
	if err != nil {
		t.Fatal("无法定位仓库 debug 配置目录")
	}
	t.Setenv("MLC_CONFIG_DIR", configDir)
	if err := Config.InitRuntimeEnv(); err != nil {
		t.Fatal("无法加载本机 debug 环境配置")
	}
	t.Run("CentrifugoPublish", func(t *testing.T) {
		endpoint, err := hgSDKDebugEndpoint("MLC_IM_CENTRIFUGO_API_URL", "18000")
		if err != nil {
			t.Fatal(err)
		}
		gateway, err := HGNewCentrifugo(endpoint, os.Getenv("MLC_IM_CENTRIFUGO_API_KEY"), os.Getenv("MLC_IM_CENTRIFUGO_TOKEN_SECRET"))
		if err != nil {
			t.Fatal("网关凭据配置不完整")
		}
		defer gateway.HGClose()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if err := gateway.HGPublish(ctx, hgSDKProbeID(t), json.RawMessage(`{"probe":"sdk-debug"}`)); err != nil {
			t.Fatal("Centrifugo SDK 未确认发布")
		}
	})
	t.Run("SeaweedFSGet", func(t *testing.T) {
		endpoint, err := hgSDKDebugEndpoint("MLC_IM_S3_ENDPOINT", "8333")
		if err != nil {
			t.Fatal(err)
		}
		region, bucket := os.Getenv("MLC_IM_S3_REGION"), os.Getenv("MLC_IM_S3_BUCKET")
		key, secret := os.Getenv("MLC_IM_S3_ACCESS_KEY"), os.Getenv("MLC_IM_S3_SECRET_KEY")
		storage, err := HGNewSeaweedFS(endpoint, region, bucket, key, secret)
		if err != nil {
			t.Fatal("私有 S3 配置不完整")
		}
		defer storage.HGClose()
		client := s3.NewFromConfig(aws.Config{Region: region, Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""), HTTPClient: hgClient()}, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
			o.RetryMaxAttempts = 1
		})
		objectKey := "quarantine/" + hgSDKProbeID(t)
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		// 先验证临时 key 的删除权限；没有权限则不写入测试对象。
		if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(objectKey)}); err != nil {
			t.Skip("当前上传身份无法清理临时对象，不执行写入")
		}
		uploaded := false
		defer func() {
			if !uploaded {
				return
			}
			cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			if _, err := client.DeleteObject(cleanup, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(objectKey)}); err != nil {
				t.Errorf("临时对象清理失败，需人工核对 key=%s", objectKey)
			}
		}()
		body := []byte("mlc-im-sdk-debug-probe")
		link, err := storage.HGUploadURL(objectKey, "text/plain", int64(len(body)))
		if err != nil {
			t.Fatal("生成临时上传签名失败")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, link, bytes.NewReader(body))
		if err != nil {
			t.Fatal("创建临时上传请求失败")
		}
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("x-amz-acl", "private")
		// 即使上传返回非 2xx 也尝试清理，避免不确定提交后的残留。
		uploaded = true
		resp, err := storage.client.Do(req)
		if err != nil {
			t.Fatal("临时上传请求失败")
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("临时上传未确认: HTTP %d", resp.StatusCode)
		}
		read, err := storage.HGReviewOpenQuarantine(ctx, objectKey, int64(len(body)))
		if err != nil {
			t.Fatal("S3 SDK 预签名 GET 未通过存储认证或长度校验")
		}
		defer read.Close()
		actual, err := io.ReadAll(io.LimitReader(read, int64(len(body))+1))
		if err != nil || !bytes.Equal(actual, body) {
			t.Fatal("S3 SDK GET 字节与写入内容不一致")
		}
	})
}
