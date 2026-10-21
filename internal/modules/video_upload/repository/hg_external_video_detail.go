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

// hgLoadExternalDetail 从本地数据库加载已经爬取的 B 站视频详情快照，并填充到视频列表对象 item 中，最终返回给前端。
// 这里的快照（Snapshot），可以简单理解为：某个时间点从 B 站爬取并保存下来的视频数据。
/* 执行流程：
B站视频详情
    |
    | 爬虫采集
    v
本地数据库（保存视频快照）
    |
    | hgLoadExternalDetail()
    v
读取视频详情 + 统计数据
    |
    v
填充 VideoListItem
    |
    v
返回前端 JSON
*/
func (r *Repository) hgLoadExternalDetail(ctx context.Context, item *VideoUploadDtoPackage.VideoListItem) error {
	// 条件是：视频来源是 bilibili｜｜ 爬虫视频处理流水线已启用。
	if item.SourcePlatform != "bilibili" || !ConfigPackage.HGCrawlerVideoPipelineEnabled() {
		return nil
	}
	// 创建一个视频扩展详情对象。
	// not_collected 表示默认尚未采集。
	// Parts 是视频分 P 列表，初始化为空数组。
	extra := &VideoUploadDtoPackage.HGExternalVideoDetail{Status: "not_collected", Parts: []VideoUploadDtoPackage.HGExternalVideoPart{}}
	// 查询视频采集状态， 根据视频的外部 ID（例如 B 站 BV 号），查询采集状态和错误信息。
	/* 比如：
	BV1xxxx
		|
		v
	查询采集状态
		|
		+-- pending：等待采集
		+-- success：采集成功（示例状态）
		+-- failed：采集失败（示例状态）
	实际状态值取决于数据库和爬虫系统的定义。
	*/
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGGetVideoCollectionStatusSQL, item.ExternalContentID).Scan(&extra.Status, &extra.ErrorMessage)
	// 没有记录：允许继续。数据库发生其他错误：返回错误。
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// 把扩展信息挂载到 item 上。
	item.HGExternalDetail = extra
	var body, stats []byte
	// 查询保存的视频快照，body：视频标题、简介、封面、作者、时长等， stats：播放、点赞、收藏、分享等统计数据
	err = r.db.QueryRowContext(ctx, SQLQueriesPackage.HGGetVideoDetailSQL, item.ExternalContentID).Scan(&body, &stats)
	// 若是没有快照，直接结束，保留原有item数据以及刚刚设置的扩展状态
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var d CrawlerModelPackage.HGVideoDetail
	// 将数据库保存的JSON转换成结构体
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
