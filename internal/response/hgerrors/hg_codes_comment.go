package hgerrors

const (
	HGVideoCommentDisabled    Code = 1602001
	HGVideoCommentNotFound    Code = 1602002
	HGVideoCommentTooLong     Code = 1602003
	HGVideoCommentRateLimited Code = 1602004
)

var hgComment = []Definition{
	{HGVideoCommentDisabled, "VIDEO_COMMENT_DISABLED", "视频禁止评论", 403, false},
	{HGVideoCommentNotFound, "VIDEO_COMMENT_NOT_FOUND", "评论不存在", 404, false},
	{HGVideoCommentTooLong, "VIDEO_COMMENT_TOO_LONG", "评论超长", 400, false},
	{HGVideoCommentRateLimited, "VIDEO_COMMENT_RATE_LIMITED", "评论频率超限", 429, true},
}
