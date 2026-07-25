package HGKafkaPackage

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHGProcessFetchBatchCommitsLastContiguousRecordPerPartition(t *testing.T) {
	fetches := hgTestFetches(
		hgTestRecord("orders", 0, 10),
		hgTestRecord("orders", 0, 11),
		hgTestRecord("orders", 1, 20),
	)
	var handled []int64

	commits, failed := hgProcessFetchBatch(context.Background(), fetches, func(_ context.Context, record *kgo.Record) error {
		handled = append(handled, record.Offset)
		return nil
	}, nil)

	if failed != nil {
		t.Fatalf("全部处理成功时不应返回失败 offset: %v", failed)
	}
	if !reflect.DeepEqual(handled, []int64{10, 11, 20}) {
		t.Fatalf("处理 offset=%v, want [10 11 20]", handled)
	}
	if got := hgRecordOffsets(commits); !reflect.DeepEqual(got, []int64{11, 20}) {
		t.Fatalf("提交 offset=%v, want [11 20]", got)
	}
}

func TestHGProcessFetchBatchStopsFailedPartitionWithoutBlockingOthers(t *testing.T) {
	fetches := hgTestFetches(
		hgTestRecord("orders", 0, 10),
		hgTestRecord("orders", 0, 11),
		hgTestRecord("orders", 0, 12),
		hgTestRecord("orders", 1, 20),
	)
	var handled []int64

	commits, failed := hgProcessFetchBatch(context.Background(), fetches, func(_ context.Context, record *kgo.Record) error {
		handled = append(handled, record.Offset)
		if record.Partition == 0 && record.Offset == 11 {
			return errors.New("handle failed")
		}
		return nil
	}, func(_ context.Context, _ *kgo.Record, _ error) {})

	if !reflect.DeepEqual(handled, []int64{10, 11, 20}) {
		t.Fatalf("处理 offset=%v, want [10 11 20]", handled)
	}
	if got := hgRecordOffsets(commits); !reflect.DeepEqual(got, []int64{10, 20}) {
		t.Fatalf("提交 offset=%v, want [10 20]", got)
	}
	wantFailed := map[string]map[int32]kgo.EpochOffset{
		"orders": {0: {Epoch: 0, Offset: 11}},
	}
	if !reflect.DeepEqual(failed, wantFailed) {
		t.Fatalf("失败 offset=%v, want %v", failed, wantFailed)
	}
}

func TestHGProcessFetchBatchDoesNotCommitPartitionWhenFirstRecordFails(t *testing.T) {
	fetches := hgTestFetches(
		hgTestRecord("orders", 0, 10),
		hgTestRecord("orders", 0, 11),
	)

	commits, failed := hgProcessFetchBatch(context.Background(), fetches, func(_ context.Context, _ *kgo.Record) error {
		return errors.New("handle failed")
	}, nil)

	if len(commits) != 0 {
		t.Fatalf("首条消息失败时不应提交该分区, got offsets=%v", hgRecordOffsets(commits))
	}
	if got := failed["orders"][0].Offset; got != 10 {
		t.Fatalf("失败 offset=%d, want 10", got)
	}
}

func TestHGProcessFetchBatchRecoversHandlerPanicAndContinuesOtherPartition(t *testing.T) {
	fetches := hgTestFetches(
		hgTestRecord("orders", 0, 10),
		hgTestRecord("orders", 1, 20),
	)
	var handled []int64

	commits, failed := hgProcessFetchBatch(context.Background(), fetches, func(_ context.Context, record *kgo.Record) error {
		handled = append(handled, record.Offset)
		if record.Partition == 0 {
			panic("boom")
		}
		return nil
	}, nil)

	if got := hgRecordOffsets(commits); !reflect.DeepEqual(got, []int64{20}) {
		t.Fatalf("commit offsets=%v, want [20]", got)
	}
	if got := failed["orders"][0].Offset; got != 10 {
		t.Fatalf("failed offset=%d, want 10", got)
	}
	if !reflect.DeepEqual(handled, []int64{10, 20}) {
		t.Fatalf("handled=%v, want [10 20]", handled)
	}
}

func hgTestFetches(records ...*kgo.Record) kgo.Fetches {
	partitions := make(map[int32][]*kgo.Record)
	partitionOrder := make([]int32, 0)
	for _, record := range records {
		if _, ok := partitions[record.Partition]; !ok {
			partitionOrder = append(partitionOrder, record.Partition)
		}
		partitions[record.Partition] = append(partitions[record.Partition], record)
	}

	fetchPartitions := make([]kgo.FetchPartition, 0, len(partitions))
	for _, partition := range partitionOrder {
		fetchPartitions = append(fetchPartitions, kgo.FetchPartition{
			Partition: partition,
			Records:   partitions[partition],
		})
	}

	return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "orders", Partitions: fetchPartitions}}}}
}

func hgTestRecord(topic string, partition int32, offset int64) *kgo.Record {
	return &kgo.Record{Topic: topic, Partition: partition, Offset: offset}
}

func hgRecordOffsets(records []*kgo.Record) []int64 {
	offsets := make([]int64, 0, len(records))
	for _, record := range records {
		offsets = append(offsets, record.Offset)
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
	return offsets
}
