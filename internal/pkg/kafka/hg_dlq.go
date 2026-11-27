/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-23 16:40:05
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_dlq.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

 * 功能： 统一死信投递、失败消息缓存补偿
 */

package HGKafkaPackage

import (
	"MLC_GO/internal/response/hgerrors"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	// HGTopicDLQBusiness 是业务事件死信 topic。
	HGTopicDLQBusiness = "hg.dlq.business"
	// HGTopicDLQLog 是日志/埋点事件死信 topic。
	HGTopicDLQLog = "hg.dlq.log"
)

// HGDLQPayload 是统一死信消息结构。
//
// 原始 value 以 []byte JSON/base64 形式编码，避免因为原消息不是 JSON 而破坏 DLQ 消息格式。
type HGDLQPayload struct {
	ErrorCode       hgerrors.Code      `json:"error_code"`       // 静态原因，不能作为提交依据。
	RetryCount      uint64             `json:"retry_count"`      // 本消费者对同一失败位点已观察到的重试次数。
	Action          HGConsumerAction   `json:"action"`           // retry 留档仍需重试；dlq 为终止隔离。
	SourceTopic     string             `json:"source_topic"`     // 原始来源主题。
	SourcePartition int32              `json:"source_partition"` // 来源分区，必须与 Offset 一起解释。
	SourceOffset    int64              `json:"source_offset"`    // 来源记录位置，不是 DLQ 自己的 offset。
	SourceKey       []byte             `json:"source_key"`       // 原 key，base64 保存，支持离线重投递。
	SourceHeaders   []kgo.RecordHeader `json:"source_headers"`   // 完整来源头，含原链路和事件关联元数据。
	SourceTimestamp time.Time          `json:"source_timestamp"` // 来源 Kafka 时间戳，不伪装为重新生产时间。
	Cluster         string             `json:"cluster"`          // 保留既有调用方的类别标识。
	Reason          string             `json:"reason"`           // 新消费路径只放安全错误 Key。
	Value           []byte             `json:"value"`            // 原始正文，base64 编码，不写入普通日志。
	CreatedAt       time.Time          `json:"created_at"`       // 本次留档 UTC 时间。
}

// HGSendDLQ 将失败消息投递到统一死信 topic。
//
// 注意：如果 Kafka 整体不可用，DLQ 也可能失败；生产环境需要结合本地磁盘缓冲或重试任务做最后兜底。
func HGSendDLQ(ctx context.Context, record *kgo.Record, cluster string, reason string) error {
	return hgSendClassifiedDLQ(ctx, record, cluster, reason, HGConsumerResult{HGConsumerRetry, hgerrors.HGCommonInternalError}, 0)
}

func hgSendClassifiedDLQ(ctx context.Context, record *kgo.Record, cluster, reason string, result HGConsumerResult, retries uint64) error {
	dlqRecord, err := hgBuildDLQRecord(ctx, record, cluster, reason, result, retries)
	if err != nil {
		return err
	}
	client := HGClient()
	if client == nil {
		return fmt.Errorf("kafka client is not initialized")
	}
	if err := client.ProduceSync(ctx, dlqRecord).FirstErr(); err != nil {
		return fmt.Errorf("produce kafka dlq topic=%s: %w", dlqRecord.Topic, err)
	}
	return nil
}

// hgBuildDLQRecord 是无 I/O 的协议适配；成功构造不等于投递成功，更不表示源位点可提交。
func hgBuildDLQRecord(ctx context.Context, record *kgo.Record, cluster, reason string, result HGConsumerResult, retries uint64) (*kgo.Record, error) {
	if record == nil {
		return nil, fmt.Errorf("kafka dlq record cannot be nil")
	}

	// DLQ 记录保留原 topic/partition/offset，便于后续定位原始消息和手工补偿。
	payload := HGDLQPayload{
		ErrorCode: result.Code, RetryCount: retries, Action: result.Action,
		SourceTopic:     record.Topic,
		SourcePartition: record.Partition,
		SourceOffset:    record.Offset,
		SourceKey:       record.Key, SourceHeaders: record.Headers, SourceTimestamp: record.Timestamp,
		Cluster:   cluster,
		Reason:    reason,
		Value:     record.Value,
		CreatedAt: time.Now().UTC(),
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal kafka dlq payload: %w", err)
	}

	dlqTopic := HGTopicDLQBusiness
	if cluster == "log" {
		// 日志类消息和业务类消息分开，避免大流量日志死信淹没核心业务死信。
		dlqTopic = HGTopicDLQLog
	}

	// DLQ 继续使用原消息 key，便于同一实体的失败消息按 key 聚合排查。
	dlqRecord := &kgo.Record{
		Topic: dlqTopic,
		Key:   record.Key,
		Value: value,
	}
	HGInjectTraceToRecord(ctx, dlqRecord)

	return dlqRecord, nil
}
