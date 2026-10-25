package product

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	q "MLC_GO/internal/pkg/mysql/queries"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestHGMallImageReserveDebugSQL 验证商品图片配额登记 SQL 能在本机 debug MySQL 执行，事务始终回滚。
func TestHGMallImageReserveDebugSQL(t *testing.T) {
	if os.Getenv("MLC_MALL_IMAGE_INTEGRATION") != "1" || os.Getenv("SERVER_ENV") != "debug" {
		t.Skip("仅在获准的本机 debug MySQL 环境执行")
	}
	t.Setenv("MLC_CONFIG_DIR", filepath.Join("..", "..", "..", "config"))
	if err := ConfigPackage.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	cfg, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Database != "HG_MLC_DB" {
		t.Fatal("数据库不是本机隔离 debug 库")
	}
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp", Addr: net.JoinHostPort(cfg.Host, cfg.Port), DBName: cfg.Database}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("无法打开本机 debug MySQL")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("无法开启本机 debug 事务")
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, q.HGMallImageReserveSQL,
		"http://localhost:8080/uploads/mall_product/sql-probe.png", "mall_product/sql-probe.png", "hg-mall-image-diag", "hg-mall-image-diag")
	if err != nil {
		t.Fatalf("图片登记 SQL 失败: %v", err)
	}
}

// TestHGMallImageUploadDebugRoundTrip 覆盖真实 MySQL 配额登记、对象写入和清理，账号及文件均为测试生成。
func TestHGMallImageUploadDebugRoundTrip(t *testing.T) {
	if os.Getenv("MLC_MALL_IMAGE_INTEGRATION") != "1" || os.Getenv("SERVER_ENV") != "debug" {
		t.Skip("仅在获准的本机 debug MySQL 环境执行")
	}
	t.Setenv("MLC_CONFIG_DIR", filepath.Join("..", "..", "..", "config"))
	if err := ConfigPackage.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	cfg, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Database != "HG_MLC_DB" {
		t.Fatal("数据库不是本机隔离 debug 库")
	}
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp", Addr: net.JoinHostPort(cfg.Host, cfg.Port), DBName: cfg.Database}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("无法打开本机 debug MySQL")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	user := "hg-mall-image-diag-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	merchantID := "MER_" + user
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, q.HGMallDebugRemoveImagesSQL, user); err != nil {
			t.Errorf("调试图片清理失败: %v", err)
		}
		if _, err := db.ExecContext(cleanupCtx, q.HGMallDebugRemoveMerchantSQL, merchantID, user); err != nil {
			t.Errorf("调试店铺清理失败: %v", err)
		}
	}()
	repo := HGNewRepository(db)
	if err := repo.Apply(ctx, HGMerchantApplication{MerchantID: merchantID, UserID: user, ShopName: "商品上传调试"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReviewMerchant(ctx, "hg-mall-image-test-reviewer", merchantID, 1); err != nil {
		t.Fatal(err)
	}
	uploadCfg := HGUploadPackage.DefaultConfig()
	uploadCfg.MaxFileSize = 5 << 20
	uploadCfg.UploadDir = t.TempDir()
	uploadCfg.BaseURL = "http://localhost:8080"
	uploader := HGUploadPackage.NewUploader(uploadCfg)
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	store := HGNewImageStore(db, uploader)
	url, err := store.HGUploadImage(ctx, user, "png", bytes.NewReader(imageBytes.Bytes()), int64(imageBytes.Len()))
	if err != nil {
		t.Fatalf("本机 debug 上传商品图片失败: %v", err)
	}
	if err := store.HGDeleteUnusedImage(ctx, user, url); err != nil {
		t.Fatalf("本机 debug 清理上传图片失败: %v", err)
	}
}
