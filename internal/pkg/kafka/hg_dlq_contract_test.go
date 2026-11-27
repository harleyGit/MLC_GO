package HGKafkaPackage

import (
	utils "MLC_GO/internal/pkg/utils"
	"MLC_GO/internal/response/hgerrors"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHGDLQRetainsReplayMetadata(t *testing.T) {
	source := &kgo.Record{Topic: "events", Partition: 3, Offset: 8, Key: []byte("entity"), Value: []byte{0xff, 0x00},
		Headers: []kgo.RecordHeader{{Key: "event-id", Value: []byte("fixture")}}, Timestamp: time.Unix(100, 0).UTC()}
	err := HGNewTerminalError(hgerrors.Wrap(hgerrors.HGCrawlerStageConflict, errors.New("private SQL")))
	record, buildErr := hgBuildDLQRecord(utils.InjectTID(context.Background(), "trace-fixture"), source, "business", "CRAWLER_STAGE_CONFLICT", HGClassifyConsumer(err), 2)
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	var payload HGDLQPayload
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ErrorCode != 1703002 || payload.RetryCount != 2 || payload.Action != HGConsumerDLQ || payload.SourceOffset != 8 || payload.SourcePartition != 3 || payload.SourceTopic != "events" ||
		!bytes.Equal(payload.Value, source.Value) || !bytes.Equal(payload.SourceKey, source.Key) || !reflect.DeepEqual(payload.SourceHeaders, source.Headers) || !payload.SourceTimestamp.Equal(source.Timestamp) ||
		record.Topic != HGTopicDLQBusiness || !bytes.Equal(record.Key, source.Key) || bytes.Contains(record.Value, []byte("private SQL")) {
		t.Fatalf("DLQ 协议丢失或泄露：%+v", payload)
	}
	if utils.GetTID(HGExtractTraceFromRecord(record)) != "trace-fixture" {
		t.Fatal("DLQ 关联链路丢失")
	}
}

func TestHGDLQFailureKeepsSourcePartition(t *testing.T) {
	fetches := hgTestFetches(hgTestRecord("t", 0, 1), hgTestRecord("t", 0, 2))
	commits, failed := hgProcessFetchBatch(context.Background(), fetches, func(context.Context, *kgo.Record) error { return HGNewTerminalError(hgerrors.HGCommonInvalidParam) },
		func(context.Context, *kgo.Record, error) bool { return false }) // 模拟同步 DLQ 写失败。
	if len(commits) != 0 || failed["t"][0].Offset != 1 {
		t.Fatal("DLQ 未确认却推进了源位点")
	}
}
