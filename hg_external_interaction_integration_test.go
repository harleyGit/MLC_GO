package main

import (
	hgComment "MLC_GO/internal/modules/video_comment/repository"
	hgDanmaku "MLC_GO/internal/modules/video_danmaku/repository"
	hgVideo "MLC_GO/internal/modules/video_upload/repository"
	hgConfig "MLC_GO/internal/pkg/config"
	hgMySQL "MLC_GO/internal/pkg/mysql"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestHGExternalInteractionTargets 仅显式开启时点查debug现有快照，不写评论或弹幕。
func TestHGExternalInteractionTargets(t *testing.T) {
	if os.Getenv("HG_EXTERNAL_INTERACTION_CHECK") != "1" {
		t.Skip("未开启debug只读核对")
	}
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("CRAWLER_VIDEO_PIPELINE_ENABLED", "true")
	if hgErr := hgConfig.InitRuntimeEnv(); hgErr != nil {
		t.Fatal(hgErr)
	}
	if hgErr := hgConfig.LoadConfig("debug"); hgErr != nil {
		t.Fatal(hgErr)
	}
	hgDB, hgErr := hgMySQL.NewSQLManager()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgCtx, hgCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer hgCancel()
	hgID := "external:bilibili:BV1wQJ26XEPT"
	hgItem, hgFound, hgErr := hgVideo.NewRepository(hgDB.GetSQLDB()).GetVideoListItemByID(hgCtx, hgID)
	if hgErr != nil || !hgFound || hgItem.HGExternalDetail == nil || len(hgItem.HGExternalDetail.Parts) == 0 {
		t.Fatalf("缺少已入库测试视频: %v", hgErr)
	}
	hgComments := hgComment.NewRepository(hgDB.GetSQLDB())
	if _, hgErr = hgComments.List(hgCtx, "", hgID, "latest", hgComment.HGListCursor{}, 20); hgErr != nil {
		t.Fatal(hgErr)
	}
	hgRepo := hgDanmaku.NewRepository(hgDB.GetSQLDB())
	hgTarget := hgID + ":" + hgItem.HGExternalDetail.Parts[0].CID
	hgSubmission, hgErr := hgRepo.ResolveVideo(hgCtx, hgTarget)
	if hgErr != nil || hgSubmission != hgID {
		t.Fatalf("外部分P解析错误: %q %v", hgSubmission, hgErr)
	}
	if _, hgErr = hgRepo.ResolveVideo(hgCtx, hgID+":0"); !errors.Is(hgErr, hgDanmaku.ErrVideoNotDanmakuEnabled) {
		t.Fatalf("未拒绝伪造CID: %v", hgErr)
	}
	if _, hgErr = hgRepo.ResolveVideo(hgCtx, hgID); !errors.Is(hgErr, hgDanmaku.ErrVideoNotDanmakuEnabled) {
		t.Fatalf("未拒绝无分P身份: %v", hgErr)
	}
	t.Log("已入库外部评论、分P弹幕身份及伪造CID拒绝均通过")
}
