package clickhouse

import (
	CrawlerConsumerPackage "MLC_GO/internal/consumer/crawler"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// HGStoreCrawlerHistory 使用有界JSONEachRow批次与异步插入确认，减少低流量小part。
// 表名固定，数据库标识已经由客户端构造校验；重放按collection_id逻辑去重。
func (c *HGClient) HGStoreCrawlerHistory(ctx context.Context, items []CrawlerConsumerPackage.HGHistoryRow) error {
	if len(items) == 0 {
		return nil
	}
	if len(items) > 500 {
		return fmt.Errorf("采集历史批次超过500条")
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return fmt.Errorf("编码采集历史: %w", err)
		}
	}
	query := fmt.Sprintf("INSERT INTO %s.crawler_video_stats_history SETTINGS async_insert=1, wait_for_async_insert=1 FORMAT JSONEachRow", c.config.Database)
	return c.execute(ctx, c.config.WriteTimeout, query, body.Bytes(), nil)
}

// HGHasCrawlerHistory 验收时按单视频及采集时间点查逻辑快照；禁止无时间范围扫描历史表。
func (c *HGClient) HGHasCrawlerHistory(ctx context.Context, bvid string, collectedAt int64) (bool, error) {
	query := fmt.Sprintf("SELECT count() AS total FROM %s.crawler_video_stats_history WHERE bvid={bvid:String} AND collected_at={collected_at:Int64} LIMIT 1 FORMAT JSON", c.config.Database)
	var result struct {
		Data []struct {
			Total uint64 `json:"total,string"`
		} `json:"data"`
	}
	var response bytes.Buffer
	if err := c.execute(ctx, c.config.QueryTimeout, query, nil, map[string]string{"param_bvid": bvid, "param_collected_at": fmt.Sprint(collectedAt)}, &response); err != nil {
		return false, err
	}
	if err := json.Unmarshal(response.Bytes(), &result); err != nil {
		return false, err
	}
	return len(result.Data) == 1 && result.Data[0].Total > 0, nil
}
