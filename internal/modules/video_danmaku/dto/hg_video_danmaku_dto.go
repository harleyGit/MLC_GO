package VideoDanmakuDtoPackage

// CreateRequest 创建绑定到具体分 P 和播放毫秒位置的滚动弹幕。
type CreateRequest struct {
	VideoID    string `json:"videoId"`
	Content    string `json:"content"`
	RequestID  string `json:"requestId"`
	ProgressMS uint32 `json:"progressMs"`
	Mode       string `json:"mode,omitempty"`
	Color      string `json:"color,omitempty"`
	FontSize   uint8  `json:"fontSize,omitempty"`
}

// ListRequest 使用有界时间窗和不透明 keyset 游标读取弹幕。
type ListRequest struct {
	VideoID      string
	Cursor       string
	FromMS, ToMS uint32
	PageSize     int
}

// DanmakuResponse 是 HTTP 与 WebSocket 共享的公开弹幕协议。
type DanmakuResponse struct {
	DanmakuID    string `json:"danmakuId"`
	SubmissionID string `json:"submissionId"`
	VideoID      string `json:"videoId"`
	Content      string `json:"content"`
	ProgressMS   uint32 `json:"progressMs"`
	Mode         string `json:"mode"`
	Color        string `json:"color"`
	FontSize     uint8  `json:"fontSize"`
	CreatedAt    string `json:"createdAt"`
}

// ListResponse 返回有上限的时间窗弹幕页和预聚合总数。
type ListResponse struct {
	Danmaku    []DanmakuResponse `json:"danmaku"`
	NextCursor string            `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
	TotalCount uint64            `json:"totalCount"`
}

// TicketRequest 为当前用户和视频申请短期单次 WebSocket 票据。
type TicketRequest struct {
	VideoID string `json:"videoId"`
}

// TicketResponse 不返回长期 JWT；客户端只在 WebSocket 握手中使用一次。
type TicketResponse struct {
	Ticket        string `json:"ticket"`
	WebSocketPath string `json:"webSocketPath"`
	ExpiresIn     int64  `json:"expiresIn"`
}
