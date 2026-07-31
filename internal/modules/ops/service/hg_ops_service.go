package OpsServicePackage

import (
	OpsCachePackage "MLC_GO/internal/modules/ops/cache"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsTaskPackage "MLC_GO/internal/modules/ops/task"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	defaultOpsPageSize       = 20
	maxOpsPageSize           = 100
	defaultAdminSearchLimit  = 10
	maxAdminSearchLimit      = 20
	maxAdminSearchKeywordLen = 64
)

// Service 定义运维管理业务逻辑
type Service struct {
	repo        *OpsRepositoryPackage.Repository
	cache       *OpsCachePackage.Cache
	taskPub     OpsTaskPackage.Publisher
	operational *HGOperationalService
}

// NewService 创建运维管理业务逻辑实例
func NewService(repo *OpsRepositoryPackage.Repository, cache *OpsCachePackage.Cache, taskPub OpsTaskPackage.Publisher, operational ...*HGOperationalService) *Service {
	service := &Service{repo: repo, cache: cache, taskPub: taskPub}
	if len(operational) > 0 {
		service.operational = operational[0]
	}
	return service
}

// Operational 返回已注入的资产与链路运维服务；仅供 ops handler 调用。
func (s *Service) Operational() *HGOperationalService { return s.operational }

// CreateRole 创建角色
func (s *Service) CreateRole(ctx context.Context, userID string, req OpsDtoPackage.CreateRoleRequest) (*OpsDtoPackage.CreateRoleResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("角色名称不能为空")
	}
	description := strings.TrimSpace(req.Description)
	id, err := s.repo.CreateRole(ctx, name, description)
	if err != nil {
		return nil, err
	}
	return &OpsDtoPackage.CreateRoleResponse{ID: id, Name: name, Description: description}, nil
}

// UpdateRole 更新角色
func (s *Service) UpdateRole(ctx context.Context, userID string, req OpsDtoPackage.UpdateRoleRequest) (*OpsDtoPackage.UpdateRoleResponse, error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return nil, errors.New("角色ID不能为空")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("角色名称不能为空")
	}
	description := strings.TrimSpace(req.Description)
	item, err := s.repo.UpdateRole(ctx, id, name, description)
	if err != nil {
		return nil, err
	}
	return &OpsDtoPackage.UpdateRoleResponse{
		ID:          toString(item["id"]),
		Name:        toString(item["name"]),
		Description: toString(item["description"]),
		CreatedAt:   toString(item["createdAt"]),
	}, nil
}

// DeleteRole 删除角色
func (s *Service) DeleteRole(ctx context.Context, userID string, req OpsDtoPackage.DeleteRoleRequest) error {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return errors.New("角色ID不能为空")
	}
	return s.repo.DeleteRole(ctx, id)
}

