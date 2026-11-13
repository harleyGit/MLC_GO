package media

import (
	Transport "MLC_GO/internal/modules/im/transport"
	Config "MLC_GO/internal/pkg/config"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/go-sql-driver/mysql"
)

// hgDebugConfig 仅允许显式 opt-in 的本机隔离服务；不打印配置和 SDK 原始错误（可能包含签名 URL）。
func hgDebugConfig(t *testing.T) (string, string, *HGSeaweedStore, *HGSeaweedStore) {
	t.Helper()
	if os.Getenv("MLC_MEDIA_DEBUG_INTEGRATION") != "1" {
		t.Skip("需要显式启用本机媒体集成测试")
	}
	if os.Getenv("SERVER_ENV") != "debug" {
		t.Fatal("媒体集成测试仅允许 SERVER_ENV=debug")
	}
	configDir, err := filepath.Abs("../../../config")
	if err != nil {
		t.Fatal("配置目录不存在")
	}
	t.Setenv("MLC_CONFIG_DIR", configDir)
	if err = Config.LoadConfig("debug"); err != nil {
		t.Fatal("无法加载本机 debug 配置")
	}
	endpoint := os.Getenv("MLC_IM_S3_ENDPOINT")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() != "8333" || u.Path != "" || u.RawQuery != "" || u.User != nil {
		t.Fatal("测试仅允许已核对的本机 S3 端口")
	}
	reviewURL := os.Getenv("MLC_IM_REVIEW_URL")
	r, err := url.Parse(reviewURL)
	if err != nil || r.Scheme != "http" || r.Hostname() != "127.0.0.1" || r.Port() != "8092" || r.Path != "/review" || r.RawQuery != "" || r.User != nil {
		t.Fatal("测试仅允许本机审核器")
	}
	bucket := os.Getenv("MLC_IM_S3_BUCKET")
	if bucket != "im-private" {
		t.Fatal("审核器必须绑定已核对的本机 debug 桶")
	}
	store, err := HGNewSeaweedStore(endpoint, os.Getenv("MLC_IM_S3_REGION"), bucket, os.Getenv("MLC_IM_S3_ACCESS_KEY"), os.Getenv("MLC_IM_S3_SECRET_KEY"))
	if err != nil {
		t.Fatal("媒体存储身份配置不可用")
	}
	verifier, err := HGNewSeaweedStore(endpoint, os.Getenv("MLC_IM_S3_REGION"), bucket, os.Getenv("MLC_IM_REVIEW_S3_ACCESS_KEY"), os.Getenv("MLC_IM_REVIEW_S3_SECRET_KEY"))
	if err != nil {
		t.Fatal("受信审核器 S3 身份配置不可用")
	}
	return bucket, reviewURL, store, verifier
}

func hgDebugRandomID(t *testing.T) string {
	t.Helper()
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal("生成随机测试标识失败")
	}
	return hex.EncodeToString(raw[:])
}

func hgDebugAbsent(t *testing.T, ctx context.Context, store *HGSeaweedStore, key string) {
	t.Helper()
	_, err := store.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)})
	var api smithy.APIError
	if err == nil || !errors.As(err, &api) || api.ErrorCode() != "NotFound" && api.ErrorCode() != "NoSuchKey" && api.ErrorCode() != "404" {
		t.Fatal("随机对象键不存在性未获确认；拒绝覆盖或操作现有文件")
	}
}

func hgDebugDeleteProbe(t *testing.T, store, verifier *HGSeaweedStore, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	for _, prefix := range []string{"quarantine/", "sealed/"} {
		key := prefix + id
		hgDebugAbsent(t, ctx, verifier, key)
		if _, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)}); err != nil {
			t.Skip(fmt.Sprintf("%s 删除权限未确认，不写入测试对象", strings.TrimSuffix(prefix, "/")))
		}
	}
}

func hgDebugCleanupStore(t *testing.T, upload *HGSeaweedStore) *HGSeaweedStore {
	t.Helper()
	key, secret := os.Getenv("MLC_MEDIA_CLEANUP_S3_ACCESS_KEY"), os.Getenv("MLC_MEDIA_CLEANUP_S3_SECRET_KEY")
	if key == "" || secret == "" {
		t.Skip("未注入本机媒体专用清理身份；不写入测试对象")
	}
	if key == os.Getenv("MLC_IM_S3_ACCESS_KEY") || key == os.Getenv("MLC_IM_REVIEW_S3_ACCESS_KEY") {
		t.Fatal("专用清理身份不能复用上传/审核身份")
	}
	store, err := HGNewSeaweedStore(os.Getenv("MLC_IM_S3_ENDPOINT"), os.Getenv("MLC_IM_S3_REGION"), upload.bucket, key, secret)
	if err != nil {
		t.Fatal("媒体专用清理身份配置不完整")
	}
	return store
}

