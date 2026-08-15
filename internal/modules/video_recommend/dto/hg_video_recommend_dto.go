package VideoRecommendDtoPackage

// HGFeedItem 是推荐首页返回的视频卡片及推荐上下文。
type HGFeedItem struct {
	SubmissionID  string `json:"submissionId"`
	VideoID       string `json:"videoId"`
	AuthorID      string `json:"authorId"`
	Title         string `json:"title"`
	CoverURL      string `json:"coverUrl"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Duration      int64  `json:"duration"`
	FilePath      string `json:"filePath"`
	PublishTime   string `json:"publishTime"`
	LikeCount     int64  `json:"likeCount"`
	CoinCount     int64  `json:"coinCount"`
	FavoriteCount int64  `json:"favoriteCount"`
	ShareCount    int64  `json:"shareCount"`
	Reason        string `json:"reason"`
}

// HGFeedResponse 是认证用户的推荐流游标页。
type HGFeedResponse struct {
	Generation string       `json:"generation"`
	PageSize   int          `json:"pageSize"`
	HasMore    bool         `json:"hasMore"`
	NextCursor string       `json:"nextCursor,omitempty"`
	Items      []HGFeedItem `json:"items"`
}
