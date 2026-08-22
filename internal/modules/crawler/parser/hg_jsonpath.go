package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type hgJSONPathToken struct {
	key      string
	index    int
	wildcard bool
	isIndex  bool
}

func hgParseJSON(config Config, body []byte) ([]map[string]string, error) {
	var document any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode crawler JSON: %w", err)
	}
	itemPath, err := hgCompileJSONPath(config.ItemSelector)
	if err != nil {
		return nil, fmt.Errorf("compile crawler item JSONPath: %w", err)
	}
	items := hgEvaluateJSONPath(document, itemPath)
	rows := make([]map[string]string, 0, min(len(items), hgMaxItems))
	compiled := make(map[string][]hgJSONPathToken, len(config.Fields))
	for name, field := range config.Fields {
		compiled[name], err = hgCompileJSONPath(field.Selector)
		if err != nil {
			return nil, fmt.Errorf("compile crawler field %q JSONPath: %w", name, err)
		}
	}
	for _, item := range items {
		row := make(map[string]string, len(compiled))
		for name, path := range compiled {
			values := hgEvaluateJSONPath(item, path)
			if len(values) > 0 {
				row[name], err = hgJSONScalarString(values[0])
				if err != nil {
					return nil, fmt.Errorf("extract crawler field %q: %w", name, err)
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func hgCompileJSONPath(expression string) ([]hgJSONPathToken, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" || expression[0] != '$' {
		return nil, errors.New("JSONPath must start with $")
	}
	tokens := make([]hgJSONPathToken, 0, 4)
	for position := 1; position < len(expression); {
		switch expression[position] {
		case '.':
			position++
			start := position
			for position < len(expression) && expression[position] != '.' && expression[position] != '[' {
				position++
			}
			if start == position {
				return nil, errors.New("dot key cannot be empty")
			}
			tokens = append(tokens, hgJSONPathToken{key: expression[start:position]})
		case '[':
			end, token, err := hgCompileJSONBracket(expression, position)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			position = end
		default:
			return nil, fmt.Errorf("unsupported JSONPath token at offset %d", position)
		}
	}
	return tokens, nil
}

func hgCompileJSONBracket(expression string, position int) (int, hgJSONPathToken, error) {
	position++
	if position >= len(expression) {
		return 0, hgJSONPathToken{}, errors.New("unclosed JSONPath bracket")
	}
	if expression[position] == '*' {
		if position+1 >= len(expression) || expression[position+1] != ']' {
			return 0, hgJSONPathToken{}, errors.New("invalid JSONPath wildcard")
		}
		return position + 2, hgJSONPathToken{wildcard: true}, nil
	}
	if expression[position] == '\'' || expression[position] == '"' {
		quote := expression[position]
		position++
		var key strings.Builder
		for position < len(expression) {
			character := expression[position]
			if character == '\\' {
				position++
				if position >= len(expression) {
					return 0, hgJSONPathToken{}, errors.New("invalid JSONPath key escape")
				}
				key.WriteByte(expression[position])
				position++
				continue
			}
			if character == quote {
				if position+1 >= len(expression) || expression[position+1] != ']' {
					return 0, hgJSONPathToken{}, errors.New("quoted JSONPath key must end with ]")
				}
				return position + 2, hgJSONPathToken{key: key.String()}, nil
			}
			key.WriteByte(character)
			position++
		}
		return 0, hgJSONPathToken{}, errors.New("unclosed quoted JSONPath key")
	}
	start := position
	for position < len(expression) && expression[position] >= '0' && expression[position] <= '9' {
		position++
	}
	if start == position || position >= len(expression) || expression[position] != ']' {
		return 0, hgJSONPathToken{}, errors.New("JSONPath bracket must contain a quoted key, numeric index, or *")
	}
	index, err := strconv.Atoi(expression[start:position])
	if err != nil {
		return 0, hgJSONPathToken{}, errors.New("JSONPath index is too large")
	}
	return position + 1, hgJSONPathToken{index: index, isIndex: true}, nil
}

func hgEvaluateJSONPath(root any, tokens []hgJSONPathToken) []any {
	values := []any{root}
	for _, token := range tokens {
		next := make([]any, 0, len(values))
		for _, value := range values {
			switch {
			case token.wildcard:
				if array, ok := value.([]any); ok {
					next = append(next, array...)
				}
			case token.isIndex:
				if array, ok := value.([]any); ok && token.index < len(array) {
					next = append(next, array[token.index])
				}
			default:
				if object, ok := value.(map[string]any); ok {
					if child, exists := object[token.key]; exists {
						next = append(next, child)
					}
				}
			}
		}
		values = next
		if len(values) == 0 {
			break
		}
	}
	return values
}

func hgJSONScalarString(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case json.Number:
		return typed.String(), nil
	case bool:
		return strconv.FormatBool(typed), nil
	default:
		return "", errors.New("selected JSON value must be scalar")
	}
}
