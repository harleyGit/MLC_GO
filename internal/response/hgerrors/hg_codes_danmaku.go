package hgerrors

const (
	HGVideoDanmakuDisabled    Code = 1603001
	HGVideoDanmakuTooLong     Code = 1603002
	HGVideoDanmakuRateLimited Code = 1603003
)

var hgDanmaku = []Definition{
	{HGVideoDanmakuDisabled, "VIDEO_DANMAKU_DISABLED", "视频禁止弹幕", 403, false},
	{HGVideoDanmakuTooLong, "VIDEO_DANMAKU_TOO_LONG", "弹幕超长", 400, false},
	{HGVideoDanmakuRateLimited, "VIDEO_DANMAKU_RATE_LIMITED", "弹幕频率超限", 429, true},
}