// GetRoleList 获取角色列表。
// 千万级表约束：角色列表使用 cursor 翻页，cursor 是上一页最后一条角色 id；cursor=0 表示首页。
// Service 只限制 pageSize 上限并透传 cursor，不提供实时 total，避免 Repository 执行 COUNT/OFFSET。
func (s *Service) GetRoleList(ctx context.Context, cursor int64, pageSize int) (*OpsDtoPackage.RoleListResponse, error) {
	if cursor < 0 {
		cursor = 0
	}
	if pageSize <= 0 {
		pageSize = defaultOpsPageSize
	}
	if pageSize > maxOpsPageSize {
		pageSize = maxOpsPageSize
	}
	items, total, hasMore, err := s.repo.GetRoleList(ctx, cursor, pageSize)
	if err != nil {
		return nil, err
	}
	list := make([]OpsDtoPackage.RoleItem, 0, len(items))
	nextCursor := ""
	for _, item := range items {
		list = append(list, OpsDtoPackage.RoleItem{
			ID:          toString(item["id"]),
			Name:        toString(item["name"]),
			Description: toString(item["description"]),
			CreatedAt:   toString(item["createdAt"]),
		})
		nextCursor = toString(item["id"])
	}
	return &OpsDtoPackage.RoleListResponse{Total: total, List: list, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// GetAdminUserList 获取管理员列表。
// 千万级表约束：列表页使用 admin_user.id cursor 翻页，Service 只校验 cursor/pageSize，不提供实时 total。
func (s *Service) GetAdminUserList(ctx context.Context, cursor int64, pageSize int) (*OpsDtoPackage.AdminUserListResponse, error) {
	if cursor < 0 {
		cursor = 0
	}
	if pageSize <= 0 {
		pageSize = defaultOpsPageSize
	}
	if pageSize > maxOpsPageSize {
		pageSize = maxOpsPageSize
	}
	items, total, hasMore, err := s.repo.GetAdminUserList(ctx, cursor, pageSize)
	if err != nil {
		return nil, err
	}
	list := make([]OpsDtoPackage.AdminUserItem, 0, len(items))
	nextCursor := ""
	for _, item := range items {
		list = append(list, OpsDtoPackage.AdminUserItem{
			ID:       toString(item["id"]),
			Name:     toString(item["name"]),
			NickName: toString(item["nickName"]),
			Email:    toString(item["email"]),
			Mobile:   toString(item["mobile"]),
			Status:   toInt(item["status"]),
		})
		nextCursor = toString(item["id"])
	}
	return &OpsDtoPackage.AdminUserListResponse{Total: total, List: list, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// SearchAdminUsers 搜索可分配角色的管理员。
// 千万级表约束：Service 层限制关键词长度和返回条数，Repository 层只做主键/唯一键/前缀索引查询，避免无界模糊查询拖垮 admin_user 表。
func (s *Service) SearchAdminUsers(ctx context.Context, keyword string, limit int) (*OpsDtoPackage.AdminUserSearchResponse, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return &OpsDtoPackage.AdminUserSearchResponse{Total: 0, List: []OpsDtoPackage.AdminUserItem{}}, nil
	}
	if len([]rune(keyword)) > maxAdminSearchKeywordLen {
		return nil, errors.New("搜索关键词过长")
	}
	if limit <= 0 {
		limit = defaultAdminSearchLimit
	}
	if limit > maxAdminSearchLimit {
		limit = maxAdminSearchLimit
	}
	items, total, err := s.repo.SearchAdminUsers(ctx, keyword, limit)
	if err != nil {
		return nil, err
	}
	list := make([]OpsDtoPackage.AdminUserItem, 0, len(items))
	for _, item := range items {
		list = append(list, OpsDtoPackage.AdminUserItem{
			ID:       toString(item["id"]),
			Name:     toString(item["name"]),
			NickName: toString(item["nickName"]),
			Email:    toString(item["email"]),
			Mobile:   toString(item["mobile"]),
			Status:   toInt(item["status"]),
		})
	}
	return &OpsDtoPackage.AdminUserSearchResponse{Total: total, List: list}, nil
}

// SearchAdminCandidates 搜索可添加为管理员的注册用户候选。
// 千万级表约束：只允许有限长度关键词和有限返回条数，Repository 仅做主键/唯一键/前缀索引查询。
func (s *Service) SearchAdminCandidates(ctx context.Context, keyword string, limit int) (*OpsDtoPackage.AdminCandidateSearchResponse, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return &OpsDtoPackage.AdminCandidateSearchResponse{Total: 0, List: []OpsDtoPackage.AdminCandidateItem{}}, nil
	}
	if len([]rune(keyword)) > maxAdminSearchKeywordLen {
		return nil, errors.New("搜索关键词过长")
	}
	if limit <= 0 {
		limit = defaultAdminSearchLimit
	}
	if limit > maxAdminSearchLimit {
		limit = maxAdminSearchLimit
	}
	items, total, err := s.repo.SearchAdminCandidates(ctx, keyword, limit)
	if err != nil {
		return nil, err
	}
	list := make([]OpsDtoPackage.AdminCandidateItem, 0, len(items))
	for _, item := range items {
		list = append(list, OpsDtoPackage.AdminCandidateItem{
			ID:       toString(item["id"]),
			UserID:   toString(item["userId"]),
			UserName: toString(item["userName"]),
			NickName: toString(item["nickName"]),
			Email:    toString(item["email"]),
			Phone:    toString(item["phone"]),
		})
	}
	return &OpsDtoPackage.AdminCandidateSearchResponse{Total: total, List: list}, nil
}

// AddAdmin 将注册用户添加为管理员。
func (s *Service) AddAdmin(ctx context.Context, operatorID string, req OpsDtoPackage.AddAdminRequest) (*OpsDtoPackage.AddAdminResponse, error) {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, errors.New("缺少用户ID")
	}
	item, err := s.repo.AddAdminFromUser(ctx, operatorID, userID)
	if err != nil {
		return nil, err
	}
	return &OpsDtoPackage.AddAdminResponse{
		ID:       toString(item["id"]),
		Name:     toString(item["name"]),
		NickName: toString(item["nickName"]),
		Email:    toString(item["email"]),
		Mobile:   toString(item["mobile"]),
		Status:   toInt(item["status"]),
	}, nil
}

// AssignUserRoles 分配用户角色
func (s *Service) AssignUserRoles(ctx context.Context, userID string, req OpsDtoPackage.AssignUserRolesRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return errors.New("缺少用户ID")
	}
	if len(req.RoleIDs) == 0 {
		return errors.New("请至少选择一个角色")
	}
	if len(req.RoleIDs) > 50 {
		return errors.New("单次分配角色数量过多")
	}
	return s.repo.AssignUserRoles(ctx, req.UserID, req.RoleIDs)
}

