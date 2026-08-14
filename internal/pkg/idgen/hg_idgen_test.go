package idgen

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHGEntityPrefix(t *testing.T) {
	tests := []struct {
		entityType EntityType
		prefix     byte
	}{
		{TypeUser, 'U'}, {TypeVideo, 'V'}, {TypeComment, 'C'}, {TypeFollow, 'O'},
		{TypeLike, 'L'}, {TypeFavorite, 'A'}, {TypeCoin, 'N'}, {TypeShare, 'S'},
		{TypeMessage, 'M'}, {TypeRoom, 'R'}, {TypeDanmaku, 'D'}, {TypeTag, 'T'},
		{TypePlaylist, 'P'}, {TypeOrder, 'X'},
	}

	seen := make(map[byte]struct{}, len(tests))
	for _, test := range tests {
		prefix, err := HGEntityPrefix(test.entityType)
		if err != nil {
			t.Fatalf("HGEntityPrefix(%d) returned error: %v", test.entityType, err)
		}
		if prefix != test.prefix {
			t.Fatalf("HGEntityPrefix(%d) = %q, want %q", test.entityType, prefix, test.prefix)
		}
		if _, exists := seen[prefix]; exists {
			t.Fatalf("duplicate prefix %q", prefix)
		}
		seen[prefix] = struct{}{}
	}

	if _, err := HGEntityPrefix(0); !errors.Is(err, ErrHGUnknownEntityType) {
		t.Fatalf("unknown type error = %v, want ErrHGUnknownEntityType", err)
	}
}

func TestHGBase62RoundTrip(t *testing.T) {
	values := []uint64{0, 1, 61, 62, 3843, 1<<63 - 1, ^uint64(0)}
	for _, value := range values {
		encoded := hgEncodeBase62(value)
		decoded, err := hgDecodeBase62(encoded)
		if err != nil {
			t.Fatalf("decode %q: %v", encoded, err)
		}
		if decoded != value {
			t.Fatalf("round trip = %d, want %d", decoded, value)
		}
	}
}

func TestHGDecodeBase62RejectsInvalidValues(t *testing.T) {
	for _, encoded := range []string{"", "00", "01", "-", "!", "LygHa16AHYFz"} {
		if _, err := hgDecodeBase62(encoded); !errors.Is(err, ErrHGInvalidID) {
			t.Fatalf("decode %q error = %v, want ErrHGInvalidID", encoded, err)
		}
	}
}

func TestNewHGSnowflakeValidation(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	clock := func() time.Time { return now }

	for _, workerID := range []int64{-1, 1024} {
		if _, err := newHGSnowflake(now.Add(-time.Hour), workerID, clock, 0); !errors.Is(err, ErrHGInvalidWorkerID) {
			t.Fatalf("worker %d error = %v, want ErrHGInvalidWorkerID", workerID, err)
		}
	}
	if _, err := newHGSnowflake(now.Add(time.Millisecond), 0, clock, 0); !errors.Is(err, ErrHGInvalidEpoch) {
		t.Fatalf("future epoch error = %v, want ErrHGInvalidEpoch", err)
	}
	if _, err := newHGSnowflake(time.UnixMilli(now.UnixMilli()-hgMaxTimestamp-1), 0, clock, 0); !errors.Is(err, ErrHGTimestampOverflow) {
		t.Fatalf("old epoch error = %v, want ErrHGTimestampOverflow", err)
	}
}

