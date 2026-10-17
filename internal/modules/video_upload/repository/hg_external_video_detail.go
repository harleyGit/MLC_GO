package VideoUploadRepositoryPackage

import (
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	ConfigPackage "MLC_GO/internal/pkg/config"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// hgLoadExternalDetail 仅在详情点查加载快照，列表维持轻量；未启用迁移的环境不访问新表。
func (r *Repository) hgLoadExternalDetail(ctx context.Context, item *VideoUploadDtoPackage.VideoListItem) error {
	if item.SourcePlatform != "bilibili" || !ConfigPackage.HGCrawlerVideoPipelineEnabled() {
		return nil
	}
	extra := &VideoUploadDtoPackage.HGExternalVideoDetail{Status: "not_collected", Parts: []VideoUploadDtoPackage.HGExternalVideoPart{}}
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGGetVideoCollectionStatusSQL, item.ExternalContentID).Scan(&extra.Status, &extra.ErrorMessage)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	item.HGExternalDetail = extra
	var body, stats []byte
	err = r.db.QueryRowContext(ctx, SQLQueriesPackage.HGGetVideoDetailSQL, item.ExternalContentID).Scan(&body, &stats)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var d CrawlerModelPackage.HGVideoDetail
	if err = json.Unmarshal(body, &d); err != nil {
		return err
	}
	if err = json.Unmarshal(stats, &d.Stats); err != nil {
		return err
	}
	item.Title = d.Title
	item.Description = d.Description
	item.CoverURL = d.CoverURL
	item.UserID = d.AuthorID
	item.AuthorName = d.AuthorName
	item.Duration = d.Duration
	item.Category = d.Category
	item.SubmitTime = d.PublishedAt.Format(time.RFC3339)
	item.ViewCount = d.Stats.View
	item.LikeCount = d.Stats.Like
	item.CommentCount = d.Stats.Danmaku
	extra.AuthorAvatar = d.AuthorAvatar
	extra.CollectedAt = d.CollectedAt.Format(time.RFC3339Nano)
	extra.Coin = d.Stats.Coin
	extra.Favorite = d.Stats.Favorite
	extra.Share = d.Stats.Share
	extra.Reply = d.Stats.Reply
	extra.Danmaku = d.Stats.Danmaku
	for _, part := range d.Parts {
		extra.Parts = append(extra.Parts, VideoUploadDtoPackage.HGExternalVideoPart{CID: part.CID, Page: part.Page, Title: part.Title, Duration: part.Duration})
	}
	return nil
}
