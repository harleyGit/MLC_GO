package idgen

import "fmt"

// Generator 定义业务 ID 生成能力，便于服务通过依赖注入统一使用同一 ID 体系。
type Generator interface {
	Generate(entityType EntityType) (string, error)
}

// HGGenerator 将业务前缀与 Base62 编码的 Snowflake 值组合为最长 12 字节的业务 ID。
type HGGenerator struct {
	snowflake *HGSnowflake
}

// NewHGGenerator 创建业务 ID 生成器。
func NewHGGenerator(snowflake *HGSnowflake) (*HGGenerator, error) {
	if snowflake == nil {
		return nil, fmt.Errorf("%w: nil snowflake", ErrHGInvalidID)
	}
	return &HGGenerator{snowflake: snowflake}, nil
}

// Generate 生成 Prefix + Base62(Snowflake) 格式的业务 ID。
func (generator *HGGenerator) Generate(entityType EntityType) (string, error) {
	if generator == nil || generator.snowflake == nil {
		return "", fmt.Errorf("%w: nil generator", ErrHGInvalidID)
	}

	prefix, err := HGEntityPrefix(entityType)
	if err != nil {
		return "", err
	}

	value, err := generator.snowflake.Generate()
	if err != nil {
		return "", fmt.Errorf("generate snowflake: %w", err)
	}

	encoded := hgEncodeBase62(value)
	var buffer [12]byte
	buffer[0] = prefix
	copy(buffer[1:], encoded)
	return string(buffer[:1+len(encoded)]), nil
}
