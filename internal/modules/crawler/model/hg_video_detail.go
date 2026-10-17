package model

import "time"

// HGVideoDetail 是清洗后的外部视频快照；计数属于原站，不代表本站用户互动。
type HGVideoDetail struct {
	BVID         string        `json:"bvid"`         // Bilibili 视频唯一标识。
	AID          int64         `json:"aid"`          // 原站稿件编号。
	Title        string        `json:"title"`        // 标题，最多 255 字符。
	Description  string        `json:"description"`  // 纯文本简介，最多 16384 字符。
	CoverURL     string        `json:"coverUrl"`     // HTTPS 封面地址。
	AuthorID     string        `json:"authorId"`     // 作者编号，字符串避免 JS 精度丢失。
	AuthorName   string        `json:"authorName"`   // 作者昵称快照。
	AuthorAvatar string        `json:"authorAvatar"` // HTTPS 头像地址。
	Category     string        `json:"category"`     // 原站分区名称。
	Duration     int64         `json:"duration"`     // 时长，单位秒。
	PublishedAt  time.Time     `json:"publishedAt"`  // 原站发布时间，UTC。
	CollectedAt  time.Time     `json:"collectedAt"`  // 抓取时间，UTC，重放保持不变。
	Parts        []HGVideoPart `json:"parts"`        // 有序分 P，最多 1000 项。
	Stats        HGVideoStats  `json:"stats"`        // 同次采集的累计统计快照。
}

// HGVideoPart 保存官方播放器定位信息，不保存临时 CDN 地址。
type HGVideoPart struct {
	CID      string `json:"cid"`      // 原站分 P 编号，十进制字符串。
	Page     int    `json:"page"`     // 从 1 开始的连续序号。
	Title    string `json:"title"`    // 分 P 标题，最多 255 字符。
	Duration int64  `json:"duration"` // 分 P 时长，单位秒。
}

// HGVideoStats 是累计值，每次覆盖最新快照，禁止累加上游计数。
type HGVideoStats struct {
	View     int64 `json:"view"`     // 播放次数。
	Like     int64 `json:"like"`     // 点赞次数。
	Coin     int64 `json:"coin"`     // 投币枚数。
	Favorite int64 `json:"favorite"` // 收藏次数。
	Share    int64 `json:"share"`    // 分享次数。
	Reply    int64 `json:"reply"`    // 评论条数。
	Danmaku  int64 `json:"danmaku"`  // 弹幕条数。
}
