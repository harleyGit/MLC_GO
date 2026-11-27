package HGKafkaPackage

import (
	"MLC_GO/internal/response/hgerrors"
	"github.com/twmb/franz-go/pkg/kgo"
)

// HGConsumerAction 描述消费动作，独立于 HTTP Status 和字典的 Retryable。
type HGConsumerAction string

const (
	HGConsumerACK   HGConsumerAction = "ack"
	HGConsumerRetry HGConsumerAction = "retry"
	HGConsumerDLQ   HGConsumerAction = "dlq"
)

// HGConsumerResult 表示意图；DLQ 动作仍须等同步投递成功后才可推进源位点。
type HGConsumerResult struct {
	Action HGConsumerAction // ack、retry 或 dlq。
	Code   hgerrors.Code    // 成功为零，失败为字典中的原因码。
}

func HGClassifyConsumer(err error) HGConsumerResult {
	if err == nil {
		return HGConsumerResult{Action: HGConsumerACK}
	}
	action := HGConsumerRetry
	if HGIsTerminalError(err) {
		action = HGConsumerDLQ
	}
	return HGConsumerResult{action, hgerrors.Resolve(err).Code}
}

type hgPartitionKey struct {
	topic     string // 订阅主题，不来自业务载荷。
	partition int32  // Kafka 物理分区。
}
type hgFailedHead struct {
	offset  int64  // 仅保留该分区最近失败的位置。
	retries uint64 // 当前消费者观察到的重复失败次数，首次为零。
}

// 仅由消费循环访问。每分区一个槽；即使订阅拓扑变化，内存也有硬上限。
type hgRetryHeads map[hgPartitionKey]hgFailedHead

func (h hgRetryHeads) failure(r *kgo.Record) uint64 {
	key := hgPartitionKey{r.Topic, r.Partition}
	head, ok := h[key]
	if ok && head.offset == r.Offset {
		head.retries++
	} else {
		if !ok && len(h) >= 4096 {
			for old := range h {
				delete(h, old)
				break
			}
		}
		head = hgFailedHead{offset: r.Offset}
	}
	h[key] = head
	return head.retries
}
