package service

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const (
	hgDebugMaxFields = 64
	hgDebugMaxDepth  = 12
)

var (
	// ErrHGDebugInvalidRequest 表示采集测试配置不合法。
	ErrHGDebugInvalidRequest = ErrHGCrawlerInvalidRequest
	// ErrHGDebugUnsafeTarget 表示目标地址可能访问服务端内网或保留地址。
	ErrHGDebugUnsafeTarget = ErrHGCrawlerUnsafeTarget
)

// HGDebugService 执行不落库、禁止重定向且有响应大小上限的采集测试。
type HGDebugService struct {
	http *HGSafeHTTPService
}

// NewHGDebugService 创建带 SSRF 防护的采集测试服务。
func NewHGDebugService(services ...*HGSafeHTTPService) *HGDebugService {
	if len(services) > 0 {
		return &HGDebugService{http: services[0]}
	}
	policy, _ := NewHGTargetPolicy([]string{"api.bilibili.com"}, false)
	httpService, _ := NewHGSafeHTTPService(policy, "MLC-GO-Crawler-Debug/1.0")
	return &HGDebugService{http: httpService}
}

// TestRequest 执行一次采集测试，并对 JSON 响应自动识别叶子字段路径。
func (s *HGDebugService) TestRequest(ctx context.Context, req CrawlerDtoPackage.HGDebugRequest) (CrawlerDtoPackage.HGDebugResponse, error) {
	if s == nil || s.http == nil {
		return CrawlerDtoPackage.HGDebugResponse{}, errors.New("采集测试服务未初始化")
	}
	response, err := s.http.Execute(ctx, req)
	if err != nil {
		return CrawlerDtoPackage.HGDebugResponse{}, err
	}

	result := CrawlerDtoPackage.HGDebugResponse{
		StatusCode: response.StatusCode, Status: response.Status,
		Headers: hgDebugResponseHeaders(response.Header), ContentType: response.Header.Get("Content-Type"),
		ResponseBytes: len(response.Body), CostMillis: response.Cost.Milliseconds(),
		Detected: make([]CrawlerDtoPackage.HGDetectedField, 0),
	}
	var payload any
	decoder := json.NewDecoder(strings.NewReader(string(response.Body)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err == nil {
		result.Body = payload
		hgDetectDebugFields(payload, "$", 0, &result.Detected)
	} else {
		result.BodyText = string(response.Body)
	}
	return result, nil
}

func hgDebugResponseHeaders(headers http.Header) map[string]string {
	allowed := []string{"Content-Type", "Content-Length", "Cache-Control", "ETag", "Last-Modified"}
	result := make(map[string]string, len(allowed))
	for _, key := range allowed {
		if value := headers.Get(key); value != "" {
			result[strings.ToLower(key)] = value
		}
	}
	return result
}

func hgDetectDebugFields(value any, path string, depth int, fields *[]CrawlerDtoPackage.HGDetectedField) {
	if depth > hgDebugMaxDepth || len(*fields) >= hgDebugMaxFields {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			hgDetectDebugFields(typed[key], path+"."+key, depth+1, fields)
			if len(*fields) >= hgDebugMaxFields {
				return
			}
		}
	case []any:
		if len(typed) > 0 {
			hgDetectDebugFields(typed[0], path+"[*]", depth+1, fields)
		}
	default:
		name := path[strings.LastIndex(path, ".")+1:]
		*fields = append(*fields, CrawlerDtoPackage.HGDetectedField{Name: strings.TrimSuffix(name, "[*]"), Path: path, Sample: typed, SampleType: fmt.Sprintf("%T", typed)})
	}
}
