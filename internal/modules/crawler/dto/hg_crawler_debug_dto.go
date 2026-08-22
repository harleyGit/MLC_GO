package dto

// HGDebugRequest 描述一次不落库的受控 HTTP 采集测试。
type HGDebugRequest struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	Body      string            `json:"body,omitempty"`
	TimeoutMS int               `json:"timeoutMs,omitempty"`
}

// HGDetectedField 是从 JSON 响应样本中识别出的叶子字段。
type HGDetectedField struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Sample     any    `json:"sample,omitempty"`
	SampleType string `json:"sampleType"`
}

// HGDebugResponse 返回安全裁剪后的上游响应和字段建议。
type HGDebugResponse struct {
	StatusCode    int               `json:"statusCode"`
	Status        string            `json:"status"`
	Headers       map[string]string `json:"headers"`
	ContentType   string            `json:"contentType"`
	Body          any               `json:"body,omitempty"`
	BodyText      string            `json:"bodyText,omitempty"`
	ResponseBytes int               `json:"responseBytes"`
	CostMillis    int64             `json:"costMillis"`
	Detected      []HGDetectedField `json:"detectedFields"`
}
