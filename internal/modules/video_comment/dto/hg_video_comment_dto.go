package VideoCommentDtoPackage

// CreateRequest 创建当前用户的视频评论；parentCommentId 为空时创建顶级评论。
type CreateRequest struct {
	SubmissionID    string   `json:"submissionId"`
	Content         string   `json:"content"`
	RequestID       string   `json:"requestId"`
	ParentCommentID string   `json:"parentCommentId,omitempty"`
	ImageURLs       []string `json:"imageURLs,omitempty"`
}

// DeleteRequest 软删除当前用户自己的评论。
type DeleteRequest struct {
	CommentID string `json:"commentId"`
}

// ListRequest 使用不透明游标按 latest 或 hot 排序读取顶级评论。
type ListRequest struct {
	SubmissionID string
	Sort         string
	Cursor       string
	PageSize     int
}

// RepliesRequest 使用 (created_at,id) 游标按时间正序读取根评论的可见回复。
type RepliesRequest struct {
	RootCommentID string
	Cursor        string
	PageSize      int
}

// ReactionRequest 将当前用户对评论的关系设置为最终状态。
type ReactionRequest struct {
	CommentID string `json:"commentId"`
	Reaction  string `json:"reaction"`
}

// CommentResponse 是视频评论 API 的公开评论结构。
type CommentResponse struct {
	CommentID       string   `json:"commentId"`
	SubmissionID    string   `json:"submissionId"`
	UserID          string   `json:"userId"`
	UserName        string   `json:"userName"`
	AvatarURL       string   `json:"avatarURL"`
	Content         string   `json:"content"`
	LikeCount       uint64   `json:"likeCount"`
	DislikeCount    uint64   `json:"dislikeCount"`
	ReplyCount      uint64   `json:"replyCount"`
	Reaction        string   `json:"reaction"`
	ImageURLs       []string `json:"imageURLs"`
	RootCommentID   string   `json:"rootCommentId,omitempty"`
	ParentCommentID string   `json:"parentCommentId,omitempty"`
	ReplyToUserID   string   `json:"replyToUserId,omitempty"`
	CreatedAt       string   `json:"createdAt"`
	CanDelete       bool     `json:"canDelete"`
}

// ListResponse 返回有上限的评论页和下一页不透明游标。
type ListResponse struct {
	Comments   []CommentResponse `json:"comments"`
	NextCursor string            `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
	TotalCount uint64            `json:"totalCount"`
}

// RepliesResponse 返回回复页，totalCount 直接使用根评论维护的 reply_count。
type RepliesResponse struct {
	Comments   []CommentResponse `json:"comments"`
	NextCursor string            `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
	TotalCount uint64            `json:"totalCount"`
}

// ReactionResponse 返回最终关系和事务内更新后的权威计数。
type ReactionResponse struct {
	CommentID    string `json:"commentId"`
	Reaction     string `json:"reaction"`
	LikeCount    uint64 `json:"likeCount"`
	DislikeCount uint64 `json:"dislikeCount"`
}

// ImageResponse 返回可用于创建评论的图片 URL。
type ImageResponse struct {
	ImageURL string `json:"imageURL"`
}

// DeleteResponse 表示评论已完成软删除。
type DeleteResponse struct {
	Deleted   bool   `json:"deleted"`
	CommentID string `json:"commentId"`
}