// TestHGDebugDeletePermissions 必须先对不存在的随机键确认服务身份有 quarantine/ 和 sealed/ 删除权限。
// 只发送 Delete/Head，不创建任何测试对象；权限不足时后续不得运行上传验收。
func TestHGDebugDeletePermissions(t *testing.T) {
	_, _, store, verifier := hgDebugConfig(t)
	cleanup := hgDebugCleanupStore(t, store)
	hgDebugDeleteProbe(t, cleanup, verifier, hgDebugRandomID(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deniedKey := "sealed/" + hgDebugRandomID(t)
	hgDebugAbsent(t, ctx, verifier, deniedKey)
	_, err := cleanup.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(deniedKey), Body: bytes.NewReader([]byte("x")), ContentLength: aws.Int64(1)})
	if err == nil {
		if removeErr := cleanup.HGDelete(ctx, deniedKey); removeErr != nil {
			t.Error("清理身份意外可 PUT，且新测试对象兜底删除失败")
		}
		t.Fatal("专用清理身份具有封存写入权限，必须停止后续验收")
	}
	var denied smithy.APIError
	if !errors.As(err, &denied) || denied.ErrorCode() != "AccessDenied" {
		t.Fatal("专用清理身份的写入拒绝结果不可验证")
	}
	t.Log("本机随机空键的隔离/封存删除请求均被接受；不代表真实对象删除已验证")
}

