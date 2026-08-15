package platform

import (
	"context"
	"time"
)

// HGRecommendation 是第三方平台推荐内容的统一业务模型。
// 字段来源于第三方公开推荐接口，计数字段是抓取时快照而不是本站互动数据。
// 第一版只保存公开元数据和原站链接，不下载、解析或代理媒体文件。
type HGRecommendation struct {
	Platform     string    `json:"platform"`        // 平台稳定标识，例如 bilibili。
	ContentID    string    `json:"contentId"`       // 平台内容唯一标识，例如 BVID。
	Title        string    `json:"title"`           // 上游公开视频标题。
	AuthorID     string    `json:"authorId"`        // 字符串保存第三方 ID，避免前端整数精度丢失。
	AuthorName   string    `json:"authorName"`      // 抓取时作者名称快照。
	CoverURL     string    `json:"coverUrl"`        // 第三方封面地址，不代表本站已持久化资源。
	TargetURL    string    `json:"targetUrl"`       // 第三方内容落地页，不是可直接播放的媒体文件。
	Duration     int64     `json:"durationSeconds"` // 视频时长，单位秒。
	ViewCount    int64     `json:"viewCount"`       // 第三方播放量快照。
	LikeCount    int64     `json:"likeCount"`       // 第三方点赞量快照。
	CommentCount int64     `json:"commentCount"`    // 当前 Bilibili 实现映射弹幕量快照。
	PublishedAt  time.Time `json:"publishedAt"`     // 第三方发布时间，统一转换为 UTC。
}

// HGPlatform 定义可被调度器执行的第三方平台推荐数据源。
// 实现必须观察调用方 context，并返回标准化模型，不能把平台私有协议泄漏到调度层。
type HGPlatform interface {
	Name() string
	FetchRecommendations(ctx context.Context) ([]HGRecommendation, error)
}
