package VideoUploadTaskPackage

import (
	"context"
	"errors"
	"log"
	"sync"
)

const defaultQueueSize = 4096

var ErrQueueFull = errors.New("视频异步任务队列已满")

// TaskType 描述视频投稿异步任务类型。
// 未来接入 RocketMQ/Kafka 时，可以直接把该类型映射为 topic/tag。
type TaskType string

const (
	TaskTypeTranscode TaskType = "transcode"
	TaskTypeSnapshot  TaskType = "snapshot"
	TaskTypeAudit     TaskType = "audit"
	TaskTypePublish   TaskType = "publish"
)

// Task 是视频投稿后续异步处理任务。
// 上传接口只负责文件入库和元数据保存，转码、抽帧、审核、发布状态流转通过任务解耦。
type Task struct {
	Type         TaskType `json:"type"`
	UserID       string   `json:"userId"`
	SubmissionID string   `json:"submissionId"`
	VideoID      string   `json:"videoId,omitempty"`
	FilePath     string   `json:"filePath,omitempty"`
}

// Publisher 定义任务发布接口。
// 生产环境可以用 RocketMQ/Kafka 实现该接口，当前默认实现为进程内有界队列。
type Publisher interface {
	Publish(ctx context.Context, task Task) error
	Close() error
}

// MemoryPublisher 是本地开发和单实例部署可用的默认任务发布器。
// 使用有界 channel 防止任务堆积无限吃内存；队列满时直接返回错误，让上层感知削峰失败。
type MemoryPublisher struct {
	queue chan Task
	done  chan struct{}
	once  sync.Once
}

// NewMemoryPublisher 创建默认内存任务队列并启动 worker。
func NewMemoryPublisher() *MemoryPublisher {
	p := &MemoryPublisher{
		queue: make(chan Task, defaultQueueSize),
		done:  make(chan struct{}),
	}
	go p.run()
	return p
}

// Publish 发布异步任务。
func (p *MemoryPublisher) Publish(ctx context.Context, task Task) error {
	select {
	case p.queue <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrQueueFull
	}
}

// Close 停止内存任务 worker。
func (p *MemoryPublisher) Close() error {
	p.once.Do(func() {
		close(p.done)
	})
	return nil
}

func (p *MemoryPublisher) run() {
	for {
		select {
		case task := <-p.queue:
			p.handle(task)
		case <-p.done:
			return
		}
	}
}

func (p *MemoryPublisher) handle(task Task) {
	// 当前默认 worker 只记录任务，避免在请求链路内做转码/审核等重活。
	// 接入真实 MQ 后，此处可替换为消费者侧的转码、抽帧、审核调用。
	log.Printf("video upload async task queued: type=%s submission=%s video=%s", task.Type, task.SubmissionID, task.VideoID)
}