// TestHGDebugMediaLifecycle 验收本机 debug 的真实图片生成/签名 PUT/可信审核/封存复查/持久化过期删除。
// 必须在确认两种键的删除权限、没有其他媒体任务后才能创建任何对象；不能用旧文件充当夹具。
func TestHGDebugMediaLifecycle(t *testing.T) {
	_, reviewURL, store, verifier := hgDebugConfig(t)
	cleanup := hgDebugCleanupStore(t, store)
	// 空键权限探测不代表真删除成功，因此测试最终还会 HEAD 校验已写对象的消失。
	hgDebugDeleteProbe(t, cleanup, verifier, hgDebugRandomID(t))
	cfg, err := Config.GetMySQLConfig()
	if err != nil || cfg.Host != "127.0.0.1" || cfg.Port != "3306" || cfg.Database != "HG_MLC_DB" {
		t.Fatal("数据库不是已授权的独占 debug 实例")
	}
	dsn := mysql.NewConfig()
	dsn.User, dsn.Passwd, dsn.Net, dsn.Addr, dsn.DBName = cfg.User, cfg.Password, "tcp", "127.0.0.1:3306", cfg.Database
	dsn.ParseTime = true
	dsn.Loc = time.UTC
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		t.Fatal("无法创建本机 debug 数据库连接")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		t.Fatal("隔离库不可达")
	}
	var version int
	var dirty bool
	if err = db.QueryRowContext(ctx, SQL.HGGetCrawlerMigrationVersionSQL).Scan(&version, &dirty); err != nil || version != 72 || dirty {
		t.Fatal("媒体 DDL 状态不是 72 dirty=false")
	}
	// 不让全局领取接口误处理任何历史或其他进程创建的媒体对象。
	var total int
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugEmptySQL).Scan(&total); err != nil || total != 0 {
		t.Skip("新媒体表已有资源或任务，不能安全执行全局 worker 验收")
	}
	reviewer, err := Transport.HGNewReviewer(reviewURL, os.Getenv("MLC_IM_REVIEW_TOKEN"))
	if err != nil {
		t.Fatal("审核器配置不完整")
	}
	defer reviewer.HGClose()
	service, err := HGNewService(db, store, store.bucket)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.HGEnableReview(reviewer, &hgWorkerStore{head: store, cleanup: cleanup}); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 60, G: 110, B: 180, A: 255})
		}
	}
	var data bytes.Buffer
	if err = png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data.Bytes())
	owner := "media_debug_" + hgDebugRandomID(t)
	session, err := service.HGInit(ctx, owner, HGUploadRequest{Purpose: "user_avatar", MIME: "image/png", Size: int64(data.Len()), SHA256: hex.EncodeToString(hash[:]), IdempotencyKey: hgDebugRandomID(t)})
	if err != nil {
		var dbErr *mysql.MySQLError
		if errors.As(err, &dbErr) {
			t.Fatalf("创建测试媒体会话失败：MySQL 错误码 %d（内容已隐藏）", dbErr.Number)
		}
		t.Fatalf("创建测试媒体会话失败：错误类型 %T（内容已隐藏）", err)
	}
	id := strings.ReplaceAll(session.AssetID, "-", "")
	quarantine, sealed := "quarantine/"+id, "sealed/"+id
	// 从此刻起仅针对本次新生成的键做兜底清理，不会触碰旧对象。
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		for _, key := range []string{quarantine, sealed} {
			if err := service.sealed.HGDelete(cleanup, key); err != nil {
				t.Errorf("随机测试对象兜底清理失败，须检查本次资源 ID=%s", session.AssetID)
			}
		}
	}()
	hgDebugAbsent(t, ctx, verifier, quarantine)
	hgDebugAbsent(t, ctx, verifier, sealed)
	if session.UploadURL == "" || session.Status != "pending" {
		t.Fatal("未生成隔离 PUT 授权")
	}
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, session.UploadURL, bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal("构造测试上传请求失败")
	}
	put.Header.Set("Content-Type", "image/png")
	put.ContentLength = int64(data.Len())
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(put)
	if err != nil {
		t.Fatal("随机图片上传失败")
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		t.Fatalf("图片 PUT 未成功：HTTP %d", response.StatusCode)
	}
	completed, err := service.HGComplete(ctx, owner, session.UploadID)
	if err != nil || completed.Status != "processing" {
		t.Fatal("完成阶段未进入审核队列")
	}
	if err = service.HGReviewOnce(ctx); err != nil {
		t.Fatal("真实审核或封存失败")
	}
	state, err := service.HGAssetStatus(ctx, owner, session.AssetID)
	if err != nil || state["status"] != "ready" {
		t.Fatal("真实审核未确认 ready")
	}
	readURL, err := verifier.HGSignRead(ctx, sealed)
	if err != nil {
		t.Fatal("封存读授权失败")
	}
	get, err := http.NewRequestWithContext(ctx, http.MethodGet, readURL, nil)
	if err != nil {
		t.Fatal("构造封存读取失败")
	}
	sealedResponse, err := (&http.Client{Timeout: 10 * time.Second}).Do(get)
	if err != nil {
		t.Fatal("读取封存对象失败")
	}
	actual, err := io.ReadAll(io.LimitReader(sealedResponse.Body, int64(data.Len()+1)))
	sealedResponse.Body.Close()
	if err != nil || sealedResponse.StatusCode != http.StatusOK || !bytes.Equal(actual, data.Bytes()) {
		t.Fatal("封存字节与生成的原图不一致")
	}
	// 仅尝试覆盖本次随机封存键：若 Gateway 忽略条件写入，测试失败并由兜底清理删除测试对象。
	_, err = verifier.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(sealed), Body: bytes.NewReader([]byte("not-an-image")), ContentLength: aws.Int64(12), ContentType: aws.String("image/png"), IfNoneMatch: aws.String("*")})
	var condition smithy.APIError
	if !errors.As(err, &condition) || condition.ErrorCode() != "PreconditionFailed" && condition.ErrorCode() != "412" {
		t.Fatal("SeaweedFS 未确认拒绝对已封存测试对象的条件覆盖")
	}
	// 审核器重复处理相同 ID 必须幂等且不能覆盖封存字节。
	result, err := reviewer.HGReview(ctx, id, store.bucket, quarantine, hex.EncodeToString(hash[:]), "image/png", int64(data.Len()))
	if err != nil || result.Status != "ready" {
		t.Fatal("重复审核未保持幂等封存")
	}
	if _, err = db.ExecContext(ctx, SQL.HGMediaDebugExpireSQL, session.UploadID, session.AssetID, owner); err != nil {
		t.Fatal("只更新本次测试会话过期时间失败")
	}
	if err = service.HGExpireOnce(ctx); err != nil {
		t.Fatal("过期任务未创建删除任务")
	}
	for i := 0; i < 2; i++ {
		if err = service.HGDeleteOnce(ctx); err != nil {
			t.Fatal("持久任务删除 S3 对象失败")
		}
	}
	var final string
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugAssetStatusSQL, session.AssetID, owner).Scan(&final); err != nil || final != "deleted" {
		t.Fatal("持久删除任务未完成资源终态")
	}
	hgDebugAbsent(t, ctx, verifier, quarantine)
	hgDebugAbsent(t, ctx, verifier, sealed)
	hgDebugPurgeFinished(t, ctx, db, session.AssetID, owner, session.UploadID)
	t.Log("新生成图片：真实审核、重复封存、持久删除任务和对象消失均经本机 debug 校验")
}

