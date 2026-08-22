package parser

import (
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// TypeRestrictedJSONPath selects the restricted, non-executable JSONPath parser.
	TypeRestrictedJSONPath Type = "restricted_jsonpath"
	// TypeCSS selects the goquery CSS parser.
	TypeCSS Type = "css"
	// TypeXPath selects the htmlquery XPath parser.
	TypeXPath Type = "xpath"

	hgMaxItems  = 50
	hgMaxFields = 32
)

// Type identifies one of the supported configurable parser implementations.
type Type string

// FieldConfig maps one canonical recommendation field to a relative expression.
// Attribute is used by CSS and XPath mappings; an empty value extracts node text.
type FieldConfig struct {
	Selector  string `json:"selector"`
	Attribute string `json:"attribute,omitempty"`
}

// Config describes how response items and their canonical fields are selected.
// ItemSelector is evaluated from the document root and field selectors from each item.
type Config struct {
	Type         Type                   `json:"type"`
	Platform     string                 `json:"platform"`
	ItemSelector string                 `json:"itemSelector"`
	Fields       map[string]FieldConfig `json:"fields"`
}

// Validate checks parser type, item selector, canonical fields, and required mappings without parsing a response.
func Validate(config Config) error {
	return hgValidateConfig(config)
}

// Parse maps a JSON or HTML response into at most 50 canonical recommendations.
func Parse(config Config, sourceURL string, body []byte) ([]CrawlerPlatformPackage.HGRecommendation, error) {
	if err := hgValidateConfig(config); err != nil {
		return nil, err
	}

	var rows []map[string]string
	var err error
	switch config.Type {
	case TypeRestrictedJSONPath:
		rows, err = hgParseJSON(config, body)
	case TypeCSS:
		rows, err = hgParseCSS(config, body)
	case TypeXPath:
		rows, err = hgParseXPath(config, body)
	default:
		return nil, fmt.Errorf("unsupported crawler parser type %q", config.Type)
	}
	if err != nil {
		return nil, err
	}

	items := make([]CrawlerPlatformPackage.HGRecommendation, 0, min(len(rows), hgMaxItems))
	seen := make(map[string]struct{}, min(len(rows), hgMaxItems))
	for index, row := range rows {
		item, err := hgMapRecommendation(config.Platform, sourceURL, row)
		if err != nil {
			return nil, fmt.Errorf("map crawler item %d: %w", index, err)
		}
		if _, exists := seen[item.ContentID]; exists {
			continue
		}
		seen[item.ContentID] = struct{}{}
		items = append(items, item)
		if len(items) == hgMaxItems {
			break
		}
	}
	return items, nil
}

func hgValidateConfig(config Config) error {
	if config.Type != TypeRestrictedJSONPath && config.Type != TypeCSS && config.Type != TypeXPath {
		return fmt.Errorf("unsupported crawler parser type %q", config.Type)
	}
	if strings.TrimSpace(config.ItemSelector) == "" {
		return errors.New("crawler item selector is required")
	}
	if len(config.Fields) > hgMaxFields {
		return fmt.Errorf("crawler field count must not exceed %d", hgMaxFields)
	}
	allowed := map[string]bool{
		"contentId": true, "title": true, "authorId": true,
		"authorName": true, "coverUrl": true, "targetUrl": true,
		"durationSeconds": true, "viewCount": true, "likeCount": true,
		"commentCount": true, "publishedAt": true,
	}
	for name, field := range config.Fields {
		if !allowed[name] {
			return fmt.Errorf("unsupported crawler canonical field %q", name)
		}
		if strings.TrimSpace(field.Selector) == "" {
			return fmt.Errorf("crawler field %q selector is required", name)
		}
	}
	return nil
}

func hgMapRecommendation(platformName, sourceURL string, row map[string]string) (CrawlerPlatformPackage.HGRecommendation, error) {
	item := CrawlerPlatformPackage.HGRecommendation{
		Platform:   strings.TrimSpace(platformName),
		ContentID:  strings.TrimSpace(row["contentId"]),
		Title:      strings.TrimSpace(row["title"]),
		AuthorID:   strings.TrimSpace(row["authorId"]),
		AuthorName: strings.TrimSpace(row["authorName"]),
	}
	raw := strings.TrimSpace(row["__raw"])
	if item.ContentID == "" {
		digest := sha256.Sum256([]byte(sourceURL + "\x00" + raw))
		item.ContentID = fmt.Sprintf("raw-%x", digest[:12])
	}
	if item.Title == "" {
		item.Title = hgBoundCrawlerText(raw, 255)
		if item.Title == "" {
			item.Title = hgBoundCrawlerText(sourceURL, 255)
		}
	}
	targetURL := strings.TrimSpace(row["targetUrl"])
	if targetURL == "" && strings.EqualFold(item.Platform, "bilibili") && strings.HasPrefix(strings.ToUpper(item.ContentID), "BV") {
		targetURL = "https://www.bilibili.com/video/" + item.ContentID
	}
	if targetURL == "" {
		targetURL = sourceURL
	}

	var err error
	if item.TargetURL, err = hgResolveBrowserURL(sourceURL, targetURL); err != nil {
		return item, fmt.Errorf("invalid targetUrl: %w", err)
	}
	if strings.TrimSpace(row["coverUrl"]) != "" {
		if item.CoverURL, err = hgResolveBrowserURL(sourceURL, row["coverUrl"]); err != nil {
			return item, fmt.Errorf("invalid coverUrl: %w", err)
		}
	}
	for name, target := range map[string]*int64{
		"durationSeconds": &item.Duration,
		"viewCount":       &item.ViewCount,
		"likeCount":       &item.LikeCount,
		"commentCount":    &item.CommentCount,
	} {
		if *target, err = hgParseNonnegativeInt(row[name]); err != nil {
			return item, fmt.Errorf("invalid %s: %w", name, err)
		}
	}
	if item.PublishedAt, err = hgParsePublishedAt(row["publishedAt"]); err != nil {
		return item, fmt.Errorf("invalid publishedAt: %w", err)
	}
	return item, nil
}

func hgBoundCrawlerText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maximum {
		return value
	}
	for maximum > 0 && (value[maximum]&0xc0) == 0x80 {
		maximum--
	}
	return value[:maximum]
}

func hgParseNonnegativeInt(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > math.MaxInt64 || math.Trunc(number) != number {
		return 0, errors.New("must be a nonnegative integer")
	}
	return int64(number), nil
}

func hgParsePublishedAt(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC(), nil
	}
	timestamp, err := strconv.ParseInt(value, 10, 64)
	if err != nil || timestamp < 0 {
		return time.Time{}, errors.New("must be RFC3339 or nonnegative unix seconds/milliseconds")
	}
	seconds, nanoseconds := timestamp, int64(0)
	if timestamp >= 1_000_000_000_000 {
		seconds = timestamp / 1000
		nanoseconds = timestamp % 1000 * int64(time.Millisecond)
	}
	return time.Unix(seconds, nanoseconds).UTC(), nil
}

func hgResolveBrowserURL(sourceURL, value string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return "", errors.New("source URL must be absolute HTTP or HTTPS")
	}
	reference, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", errors.New("URL cannot be parsed")
	}
	resolved := base.ResolveReference(reference)
	if resolved.Host == "" || (resolved.Scheme != "http" && resolved.Scheme != "https") {
		return "", errors.New("only HTTP and HTTPS URLs are allowed")
	}
	return resolved.String(), nil
}
