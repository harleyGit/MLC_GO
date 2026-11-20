package notification

import (
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var hgTemplateCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{1,99}$`)
var hgVariableNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var hgActionParamPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]{0,63}$`)
var hgPlaceholderPattern = regexp.MustCompile(`\{\{([a-z][a-z0-9_]*)\}\}`)
var hgTemplateCategories = map[string]bool{"system": true, "security": true, "transaction": true, "content": true, "social": true, "announcement": true, "marketing": true}
var hgActionTypes = map[string]bool{"none": true, "order_detail": true, "security_center": true, "announcement_detail": true, "video_detail": true, "product_detail": true}

// HGTemplateForm 是模板创建或修改的管理协议；仅受控纯文本、JSON 对象和平台作用域。
type HGTemplateForm struct {
	TemplateCode    string          `json:"template_code"`    // 创建时唯一编码；更新时须省略。
	Name            string          `json:"name"`             // 展示名称，1-128 字符。
	Category        string          `json:"category"`         // 固定通知分类枚举。
	TitleTemplate   string          `json:"title_template"`   // 纯文本标题，至多 200 字符。
	BodyTemplate    string          `json:"body_template"`    // 纯文本正文，至多 2048 字符。
	VariablesSchema json.RawMessage `json:"variables_schema"` // 变量规则 JSON 对象；缺省 {}。
	ActionType      string          `json:"action_type"`      // 受控跳转枚举；缺省 none。
	ActionConfig    json.RawMessage `json:"action_config"`    // 受控路由配置；无跳转时 {}。
	Channels        json.RawMessage `json:"channels"`         // 渠道 JSON 数组；目前仅 in_app。
	Priority        uint8           `json:"priority"`         // 优先级 1-3；缺省 2。
	Locale          string          `json:"locale"`           // 当前仅 zh-CN。
	Version         uint32          `json:"version"`          // 更新必传的当前版本号；创建时为零。
}

type hgVariableRule struct {
	Type      string `json:"type"`       // 当前仅支持 string。
	Required  bool   `json:"required"`   // 是否必须提供；在模板中引用的变量均须声明。
	MaxLength int    `json:"max_length"` // 字符上限；0 表示使用默认上限。
	Pattern   string `json:"pattern"`    // 可选正则，须可编译且有界。
}

type hgActionConfig struct {
	RouteName string            `json:"route_name"` // 与 action_type 相同的白名单路由名称。
	Params    map[string]string `json:"params"`     // 路由参数仅支持模板占位符，不允许 URL。
}

// HGValidateTemplateForm 在入库前收紧类型、长度、占位符及跳转范围；并非实际发送的变量校验。
func HGValidateTemplateForm(form *HGTemplateForm, creating bool) error {
	if creating && !hgTemplateCodePattern.MatchString(form.TemplateCode) {
		return HGErrInvalid
	}
	if !creating && (form.TemplateCode != "" || form.Version == 0) {
		return HGErrInvalid
	}
	if len([]rune(strings.TrimSpace(form.Name))) < 1 || len([]rune(form.Name)) > 128 || !hgTemplateCategories[form.Category] ||
		len([]rune(strings.TrimSpace(form.TitleTemplate))) < 1 || len([]rune(form.TitleTemplate)) > 200 ||
		len([]rune(strings.TrimSpace(form.BodyTemplate))) < 1 || len([]rune(form.BodyTemplate)) > 2048 {
		return HGErrInvalid
	}
	if form.Locale == "" {
		form.Locale = "zh-CN"
	}
	if form.Locale != "zh-CN" {
		return HGErrInvalid
	}
	if form.Priority == 0 {
		form.Priority = 2
	}
	if form.Priority < 1 || form.Priority > 3 {
		return HGErrInvalid
	}
	if form.ActionType == "" {
		form.ActionType = "none"
	}
	if !hgActionTypes[form.ActionType] {
		return HGErrInvalid
	}
	if len(form.VariablesSchema) == 0 {
		form.VariablesSchema = json.RawMessage(`{}`)
	}
	if len(form.ActionConfig) == 0 {
		form.ActionConfig = json.RawMessage(`{}`)
	}
	if len(form.Channels) == 0 {
		form.Channels = json.RawMessage(`["in_app"]`)
	}
	var schema map[string]hgVariableRule
	if len(form.VariablesSchema) > 8192 || json.Unmarshal(form.VariablesSchema, &schema) != nil || schema == nil || len(schema) > 30 {
		return HGErrInvalid
	}
	for name, rule := range schema {
		if !hgVariableNamePattern.MatchString(name) || rule.Type != "string" || rule.MaxLength < 0 || rule.MaxLength > 2048 || len(rule.Pattern) > 256 {
			return HGErrInvalid
		}
		if rule.Pattern != "" {
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return HGErrInvalid
			}
		}
	}
	var channels []string
	if len(form.Channels) > 256 || json.Unmarshal(form.Channels, &channels) != nil || len(channels) == 0 || len(channels) > 2 {
		return HGErrInvalid
	}
	seen := map[string]bool{}
	for _, channel := range channels {
		if (channel != "in_app" && channel != "realtime") || seen[channel] {
			return HGErrInvalid
		}
		seen[channel] = true
	}
	if !seen["in_app"] {
		return HGErrInvalid
	}
	var action hgActionConfig
	if len(form.ActionConfig) > 2048 || json.Unmarshal(form.ActionConfig, &action) != nil {
		return HGErrInvalid
	}
	if form.ActionType == "none" {
		if action.RouteName != "" || len(action.Params) > 0 {
			return HGErrInvalid
		}
	} else if action.RouteName != form.ActionType || len(action.Params) > 5 {
		return HGErrInvalid
	}
	for key, value := range action.Params {
		if !hgActionParamPattern.MatchString(key) || !hgPlaceholderPattern.MatchString(value) ||
			hgPlaceholderPattern.FindString(value) != value {
			return HGErrInvalid
		}
	}
	for _, text := range append([]string{form.TitleTemplate, form.BodyTemplate}, hgActionValues(action.Params)...) {
		matches := hgPlaceholderPattern.FindAllStringSubmatch(text, -1)
		remaining := hgPlaceholderPattern.ReplaceAllString(text, "")
		if strings.Contains(remaining, "{{") || strings.Contains(remaining, "}}") {
			return HGErrInvalid
		}
		for _, match := range matches {
			if _, ok := schema[match[1]]; !ok {
				return HGErrInvalid
			}
		}
	}
	return nil
}

