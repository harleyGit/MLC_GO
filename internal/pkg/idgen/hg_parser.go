package idgen

import "fmt"

// HGParsedID 是业务 ID 的解析结果，包含实体类型和原始 Snowflake 值。
type HGParsedID struct {
	Type  EntityType
	Value uint64
}

// Parse 校验并解析 Prefix + Base62(Snowflake) 格式的业务 ID。
func Parse(id string) (HGParsedID, error) {
	if len(id) < 2 || len(id) > 12 {
		return HGParsedID{}, fmt.Errorf("%w: length must be 2..12", ErrHGInvalidID)
	}

	entityType, err := hgEntityTypeFromPrefix(id[0])
	if err != nil {
		return HGParsedID{}, err
	}

	value, err := hgDecodeBase62(id[1:])
	if err != nil {
		return HGParsedID{}, err
	}
	if value>>63 != 0 {
		return HGParsedID{}, fmt.Errorf("%w: snowflake exceeds 63 bits", ErrHGInvalidID)
	}

	return HGParsedID{Type: entityType, Value: value}, nil
}
