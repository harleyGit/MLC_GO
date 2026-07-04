package VideoUploadDtoPackage

import (
	"encoding/json"

	hg_time "MLC_GO/internal/pkg/hg_time"
)

// UploadVideoResponse 是单个视频文件上传成功后的响应。
// 前端会把这里返回的 submissionId/videoId 写入页面状态，后续保存草稿或提交审核时再带回来。
type UploadVideoResponse struct {
	// SubmissionID 稿件业务 ID；同一次多 P 投稿下的多个视频共享同一个 submissionId。
	SubmissionID string `json:"submissionId"`
	// VideoID 视频业务 ID；用于后续更新该视频的标题、封面、分区、标签等配置。
	VideoID string `json:"videoId"`
	// FileName 用户上传时的原始文件名，用于页面展示和问题排查。
	FileName string `json:"fileName"`
	// FilePath 服务端保存后的访问路径；当前为本地 uploads 路径，后续可替换成对象存储 key。
	FilePath string `json:"filePath"`
	// FileURL 给前端直接预览/展示用的 URL；当前和 FilePath 保持一致。
	FileURL string `json:"fileUrl"`
	// FileSize 视频文件大小，单位字节。
	FileSize int64 `json:"fileSize"`
	// MimeType 上传请求里携带的 MIME 类型，例如 video/mp4。
	MimeType string `json:"mimeType"`
	// MD5 文件内容摘要，用于重复上传识别、排查和后续秒传能力预留。
	MD5 string `json:"md5"`
	// PartNumber 分 P 序号，从 1 开始。
	PartNumber uint32 `json:"partNumber"`
}

// SaveSubmissionRequest 是保存草稿和提交审核共用的稿件请求体。
// Status 不由前端决定，handler 会根据 /draft 或 /submit 路由覆盖为 draft/reviewing。
type SaveSubmissionRequest struct {
	// SubmissionID 稿件业务 ID，必须来自上传接口返回值。
	SubmissionID string `json:"submissionId"`
	// Title 稿件标题，默认取第一个视频标题，最长 80 字。
	Title string `json:"title"`
	// CoverURL 稿件封面 URL；可以来自视频帧截图或前端已有封面地址。
	CoverURL string `json:"coverUrl"`
	// Category 稿件分区，默认取第一个视频分区。
	Category string `json:"category"`
	// VideoType 稿件类型：自制/转载。
	VideoType string `json:"videoType"`
	// SourceURL 转载来源 URL；当 VideoType 为"转载"时业务上必填。
	SourceURL string `json:"sourceUrl"`
	// Description 稿件简介。
	Description string `json:"description"`
	// AllowSecondaryCreation 是否允许二创。
	AllowSecondaryCreation bool `json:"allowSecondaryCreation"`
	// Watermark 是否为本次投稿添加水印。
	Watermark bool `json:"watermark"`
	// Visibility 可见范围：public 公开，private 仅自己可见。
	Visibility string `json:"visibility"`
	// Declaration 创作声明，例如 ai/danger/entertainment/uncomfortable/consume/personal。
	Declaration string `json:"declaration"`
	// CardConfig 个性化卡片配置，使用 JSON 方便后续扩展卡片类型和展示参数。
	CardConfig map[string]interface{} `json:"cardConfig"`
	// DolbyAudio 是否启用杜比音效。
	DolbyAudio bool `json:"dolbyAudio"`
	// HiresAudio 是否启用 Hi-Res 无损音质。
	HiresAudio bool `json:"hiResAudio"`
	// CloseDanmaku 是否关闭弹幕。
	CloseDanmaku bool `json:"closeDanmaku"`
	// CloseComment 是否关闭评论。
	CloseComment bool `json:"closeComment"`
	// FeaturedComment 是否开启精选评论。
	FeaturedComment bool `json:"featuredComment"`
	// DynamicDescription 粉丝动态描述，最长 233 字。
	DynamicDescription string `json:"dynamicDescription"`
	// HideFromProfile 是否在个人空间-投稿中隐藏。
	HideFromProfile bool `json:"hideFromProfile"`
	// Status 稿件状态；由 handler 写入，service 只校验允许 draft/reviewing。
	Status string `json:"status"`
	// Schedule 定时发布配置；未开启时可以为空或 enabled=false。
	Schedule *ScheduleRequest `json:"schedule"`
	// Commercial 商业推广配置；未开启时可以为空或 enabled=false。
	Commercial *CommercialRequest `json:"commercial"`
	// Videos 本次稿件包含的视频/分 P 配置列表。
	Videos []VideoConfigRequest `json:"videos"`
}

// VideoConfigRequest 描述单个视频/分 P 的独立投稿配置。
// 这些字段最终写入 video_files 和 video_tags。
type VideoConfigRequest struct {
	// VideoID 上传接口返回的视频业务 ID。
	VideoID string `json:"videoId"`
	// PartNumber 分 P 序号，从 1 开始；为空时 service 会按数组顺序补齐。
	PartNumber uint32 `json:"partNumber"`
	// Title 单个视频标题，最长 80 字。
	Title string `json:"title"`
	// CoverURL 单个视频封面 URL。
	CoverURL string `json:"coverUrl"`
	// VideoType 视频类型：自制/转载。
	VideoType string `json:"videoType"`
	// SourceURL 转载来源 URL。
	SourceURL string `json:"sourceUrl"`
	// Category 视频分区。
	Category string `json:"category"`
	// Description 视频简介。
	Description string `json:"description"`
	// Tags 视频标签，至少 1 个，最多 7 个。
	Tags []string `json:"tags"`
}