func TestHGGeneratorGenerateAndParse(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	snowflake, err := newHGSnowflake(now.Add(-time.Hour), 17, func() time.Time { return now }, 0)
	if err != nil {
		t.Fatalf("new snowflake: %v", err)
	}
	generator, err := NewHGGenerator(snowflake)
	if err != nil {
		t.Fatalf("new generator: %v", err)
	}

	id, err := generator.Generate(TypeVideo)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(id) > 12 || id[0] != 'V' {
		t.Fatalf("generated id = %q, want V prefix and <= 12 bytes", id)
	}

	parsed, err := Parse(id)
	if err != nil {
		t.Fatalf("parse %q: %v", id, err)
	}
	if parsed.Type != TypeVideo {
		t.Fatalf("parsed type = %d, want %d", parsed.Type, TypeVideo)
	}
	if parsed.Value&hgMaxSequence != 0 {
		t.Fatalf("first sequence = %d, want 0", parsed.Value&hgMaxSequence)
	}
}

func TestParseRejectsInvalidIDs(t *testing.T) {
	for _, id := range []string{"", "U", "Z1", "U01", "U!", "Uzzzzzzzzzzz", "U000000000000"} {
		if _, err := Parse(id); err == nil {
			t.Fatalf("Parse(%q) succeeded, want error", id)
		}
	}
}

func TestHGSnowflakeClockRollback(t *testing.T) {
	var current atomic.Int64
	current.Store(1_800_000_000_000)
	clock := func() time.Time { return time.UnixMilli(current.Load()) }
	snowflake, err := newHGSnowflake(time.UnixMilli(current.Load()-1000), 1, clock, 0)
	if err != nil {
		t.Fatalf("new snowflake: %v", err)
	}

	if _, err = snowflake.Generate(); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	current.Add(-1)
	if _, err = snowflake.Generate(); !errors.Is(err, ErrHGClockRollback) {
		t.Fatalf("rollback error = %v, want ErrHGClockRollback", err)
	}
}

func TestHGSnowflakeWaitsForSmallClockRollback(t *testing.T) {
	var current atomic.Int64
	current.Store(1_800_000_000_000)
	clock := func() time.Time { return time.UnixMilli(current.Load()) }
	snowflake, err := newHGSnowflake(time.UnixMilli(current.Load()-1000), 1, clock, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new snowflake: %v", err)
	}

	first, err := snowflake.Generate()
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	current.Add(-1)
	go func() {
		time.Sleep(time.Millisecond)
		current.Add(1)
	}()
	second, err := snowflake.Generate()
	if err != nil {
		t.Fatalf("generate after small rollback: %v", err)
	}
	if second <= first {
		t.Fatalf("second value = %d, want greater than %d", second, first)
	}
}

func TestHGSnowflakeSequenceExhaustion(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	snowflake, err := newHGSnowflake(now.Add(-time.Second), 1, func() time.Time { return now }, 0)
	if err != nil {
		t.Fatalf("new snowflake: %v", err)
	}
	snowflake.lastMillis = now.UnixMilli()
	snowflake.sequence = hgMaxSequence

	if _, err = snowflake.Generate(); !errors.Is(err, ErrHGSequenceExhausted) {
		t.Fatalf("sequence exhaustion error = %v, want ErrHGSequenceExhausted", err)
	}
}

func TestHGGeneratorConcurrentUniqueness(t *testing.T) {
	epoch := time.Now().Add(-time.Hour)
	snowflake, err := NewHGSnowflake(epoch, 23)
	if err != nil {
		t.Fatalf("new snowflake: %v", err)
	}
	generator, err := NewHGGenerator(snowflake)
	if err != nil {
		t.Fatalf("new generator: %v", err)
	}

	const total = 10_000
	ids := make(chan string, total)
	errorsChannel := make(chan error, total)
	var waitGroup sync.WaitGroup
	for index := 0; index < total; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			id, generateErr := generator.Generate(TypeComment)
			if generateErr != nil {
				errorsChannel <- generateErr
				return
			}
			ids <- id
		}()
	}
	waitGroup.Wait()
	close(ids)
	close(errorsChannel)

	for generateErr := range errorsChannel {
		t.Fatalf("concurrent generate: %v", generateErr)
	}
	seen := make(map[string]struct{}, total)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != total {
		t.Fatalf("generated %d unique ids, want %d", len(seen), total)
	}
}