func hgActionValues(params map[string]string) []string {
	values := make([]string, 0, len(params))
	for _, value := range params {
		values = append(values, value)
	}
	return values
}

func hgTemplateDBError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return HGErrTemplateConflict
	}
	return err
}

// HGTemplateHandler 将管理端模板 CRUD 映射到受 IAM 权限保护的事务仓储。
type HGTemplateHandler struct{ repo *HGTemplateRepository /* 负责受限的模板增删改查事务。 */ }

// HGNewTemplateHandler 复用共享连接池。
func HGNewTemplateHandler(repo *HGTemplateRepository) *HGTemplateHandler {
	return &HGTemplateHandler{repo: repo}
}

func hgTemplateFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, HGErrInvalid):
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
	case errors.Is(err, HGErrTemplateMissing):
		w.WriteHeader(http.StatusNotFound)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
	case errors.Is(err, HGErrTemplateConflict):
		w.WriteHeader(http.StatusConflict)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.DatabaseError)
	}
}

func hgTemplateBody(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return HGErrInvalid
	}
	return nil
}

func hgTemplatePathID(r *http.Request) (uint64, error) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id == 0 || id > math.MaxInt64 {
		return 0, HGErrInvalid
	}
	return id, nil
}

func hgTemplateUser(r *http.Request) (string, error) {
	id, ok := HGContextPackage.CurrentUserID(r)
	if !ok || id == "" {
		return "", HGErrInvalid
	}
	return id, nil
}

// HGList 返回最多 20 条模板最新版本；游标保持字符串协议。
func (handler *HGTemplateHandler) HGList(w http.ResponseWriter, r *http.Request) {
	cursor := uint64(math.MaxInt64)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 || value > math.MaxInt64 {
			hgTemplateFail(w, r, HGErrInvalid)
			return
		}
		cursor = value
	}
	items, next, err := handler.repo.HGPage(r.Context(), cursor, 20)
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items, "nextCursor": next})
}

// HGDetail 查询数据库当前版本，供编辑回填，不能只使用列表缓存。
func (handler *HGTemplateHandler) HGDetail(w http.ResponseWriter, r *http.Request) {
	id, err := hgTemplatePathID(r)
	if err == nil {
		var item HGTemplate
		item, err = handler.repo.HGDetail(r.Context(), id)
		if err == nil {
			HGResponsePackage.SuccessResult(w, r, item)
			return
		}
	}
	hgTemplateFail(w, r, err)
}

// HGCreate 原子写入草稿主记录与首版快照。
func (handler *HGTemplateHandler) HGCreate(w http.ResponseWriter, r *http.Request) {
	user, err := hgTemplateUser(r)
	var form HGTemplateForm
	if err == nil {
		err = hgTemplateBody(w, r, &form)
	}
	if err == nil && form.Version != 0 {
		err = HGErrInvalid
	}
	if err == nil {
		err = HGValidateTemplateForm(&form, true)
	}
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	id, err := handler.repo.HGCreate(r.Context(), form, user)
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"id": hgTemplateID(id), "version": 1, "status": "draft"})
}

// HGUpdate 使用客户端读取的版本创建新草稿；冲突不自动重试覆盖。
func (handler *HGTemplateHandler) HGUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := hgTemplatePathID(r)
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	user, err := hgTemplateUser(r)
	var form HGTemplateForm
	if err == nil {
		err = hgTemplateBody(w, r, &form)
	}
	if err == nil {
		err = HGValidateTemplateForm(&form, false)
	}
	if err == nil {
		err = handler.repo.HGUpdate(r.Context(), id, form.Version, form, user)
	}
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"id": hgTemplateID(id), "version": form.Version + 1, "status": "draft"})
}

// HGDelete 软删除从未发布的草稿，保留版本历史；需提供读取时的版本。
func (handler *HGTemplateHandler) HGDelete(w http.ResponseWriter, r *http.Request) {
	id, err := hgTemplatePathID(r)
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	user, err := hgTemplateUser(r)
	var request struct {
		Version uint32 `json:"version"`
	}
	if err == nil {
		err = hgTemplateBody(w, r, &request)
	}
	if err == nil && request.Version == 0 {
		err = HGErrInvalid
	}
	if err == nil {
		err = handler.repo.HGDelete(r.Context(), id, request.Version, user)
	}
	if err != nil {
		hgTemplateFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"deleted": true})
}
