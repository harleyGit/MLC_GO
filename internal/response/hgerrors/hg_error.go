// Package hgerrors 维护跨协议的业务错误字典；不访问存储，不依赖业务模块。
package hgerrors

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Code 使用 MM SS EEE 十进制分段，常量可用于 errors.Is，不含 HTTP 成功码。
type Code int32

// Definition 是可导出给客户端的公开元数据。调用者只能取到字典的副本。
type Definition struct {
	Code       Code   `json:"code"`        // 发布后不可重分配的七位数字。
	Key        string `json:"error_key"`   // 前端业务判断和国际化键。
	Message    string `json:"message"`     // 可公开的默认提示。
	HTTPStatus int    `json:"http_status"` // HTTP 适配层使用，不能用于推断 Kafka 动作。
	Retryable  bool   `json:"retryable"`   // 可能恢复，不代表写请求可以自动重放。
}

// AppError 保留原始 cause；序列化响应时只取 Resolve 的静态定义。
type AppError struct {
	code  Code  // 原因分类，未注册值统一降级。
	cause error // 内部错误链，不输出到客户端。
}

func (c Code) Error() string { return c.Definition().Key }
func (e *AppError) Error() string {
	if e == nil {
		return HGCommonInternalError.Error()
	}
	return e.code.Error()
}
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
func (e *AppError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch t := target.(type) {
	case Code:
		return t == e.code
	case *AppError:
		return t != nil && t.code == e.code
	default:
		return false
	}
}

// New 创建无底层 cause 的业务错误。
func New(c Code) *AppError { return Wrap(c, nil) }

// Wrap 在错误产生处或协议边界添加分类，保留 errors.Is/As 可追溯性。
func Wrap(c Code, cause error) *AppError { return &AppError{code: c, cause: cause} }

// Resolve 仅返回受信的静态定义，未知错误、空值均安全降级。
func Resolve(err error) Definition {
	var app *AppError
	if errors.As(err, &app) && app != nil {
		return app.code.Definition()
	}
	var code Code
	if errors.As(err, &code) {
		return code.Definition()
	}
	return HGCommonInternalError.Definition()
}

var hgKey = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)
var hgDomains = []string{"COMMON", "AUTH", "IAM", "IM", "MALL", "PAYMENT", "VIDEO", "CRAWLER", "OPS", "NOTIFICATION", "MEDIA", "REALTIME", "RISK", "SEARCH", "MEMBERSHIP"}

// HGModule 返回编号所属主领域；钱包归 PAYMENT，评论和弹幕归 VIDEO。
func (c Code) HGModule() string {
	i := int(c)/100000 - 10
	if i < 0 || i >= len(hgDomains) {
		return ""
	}
	return hgDomains[i]
}

// ValidateRegistry 用于启动校验及 CI 门禁；普通异步受理 202 不注册为错误。
func ValidateRegistry(defs []Definition) error {
	codes, keys := make(map[Code]bool, len(defs)), make(map[string]bool, len(defs))
	for _, d := range defs {
		module := d.Code.HGModule()
		domain, _, _ := strings.Cut(d.Key, "_")
		if domain == "WALLET" && d.Code/1000 == 1502 {
			domain = "PAYMENT"
		}
		if module == "" || domain != module || d.Code%1000 == 0 || !hgKey.MatchString(d.Key) || strings.TrimSpace(d.Message) == "" ||
			d.HTTPStatus < 400 || d.HTTPStatus > 599 || codes[d.Code] || keys[d.Key] || hgReserved(d.Code) {
			return fmt.Errorf("业务错误定义无效、保留或重复：%d %s", d.Code, d.Key)
		}
		codes[d.Code], keys[d.Key] = true, true
	}
	return nil
}

func hgReserved(c Code) bool {
	// 技术规格中的 *_PENDING 仅保留编号，正常异步受理必须使用 code=200。
	return c == 1501004 || c == 1605001 || c == 2001005
}

var hgRegistry = func() map[Code]Definition {
	var defs []Definition
	for _, group := range hgGroups {
		defs = append(defs, group...)
	}
	if err := ValidateRegistry(defs); err != nil {
		panic(err)
	}
	result := make(map[Code]Definition, len(defs))
	for _, d := range defs {
		result[d.Code] = d
	}
	return result
}()

// Definition 查静态字典；不能通过自定义错误文案或未注册编号扩张公共协议。
func (c Code) Definition() Definition {
	if d, ok := hgRegistry[c]; ok {
		return d
	}
	return hgRegistry[HGCommonInternalError]
}

// HGDefinitions 返回按编号排序的快照，供前端、迁移和兼容性验证使用。
func HGDefinitions() []Definition {
	defs := make([]Definition, 0, len(hgRegistry))
	for _, d := range hgRegistry {
		defs = append(defs, d)
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Code < defs[j].Code })
	return defs
}

// HGValidateCompatibility 检查历史发布快照仍是当前字典的不可变子集；允许新增，不允许删除或重用。
func HGValidateCompatibility(baseline []Definition) error {
	if err := ValidateRegistry(baseline); err != nil {
		return err
	}
	for _, old := range baseline {
		if current, ok := hgRegistry[old.Code]; !ok || current != old {
			return fmt.Errorf("历史业务错误被删除或修改：%d %s", old.Code, old.Key)
		}
	}
	return nil
}
