package idgen

import "errors"

var (
	// ErrHGInvalidWorkerID 表示 Worker ID 超出 Snowflake 的 10 bit 范围。
	ErrHGInvalidWorkerID = errors.New("idgen: invalid worker id")
	// ErrHGInvalidEpoch 表示纪元晚于当前时间，无法生成非负时间戳。
	ErrHGInvalidEpoch = errors.New("idgen: invalid epoch")
	// ErrHGUnknownEntityType 表示业务类型未在全局 ID 类型表中注册。
	ErrHGUnknownEntityType = errors.New("idgen: unknown entity type")
	// ErrHGInvalidID 表示业务 ID 的长度、前缀或 Base62 内容不合法。
	ErrHGInvalidID = errors.New("idgen: invalid id")
	// ErrHGClockRollback 表示系统时钟回拨超过允许等待的范围。
	ErrHGClockRollback = errors.New("idgen: clock rollback")
	// ErrHGSequenceExhausted 表示同一毫秒的 4096 个序列已用尽且等待下一毫秒超时。
	ErrHGSequenceExhausted = errors.New("idgen: sequence exhausted")
	// ErrHGTimestampOverflow 表示相对纪元的毫秒数已超过 41 bit 容量。
	ErrHGTimestampOverflow = errors.New("idgen: timestamp overflow")
)