// TestHGDebugLegacyMetadataSample 仅查看每个旧来源主键前 100 条元数据的形态与归属缺口；
// 不读取旧对象、不扫描全表、不开启双读，且不输出 URL、键、用户或会话标识。
func TestHGDebugLegacyMetadataSample(t *testing.T) {
	_, _, _, _ = hgDebugConfig(t)
	cfg, err := Config.GetMySQLConfig()
	if err != nil || cfg.Host != "127.0.0.1" || cfg.Port != "3306" || cfg.Database != "HG_MLC_DB" {
		t.Fatal("只允许本机独占 debug 数据库")
	}
	dsn := mysql.NewConfig()
	dsn.User, dsn.Passwd, dsn.Net, dsn.Addr, dsn.DBName = cfg.User, cfg.Password, "tcp", "127.0.0.1:3306", cfg.Database
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		t.Fatal("无法创建只读盘点连接")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for _, source := range []struct {
		name, query string
		fields      int
	}{
		{"users.avatar_url", SQL.HGMediaLegacyAvatarSampleSQL, 2},
		{"mall_product_image.storage_key", SQL.HGMediaLegacyMallSampleSQL, 4},
		{"video_files.file_path", SQL.HGMediaLegacyVideoSampleSQL, 4},
		{"im_private_attachment.object_key", SQL.HGMediaLegacyIMSampleSQL, 5},
	} {
		rows, err := db.QueryContext(ctx, source.query)
		if err != nil {
			t.Fatalf("%s 的只读样本不可用", source.name)
		}
		observed := 0
		for rows.Next() {
			fields := make([]any, source.fields)
			values := make([]sql.RawBytes, source.fields)
			for i := range fields {
				fields[i] = &values[i]
			}
			if err = rows.Scan(fields...); err != nil {
				break
			}
			observed++
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			t.Fatalf("%s 的只读样本解析失败", source.name)
		}
		t.Logf("%s：主键前 %d 条仅核对字段可读性；不是全量盘点或权威归属确认", source.name, observed)
	}
}

// TestHGDebugInspectPendingFixture 只读确认第一次试验遗留的单条新媒体记录，不输出标识或敏感字段。
func TestHGDebugInspectPendingFixture(t *testing.T) {
	if os.Getenv("MLC_MEDIA_DEBUG_RECOVER") != "1" {
		t.Skip("仅在明确恢复遗留测试记录时执行")
	}
	_, _, store, _ := hgDebugConfig(t)
	cfg, err := Config.GetMySQLConfig()
	if err != nil || cfg.Host != "127.0.0.1" || cfg.Database != "HG_MLC_DB" {
		t.Fatal("仅允许本机隔离库")
	}
	dsn := mysql.NewConfig()
	dsn.User, dsn.Passwd, dsn.Net, dsn.Addr, dsn.DBName = cfg.User, cfg.Password, "tcp", "127.0.0.1:3306", cfg.Database
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		t.Fatal("数据库连接失败")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var total int
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugEmptySQL).Scan(&total); err != nil {
		t.Fatal("检查媒体表失败")
	}
	var asset, owner, state, key string
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugInspectSQL).Scan(&asset, &owner, &state, &key); err != nil {
		t.Fatal("新媒体记录不可读")
	}
	var upload, uploadState string
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugInspectSessionSQL, asset).Scan(&upload, &uploadState); err != nil {
		t.Fatal("测试会话不可读")
	}
	if total != 1 || !strings.HasPrefix(owner, "media_debug_") || !hgQuarantineKey.MatchString(key) || key != "quarantine/"+strings.ReplaceAll(asset, "-", "") {
		t.Fatal("新媒体表内容不全是本次试验记录，拒绝自动清理")
	}
	_, signErr := store.HGSignUpload(ctx, key, "image/png", 128)
	var api smithy.APIError
	if errors.As(signErr, &api) {
		t.Logf("只读诊断：会话状态=%s/%s，签名服务错误码=%s", state, uploadState, api.ErrorCode())
	} else {
		t.Logf("只读诊断：会话状态=%s/%s，签名错误类型=%T", state, uploadState, signErr)
	}
}

