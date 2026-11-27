package hgerrors

const (
	HGVideoNotFound              Code = 1601001
	HGVideoNotPublished          Code = 1601002
	HGVideoAccessDenied          Code = 1601003
	HGVideoPlaybackUnavailable   Code = 1601004
	HGVideoAlreadyLiked          Code = 1604001
	HGVideoFavoriteLimitExceeded Code = 1604002
	HGVideoTranscodeFailed       Code = 1605002
)

var hgVideo = []Definition{
	{HGVideoNotFound, "VIDEO_NOT_FOUND", "视频不存在", 404, false},
	{HGVideoNotPublished, "VIDEO_NOT_PUBLISHED", "视频尚未发布", 404, false},
	{HGVideoAccessDenied, "VIDEO_ACCESS_DENIED", "无权观看视频", 403, false},
	{HGVideoPlaybackUnavailable, "VIDEO_PLAYBACK_UNAVAILABLE", "视频暂不可播放", 503, true},
	{HGVideoAlreadyLiked, "VIDEO_ALREADY_LIKED", "已点赞", 409, false},
	{HGVideoFavoriteLimitExceeded, "VIDEO_FAVORITE_LIMIT_EXCEEDED", "收藏数量超限", 409, false},
	{HGVideoTranscodeFailed, "VIDEO_TRANSCODE_FAILED", "视频转码失败", 500, false},
}
