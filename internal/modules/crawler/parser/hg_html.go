package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func hgParseCSS(config Config, body []byte) ([]map[string]string, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse crawler HTML for CSS: %w", err)
	}
	rows := make([]map[string]string, 0, hgMaxItems)
	document.Find(config.ItemSelector).EachWithBreak(func(_ int, item *goquery.Selection) bool {
		row := make(map[string]string, len(config.Fields))
		if raw, htmlErr := goquery.OuterHtml(item); htmlErr == nil {
			row["__raw"] = raw
		}
		for name, field := range config.Fields {
			selected := item.Find(field.Selector).First()
			if selected.Length() == 0 && item.Is(field.Selector) {
				selected = item
			}
			if field.Attribute != "" {
				row[name], _ = selected.Attr(field.Attribute)
			} else {
				row[name] = strings.TrimSpace(selected.Text())
			}
		}
		rows = append(rows, row)
		return len(rows) < hgMaxItems
	})
	return rows, nil
}

func hgParseXPath(config Config, body []byte) ([]map[string]string, error) {
	document, err := htmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse crawler HTML for XPath: %w", err)
	}
	items, err := htmlquery.QueryAll(document, config.ItemSelector)
	if err != nil {
		return nil, fmt.Errorf("compile crawler item XPath: %w", err)
	}
	rows := make([]map[string]string, 0, min(len(items), hgMaxItems))
	for _, item := range items {
		row := make(map[string]string, len(config.Fields))
		var raw bytes.Buffer
		if renderErr := html.Render(&raw, item); renderErr == nil {
			row["__raw"] = raw.String()
		}
		for name, field := range config.Fields {
			selected, queryErr := htmlquery.Query(item, field.Selector)
			if queryErr != nil {
				return nil, fmt.Errorf("compile crawler field %q XPath: %w", name, queryErr)
			}
			if selected == nil {
				continue
			}
			if field.Attribute != "" {
				row[name] = htmlquery.SelectAttr(selected, field.Attribute)
			} else {
				row[name] = strings.TrimSpace(htmlquery.InnerText(selected))
			}
		}
		rows = append(rows, row)
		if len(rows) == hgMaxItems {
			break
		}
	}
	return rows, nil
}