// GetUserRoles 获取用户角色
func (s *Service) GetUserRoles(ctx context.Context, userID string) (*OpsDtoPackage.UserRoleResponse, error) {
	items, err := s.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles := make([]OpsDtoPackage.RoleItem, 0, len(items))
	for _, item := range items {
		roles = append(roles, OpsDtoPackage.RoleItem{
			ID:          toString(item["id"]),
			Name:        toString(item["name"]),
			Description: toString(item["description"]),
			CreatedAt:   toString(item["createdAt"]),
		})
	}
	return &OpsDtoPackage.UserRoleResponse{UserID: userID, Roles: roles}, nil
}

// CreateMenu 创建菜单
func (s *Service) CreateMenu(ctx context.Context, userID string, req OpsDtoPackage.CreateMenuRequest) (*OpsDtoPackage.CreateMenuResponse, error) {
	// TODO: 实现创建菜单逻辑
	return nil, nil
}

// GetMenuList 获取菜单列表
func (s *Service) GetMenuList(ctx context.Context) (*OpsDtoPackage.MenuListResponse, error) {
	// TODO: 实现获取菜单列表逻辑
	return nil, nil
}

// AssignRolePermissions 分配角色权限
func (s *Service) AssignRolePermissions(ctx context.Context, userID string, req OpsDtoPackage.AssignRolePermissionsRequest) error {
	// TODO: 实现分配角色权限逻辑
	return nil
}

// GetRolePermissions 获取角色权限
func (s *Service) GetRolePermissions(ctx context.Context, roleID string) (*OpsDtoPackage.RolePermissionResponse, error) {
	// TODO: 实现获取角色权限逻辑
	return nil, nil
}

// UploadFile 上传文件
func (s *Service) UploadFile(ctx context.Context, userID string, file io.Reader, fileName string, fileSize int64, mimeType string) (*OpsDtoPackage.UploadFileResponse, error) {
	// TODO: 实现上传文件逻辑
	return nil, nil
}

// GetFileList 获取文件列表
func (s *Service) GetFileList(ctx context.Context, page, pageSize int) (*OpsDtoPackage.FileListResponse, error) {
	// TODO: 实现获取文件列表逻辑
	return nil, nil
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case nil:
		return ""
	default:
		return fmt.Sprint(val)
	}
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case uint8:
		return int(val)
	default:
		return 0
	}
}
