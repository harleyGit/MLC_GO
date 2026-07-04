/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-04 17:10:39
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_dlq.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 
 * 功能： 统一死信投递、失败消息缓存补偿
 */

package HGKafkaPackage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	HGTopicDLQBusiness = "hg.dlq.business"
	HGTopicDLQLog      = "hg.dlq.log"
)

// HGDLQPayload 是统一死信消息结构。
//
// 原始 value 以 []byte JSON/base64 形式编码，避免因为原消息不是 JSON 而破坏 DLQ 消息格式。
type HGDLQPayload struct {
	SourceTopic     string    `json:"source_topic"`
	SourcePartition int32     `json:"source_partition"`
	SourceOffset    int64     `json:"source_offset"`
	Cluster         string    `json:"cluster"`
	Reason          string    `json:"reason"`
	Value           []byte    `json:"value"`
	CreatedAt       time.Time `json:"created_at"`
}

// HGSendDLQ 将失败消息投递到统一死信 topic。
//
// 注意：如果 Kafka 整体不可用，DLQ 也可能失败；生产环境需要结合本地磁盘缓冲或重试任务做最后兜底。
func HGSendDLQ(ctx context.Context, record *kgo.Record, cluster string, reason string) error {
	if record == nil {
		return fmt.Errorf("kafka dlq record cannot be nil")
	}

	payload := HGDLQPayload{
		SourceTopic:     record.Topic,
		SourcePartition: record.Partition,
		SourceOffset:    record.Offset,
		Cluster:         cluster,
		Reason:          reason,
		Value:           record.Value,
		CreatedAt:       time.Now().UTC(),
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal kafka dlq payload: %w", err)
	}

	dlqTopic := HGTopicDLQBusiness
	if cluster == "log" {
		dlqTopic = HGTopicDLQLog
	}

	dlqRecord := &kgo.Record{
		Topic: dlqTopic,
		Key:   record.Key,
		Value: value,
	}
	HGInjectTraceToRecord(ctx, dlqRecord)

	client := HGClient()
	if client == nil {
		return fmt.Errorf("kafka client is not initialized")
	}

	if err := client.ProduceSync(ctx, dlqRecord).FirstErr(); err != nil {
		return fmt.Errorf("produce kafka dlq topic=%s: %w", dlqTopic, err)
	}

	return nil
}
