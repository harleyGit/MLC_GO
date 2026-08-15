package VideoRecommendModelPackage

// HGCandidate 是从 Redis Feed 分片召回的轻量候选，只在推荐模块内部流转。
type HGCandidate struct {
	SubmissionID string
	Score        int64
}

// HGCursor 使用全局 score + submission_id 复合边界稳定翻页，不依赖 OFFSET。
type HGCursor struct {
	Generation   string `json:"g"`
	Score        int64  `json:"s"`
	SubmissionID string `json:"id"`
}
