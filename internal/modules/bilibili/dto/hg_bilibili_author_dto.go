package BilibiliDtoPackage

// HGAuthorProfileResponse 是作者公开资料，不包含邮箱、手机号等账号安全字段。
type HGAuthorProfileResponse struct {
	UserID      string `json:"userId"`
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
	Signature   string `json:"signature"`
	Gender      uint8  `json:"gender"`
	AvatarURL   string `json:"avatarUrl"`
	CreatedAt   string `json:"createdAt"`
}

// HGAuthorStatsResponse 是作者主页使用的预聚合/有界统计数据。
type HGAuthorStatsResponse struct {
	FollowingCount int64 `json:"followingCount"`
	FollowerCount  int64 `json:"followerCount"`
	VideoCount     int64 `json:"videoCount"`
}

// HGAuthorVideoItem 是作者公开视频卡片。
type HGAuthorVideoItem struct {
	SubmissionID  string `json:"submissionId"`
	VideoID       string `json:"videoId"`
	UserID        string `json:"userId"`
	Title         string `json:"title"`
	CoverURL      string `json:"coverUrl"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Duration      uint32 `json:"duration"`
	FilePath      string `json:"filePath"`
	PublishTime   string `json:"publishTime"`
	LikeCount     int64  `json:"likeCount"`
	CoinCount     int64  `json:"coinCount"`
	FavoriteCount int64  `json:"favoriteCount"`
	ShareCount    int64  `json:"shareCount"`
}

// HGAuthorVideoListResponse 使用复合游标分页，避免大数据量下 OFFSET 深分页。
type HGAuthorVideoListResponse struct {
	PageSize   int                 `json:"pageSize"`
	HasMore    bool                `json:"hasMore"`
	NextCursor string              `json:"nextCursor,omitempty"`
	Videos     []HGAuthorVideoItem `json:"videos"`
}

// HGAuthorHomepageResponse 是作者空间首屏聚合结果；后续翻页只请求 videos 接口。
type HGAuthorHomepageResponse struct {
	Profile HGAuthorProfileResponse   `json:"profile"`
	Stats   HGAuthorStatsResponse     `json:"stats"`
	Videos  HGAuthorVideoListResponse `json:"videos"`
}