// TestHGDebugRecoverPendingFixture 仅针对本次 HGInit 已落库、从未上传的唯一测试会话执行正常过期/删除。
// 全局新媒体表存在任何其他资产或任务时停止，绝不清理旧文件。
func TestHGDebugRecoverPendingFixture(t *testing.T) {
	if os.Getenv("MLC_MEDIA_DEBUG_RECOVER") != "1" {
		t.Skip("仅在明确恢复遗留测试记录时执行")
	}
	_, reviewURL, uploadStore, verifier := hgDebugConfig(t)
	_ = reviewURL
	cleanup := hgDebugCleanupStore(t, uploadStore)
	hgDebugDeleteProbe(t, cleanup, verifier, hgDebugRandomID(t))
	cfg, err := Config.GetMySQLConfig()
	if err != nil || cfg.Host != "127.0.0.1" || cfg.Port != "3306" || cfg.Database != "HG_MLC_DB" {
		t.Fatal("仅允许本机隔离库")
	}
	dsn := mysql.NewConfig()
	dsn.User, dsn.Passwd, dsn.Net, dsn.Addr, dsn.DBName = cfg.User, cfg.Password, "tcp", "127.0.0.1:3306", cfg.Database
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		t.Fatal("无法连接隔离库")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var total int
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugEmptySQL).Scan(&total); err != nil || total != 1 {
		t.Fatal("新媒体表不是唯一一条待恢复记录，停止")
	}
	var asset, owner, state, key string
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugInspectSQL).Scan(&asset, &owner, &state, &key); err != nil || !strings.HasPrefix(owner, "media_debug_") || state != "pending" || key != "quarantine/"+strings.ReplaceAll(asset, "-", "") {
		t.Fatal("记录不匹配本次测试会话，停止")
	}
	var upload, sessionState string
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugInspectSessionSQL, asset).Scan(&upload, &sessionState); err != nil || sessionState != "pending" {
		t.Fatal("只有未上传的测试会话可恢复")
	}
	sealed := "sealed/" + strings.ReplaceAll(asset, "-", "")
	hgDebugAbsent(t, ctx, verifier, key)
	hgDebugAbsent(t, ctx, verifier, sealed)
	result, err := db.ExecContext(ctx, SQL.HGMediaDebugExpirePendingSQL, upload, asset, owner)
	if err != nil {
		t.Fatal("更新测试会话过期时间失败")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		t.Fatal("测试过期标记未命中唯一会话")
	}
	service, err := HGNewService(db, uploadStore, uploadStore.bucket)
	if err != nil {
		t.Fatal(err)
	}
	service.sealed = &hgWorkerStore{head: uploadStore, cleanup: cleanup}
	if err = service.HGExpireOnce(ctx); err != nil {
		t.Fatal("测试会话过期任务失败")
	}
	for i := 0; i < 2; i++ {
		if err = service.HGDeleteOnce(ctx); err != nil {
			t.Fatal("测试会话持久删除任务失败")
		}
	}
	hgDebugAbsent(t, ctx, verifier, key)
	hgDebugAbsent(t, ctx, verifier, sealed)
	var final string
	if err = db.QueryRowContext(ctx, SQL.HGMediaDebugAssetStatusSQL, asset, owner).Scan(&final); err != nil || final != "deleted" {
		t.Fatal("测试记录未完成删除终态")
	}
	hgDebugPurgeFinished(t, ctx, db, asset, owner, upload)
	t.Log("唯一测试会话的两个不存在对象已通过持久任务删除，测试数据库夹具已清理")
}

func hgDebugPurgeFinished(t *testing.T, ctx context.Context, db *sql.DB, asset, owner, upload string) {
	t.Helper()
	var jobs int
	if err := db.QueryRowContext(ctx, SQL.HGMediaDebugDoneJobsSQL, asset).Scan(&jobs); err != nil || jobs != 2 {
		t.Fatal("两个测试对象删除任务未全部完成")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("测试元数据清理事务失败")
	}
	defer tx.Rollback()
	for _, step := range []struct {
		query    string
		args     []any
		expected int64
	}{
		{SQL.HGMediaDebugDeleteJobsSQL, []any{asset}, 2},
		{SQL.HGMediaDebugDeleteSessionSQL, []any{upload, asset, owner}, 1},
		{SQL.HGMediaDebugDeleteAssetSQL, []any{asset, owner}, 1},
		{SQL.HGMediaDebugDeleteQuotaSQL, []any{owner}, 1},
	} {
		res, err := tx.ExecContext(ctx, step.query, step.args...)
		if err != nil {
			t.Fatal("测试夹具元数据清理失败")
		}
		affected, err := res.RowsAffected()
		if err != nil || affected != step.expected {
			t.Fatal("测试夹具清理行数不符，已回滚")
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal("测试元数据清理提交失败")
	}
}
