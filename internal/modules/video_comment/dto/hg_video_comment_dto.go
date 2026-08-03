package VideoCommentDtoPackage

// CreateRequest 创建当前用户的顶级视频评论；requestId 在用户维度保证幂等。
type CreateRequest struct {
	SubmissionID string `json:"submissionId"`
	Content      string `json:"content"`
	RequestID    string `json:"requestId"`
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

// CommentResponse 是视频评论 API 的公开评论结构。
type CommentResponse struct {
	CommentID    string `json:"commentId"`
	SubmissionID string `json:"submissionId"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	AvatarURL    string `json:"avatarURL"`
	Content      string `json:"content"`
	LikeCount    uint64 `json:"likeCount"`
	ReplyCount   uint64 `json:"replyCount"`
	CreatedAt    string `json:"createdAt"`
	CanDelete    bool   `json:"canDelete"`
}

// ListResponse 返回有上限的评论页和下一页不透明游标。
type ListResponse struct {
	Comments   []CommentResponse `json:"comments"`
	NextCursor string            `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
}

// DeleteResponse 表示评论已完成软删除。
type DeleteResponse struct {
	Deleted   bool   `json:"deleted"`
	CommentID string `json:"commentId"`
}
