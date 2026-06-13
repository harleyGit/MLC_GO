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
//
// 设计思路：
//   - 使用带缓冲的 channel 作为任务队列，容量为 4096，既能缓冲大量任务，又不会无限占用内存
//   - 创建 done channel 用于优雅停止 worker 协程
//   - 启动独立的 goroutine 运行 run() 方法，持续消费队列中的任务
//
// 使用场景：
//   - 本地开发、单实例部署时使用
//   - 生产环境建议替换为 RocketMQ/Kafka 实现
func NewMemoryPublisher() *MemoryPublisher {
	p := &MemoryPublisher{
		queue: make(chan Task, defaultQueueSize), // 有界队列，防止内存溢出
		done:  make(chan struct{}),               // 停止信号 channel
	}
	// 启动 worker 协程，持续消费队列中的任务
	// 这是一个独立的 goroutine，不会阻塞当前函数
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

// run 是 worker 协程的主循环，负责从队列中消费任务并执行。
//
// 工作机制：
//   - 使用 select 多路复用，同时监听两个 channel：
//     1. queue: 有新任务时，取出并调用 handle() 处理
//     2. done: 收到停止信号时，退出循环，worker 协程结束
//   - 当两个 channel 都没有数据时，select 会阻塞等待
//   - 优先处理 queue 中的任务，保证任务及时消费
//
// 生命周期：
//   - 随 NewMemoryPublisher() 启动
//   - 随 Close() 调用结束
func (p *MemoryPublisher) run() {
	for {
		select {
		case task := <-p.queue: // 从队列中取出任务
			p.handle(task) // 处理任务
		case <-p.done: // 收到停止信号
			return // 退出循环，worker 协程结束
		}
	}
}

func (p *MemoryPublisher) handle(task Task) {
	// 当前默认 worker 只记录任务，避免在请求链路内做转码/审核等重活。
	// 接入真实 MQ 后，此处可替换为消费者侧的转码、抽帧、审核调用。
	log.Printf("video upload async task queued: type=%s submission=%s video=%s", task.Type, task.SubmissionID, task.VideoID)
}