// ScheduleRequest 描述稿件级定时发布配置。
type ScheduleRequest struct {
	// Enabled 是否开启定时发布。
	Enabled bool `json:"enabled"`
	// ScheduledTime 客户端显式标记格式和时区的定时发布时间。
	ScheduledTime *hg_time.ClientTime `json:"scheduledTime"`
}

// UnmarshalJSON 自定义反序列化，兼容前端传空字符串 "" 或 null 的场景。
// 前端 form 表单清空时 scheduledTime 会传 ""，但 Go 结构体期望对象或 null。
// 使用 rawMessage 逐字段处理，scheduledTime 单独容错。
func (s *ScheduleRequest) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}

	//map[string]json.RawMessage :key 是字符串，value 是 json.RawMessage 类型。
	// json.RawMessage 本质：[]byte 的别名，专门用来暂存未解析的原始 JSON 片段，不立刻内部解析
	var raw map[string]json.RawMessage
	// 把顶层 JSON 对象解析成一个 map，每个字段不做深层解析，只原样保留原始 JSON 字节。
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if v, ok := raw["enabled"]; ok {
		_ = json.Unmarshal(v, &s.Enabled)
	}

	if v, ok := raw["scheduledTime"]; ok {
		str := string(v)
		if str != "null" && str != `""` {
			var ct hg_time.ClientTime
			if err := json.Unmarshal(v, &ct); err == nil {
				s.ScheduledTime = &ct
			}
		}
	}

	return nil
}

// CommercialRequest 描述稿件级商业推广配置。
// 需求约束为单稿件仅支持一种商业推广信息。
type CommercialRequest struct {
	// Enabled 是否填写商业推广信息。
	Enabled bool `json:"enabled"`
	// PromotionType 推广类型，例如手机游戏、通用行业、PC网络游戏。
	PromotionType string `json:"promotionType"`
	// PromotionName 推广名称。
	PromotionName string `json:"promotionName"`
	// PromotionForm 推广形式，例如口播、贴片、Logo、二维码。
	PromotionForm string `json:"promotionForm"`
}

// SaveSubmissionResponse 是保存草稿或提交审核后的统一响应。
type SaveSubmissionResponse struct {
	// SubmissionID 稿件业务 ID。
	SubmissionID string `json:"submissionId"`
	// Status 保存后的稿件状态：draft 或 reviewing。
	Status string `json:"status"`
	// VideoCount 本次稿件包含的视频数量。
	VideoCount int `json:"videoCount"`
}

// GetVideoListRequest 获取视频列表的请求参数。
type GetVideoListRequest struct {
	// Cursor 翻页游标，首次调用传空，后续使用响应中的 NextCursor。
	Cursor string `json:"cursor"`
	// PageSize 每页数量，默认 20，最大 100。
	PageSize int `json:"pageSize"`
}

// GetVideoListResponse 获取视频列表的响应。
type GetVideoListResponse struct {
	// Total 视频总数（Redis 缓存，60s 刷新一次）。
	Total int `json:"total"`
	// PageSize 每页数量。
	PageSize int `json:"pageSize"`
	// HasMore 是否还有下一页。
	HasMore bool `json:"hasMore"`
	// NextCursor 下一页游标，传入下次请求的 Cursor 字段；为空表示没有更多数据。
	NextCursor string `json:"nextCursor,omitempty"`
	// Videos 视频列表。
	Videos []VideoListItem `json:"videos"`
}

// VideoListItem 视频列表项，包含稿件级信息和第一个分 P 的视频文件信息。
type VideoListItem struct {
	// SubmissionID 稿件业务 ID。
	SubmissionID string `json:"submissionId"`
	// UserID 作者用户 ID。
	UserID string `json:"userId"`
	// Title 稿件标题。
	Title string `json:"title"`
	// CoverURL 稿件封面 URL。
	CoverURL string `json:"coverUrl"`
	// Category 稿件分区。
	Category string `json:"category"`
	// VideoType 稿件类型：自制/转载。
	VideoType string `json:"videoType"`
	// Description 稿件简介。
	Description string `json:"description"`
	// Visibility 可见范围：public/private。
	Visibility string `json:"visibility"`
	// Status 稿件状态：draft/reviewing/published。
	Status string `json:"status"`
	// VideoCount 稿件包含的视频数量。
	VideoCount int `json:"videoCount"`
	// TotalSize 稿件总大小，单位字节。
	TotalSize int64 `json:"totalSize"`
	// SubmitTime 提交审核时间。
	SubmitTime string `json:"submitTime"`
	// CreatedAt 创建时间。
	CreatedAt string `json:"createdAt"`
	// VideoID 第一个分 P 的视频 ID。
	VideoID string `json:"videoId"`
	// FilePath 视频文件访问路径。
	FilePath string `json:"filePath"`
	// FileName 原始文件名。
	FileName string `json:"fileName"`
	// FileSize 视频文件大小，单位字节。
	FileSize int64 `json:"fileSize"`
	// MimeType 视频 MIME 类型。
	MimeType string `json:"mimeType"`
	// PartNumber 分 P 序号。
	PartNumber uint32 `json:"partNumber"`
}
