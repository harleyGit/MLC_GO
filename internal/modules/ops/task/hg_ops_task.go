package OpsTaskPackage

// Publisher 定义任务发布接口
type Publisher interface {
	Publish(taskType string, payload interface{}) error
}

// MemoryPublisher 内存任务发布器（用于测试）
type MemoryPublisher struct{}

// NewMemoryPublisher 创建内存任务发布器
func NewMemoryPublisher() *MemoryPublisher {
	return &MemoryPublisher{}
}

// Publish 发布任务到内存（测试用）
func (p *MemoryPublisher) Publish(taskType string, payload interface{}) error {
	// TODO: 实现内存任务发布
	return nil
}