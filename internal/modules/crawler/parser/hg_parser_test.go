package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseRestrictedJSONPath(t *testing.T) {
	config := Config{
		Type: TypeRestrictedJSONPath, Platform: "example", ItemSelector: `$.data['items'][*]`,
		Fields: map[string]FieldConfig{
			"contentId":       {Selector: `$.id`},
			"title":           {Selector: `$["display.title"]`},
			"targetUrl":       {Selector: `$.links[0]`},
			"coverUrl":        {Selector: `$.cover`},
			"viewCount":       {Selector: `$.views`},
			"publishedAt":     {Selector: `$.published`},
			"durationSeconds": {Selector: `$.duration`},
		},
	}
	body := []byte(`{"data":{"items":[{"id":"one","display.title":"First","links":["/watch/one"],"cover":"//cdn.example/one.jpg","views":12,"duration":"30","published":"2025-01-02T03:04:05+08:00"},{"id":"one","display.title":"Duplicate","links":["/duplicate"]}]}}`)

	items, err := Parse(config, "https://www.example/base/list", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}
	item := items[0]
	if item.ContentID != "one" || item.Title != "First" || item.TargetURL != "https://www.example/watch/one" || item.CoverURL != "https://cdn.example/one.jpg" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.ViewCount != 12 || item.Duration != 30 || !item.PublishedAt.Equal(time.Date(2025, 1, 1, 19, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected normalized values: %#v", item)
	}
}

func TestParseCSSAndXPath(t *testing.T) {
	html := []byte(`<html><body><article data-id="a"><h2>Alpha</h2><a href="/a">Open</a><time>1700000000000</time></article></body></html>`)
	tests := []struct {
		name   string
		config Config
	}{
		{name: "css", config: Config{Type: TypeCSS, ItemSelector: "article", Fields: map[string]FieldConfig{
			"contentId": {Selector: "article", Attribute: "data-id"}, "title": {Selector: "h2"},
			"targetUrl": {Selector: "a", Attribute: "href"}, "publishedAt": {Selector: "time"},
		}}},
		{name: "xpath", config: Config{Type: TypeXPath, ItemSelector: "//article", Fields: map[string]FieldConfig{
			"contentId": {Selector: ".", Attribute: "data-id"}, "title": {Selector: ".//h2"},
			"targetUrl": {Selector: ".//a", Attribute: "href"}, "publishedAt": {Selector: ".//time"},
		}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items, err := Parse(test.config, "https://example.test/feed", html)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 || items[0].ContentID != "a" || items[0].Title != "Alpha" || items[0].TargetURL != "https://example.test/a" {
				t.Fatalf("unexpected items: %#v", items)
			}
			if items[0].PublishedAt.UnixMilli() != 1700000000000 {
				t.Fatalf("publishedAt = %s", items[0].PublishedAt)
			}
		})
	}
}

func TestParseLimitsItemsAndRejectsInvalidConfiguration(t *testing.T) {
	var records strings.Builder
	for index := 0; index < 55; index++ {
		if index > 0 {
			records.WriteByte(',')
		}
		fmt.Fprintf(&records, `{"id":"%d","title":"Title","url":"/%d"}`, index, index)
	}
	config := Config{Type: TypeRestrictedJSONPath, ItemSelector: "$[*]", Fields: map[string]FieldConfig{
		"contentId": {Selector: "$.id"}, "title": {Selector: "$.title"}, "targetUrl": {Selector: "$.url"},
	}}
	items, err := Parse(config, "https://example.test", []byte("["+records.String()+"]"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != hgMaxItems {
		t.Fatalf("item count = %d, want %d", len(items), hgMaxItems)
	}

	fields := make(map[string]FieldConfig, hgMaxFields+1)
	for index := 0; index <= hgMaxFields; index++ {
		fields[fmt.Sprintf("field%d", index)] = FieldConfig{Selector: "$.value"}
	}
	if _, err := Parse(Config{Type: TypeRestrictedJSONPath, ItemSelector: "$[*]", Fields: fields}, "https://example.test", []byte(`[]`)); err == nil {
		t.Fatal("expected excessive field count error")
	}
}

func TestParseUsesRawItemFallbackWhenMappingsAreEmpty(t *testing.T) {
	config := Config{Type: TypeRestrictedJSONPath, Platform: "custom", ItemSelector: "$.items[*]", Fields: map[string]FieldConfig{}}
	body := []byte(`{"items":[{"name":"first","value":1}]}`)

	items, err := Parse(config, "https://example.test/items", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.HasPrefix(items[0].ContentID, "raw-") {
		t.Fatalf("unexpected fallback items: %#v", items)
	}
	if items[0].Title != `{"name":"first","value":1}` || items[0].TargetURL != "https://example.test/items" {
		t.Fatalf("unexpected fallback item: %#v", items[0])
	}
}

func TestParseRejectsUnsafeURLNegativeNumberAndUnsupportedJSONPath(t *testing.T) {
	config := Config{Type: TypeRestrictedJSONPath, ItemSelector: "$[*]", Fields: map[string]FieldConfig{
		"contentId": {Selector: "$.id"}, "title": {Selector: "$.title"}, "targetUrl": {Selector: "$.url"},
		"likeCount": {Selector: "$.likes"},
	}}
	tests := []struct {
		name string
		body string
	}{
		{name: "unsafe scheme", body: `[{"id":"1","title":"Title","url":"javascript:alert(1)","likes":1}]`},
		{name: "negative number", body: `[{"id":"1","title":"Title","url":"/safe","likes":-1}]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(config, "https://example.test/feed", []byte(test.body)); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}

	config.ItemSelector = "$..items"
	if _, err := Parse(config, "https://example.test/feed", []byte(`[]`)); err == nil {
		t.Fatal("expected recursive JSONPath rejection")
	}
	config.ItemSelector = "$[?(@.id)]"
	if _, err := Parse(config, "https://example.test/feed", []byte(`[]`)); err == nil {
		t.Fatal("expected filter JSONPath rejection")
	}
}
