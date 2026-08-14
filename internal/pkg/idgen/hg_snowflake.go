package idgen

import (
	"fmt"
	"sync"
	"time"
)

const (
	hgWorkerBits       = 10
	hgSequenceBits     = 12
	hgMaxWorkerID      = int64(1<<hgWorkerBits - 1)
	hgMaxSequence      = uint64(1<<hgSequenceBits - 1)
	hgWorkerShift      = hgSequenceBits
	hgTimestampShift   = hgWorkerBits + hgSequenceBits
	hgMaxTimestamp     = int64(1<<41 - 1)
	hgDefaultMaxWait   = 5 * time.Millisecond
	hgWaitPollInterval = 50 * time.Microsecond
)

type hgClock func() time.Time

// HGSnowflake 并发安全地生成 41 bit 时间戳、10 bit Worker ID 和 12 bit 序列组成的值。
// 同一 Worker ID 在任一时刻只能由一个进程持有，否则无法保证分布式唯一性。
type HGSnowflake struct {
	mu sync.Mutex

	epochMillis int64
	workerID    uint64
	lastMillis  int64
	sequence    uint64
	maxWait     time.Duration
	now         hgClock
}

// NewHGSnowflake 创建 Snowflake 生成器。epoch 必须不晚于当前时间，workerID 范围为 0 到 1023。
func NewHGSnowflake(epoch time.Time, workerID int64) (*HGSnowflake, error) {
	return newHGSnowflake(epoch, workerID, time.Now, hgDefaultMaxWait)
}

func newHGSnowflake(epoch time.Time, workerID int64, now hgClock, maxWait time.Duration) (*HGSnowflake, error) {
	if workerID < 0 || workerID > hgMaxWorkerID {
		return nil, fmt.Errorf("%w: got %d, want 0..%d", ErrHGInvalidWorkerID, workerID, hgMaxWorkerID)
	}
	if now == nil || maxWait < 0 {
		return nil, ErrHGInvalidEpoch
	}

	currentMillis := now().UnixMilli()
	epochMillis := epoch.UnixMilli()
	if epochMillis > currentMillis {
		return nil, fmt.Errorf("%w: epoch is after current time", ErrHGInvalidEpoch)
	}
	if currentMillis-epochMillis > hgMaxTimestamp {
		return nil, ErrHGTimestampOverflow
	}

	return &HGSnowflake{
		epochMillis: epochMillis,
		workerID:    uint64(workerID),
		lastMillis:  -1,
		maxWait:     maxWait,
		now:         now,
	}, nil
}

// Generate 返回下一个 Snowflake 值。小幅时钟回拨或同毫秒序列耗尽时最多等待 5ms，超时后返回错误。
func (snowflake *HGSnowflake) Generate() (uint64, error) {
	if snowflake == nil {
		return 0, fmt.Errorf("%w: nil snowflake", ErrHGInvalidID)
	}

	snowflake.mu.Lock()
	defer snowflake.mu.Unlock()

	nowMillis := snowflake.now().UnixMilli()
	if nowMillis < snowflake.lastMillis {
		var err error
		nowMillis, err = snowflake.waitUntilAfter(snowflake.lastMillis - 1)
		if err != nil {
			return 0, fmt.Errorf("%w: rollback=%dms", ErrHGClockRollback, snowflake.lastMillis-nowMillis)
		}
	}

	if nowMillis == snowflake.lastMillis {
		if snowflake.sequence == hgMaxSequence {
			var err error
			nowMillis, err = snowflake.waitUntilAfter(snowflake.lastMillis)
			if err != nil {
				return 0, fmt.Errorf("%w: millisecond=%d", ErrHGSequenceExhausted, snowflake.lastMillis)
			}
			snowflake.sequence = 0
		} else {
			snowflake.sequence++
		}
	} else {
		snowflake.sequence = 0
	}

	timestamp := nowMillis - snowflake.epochMillis
	if timestamp < 0 {
		return 0, ErrHGInvalidEpoch
	}
	if timestamp > hgMaxTimestamp {
		return 0, ErrHGTimestampOverflow
	}

	snowflake.lastMillis = nowMillis
	return uint64(timestamp)<<hgTimestampShift |
		snowflake.workerID<<hgWorkerShift |
		snowflake.sequence, nil
}

func (snowflake *HGSnowflake) waitUntilAfter(targetMillis int64) (int64, error) {
	deadline := time.Now().Add(snowflake.maxWait)
	for {
		nowMillis := snowflake.now().UnixMilli()
		if nowMillis > targetMillis {
			return nowMillis, nil
		}
		if snowflake.maxWait == 0 || !time.Now().Before(deadline) {
			return nowMillis, ErrHGClockRollback
		}
		time.Sleep(hgWaitPollInterval)
	}
}
