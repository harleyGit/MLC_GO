package VideoInteractionDtoPackage

// ActionRequest 是点赞、投币、收藏和分享接口的请求体。
type ActionRequest struct {
	SubmissionID string `json:"submissionId"`
	Action       string `json:"-"`
	Active       bool   `json:"active"`
	Quantity     int    `json:"quantity,omitempty"`
}

// FollowRequest 是关注接口请求体，当前用户由 JWT 上下文提供。
type FollowRequest struct {
	FolloweeID string `json:"followeeId"`
	Active     bool   `json:"active"`
}

// AcceptedResponse 表示命令已被 Kafka 可靠接收，持久化最终一致完成。
type AcceptedResponse struct {
	Accepted bool   `json:"accepted"`
	Action   string `json:"action"`
	Active   bool   `json:"active,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
}

// StateResponse 返回详情页所需的互动状态和实时计数。
type StateResponse struct {
	SubmissionID  string `json:"submissionId"`
	AuthorID      string `json:"authorId,omitempty"`
	Liked         bool   `json:"liked"`
	Favorited     bool   `json:"favorited"`
	Followed      bool   `json:"followed"`
	CoinCount     int64  `json:"coinCount"`
	UserCoinCount int64  `json:"userCoinCount"`
	LikeCount     int64  `json:"likeCount"`
	FavoriteCount int64  `json:"favoriteCount"`
	ShareCount    int64  `json:"shareCount"`
	FollowerCount int64  `json:"followerCount"`
}
