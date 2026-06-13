package OpsDtoPackage

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateRoleResponse 创建角色响应
type CreateRoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}

// RoleListResponse 角色列表响应
type RoleListResponse struct {
	Total      int64      `json:"total"`
	List       []RoleItem `json:"list"`
	NextCursor string     `json:"nextCursor"`
	HasMore    bool       `json:"hasMore"`
}

// RoleItem 角色项
type RoleItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}

// AdminUserSearchResponse 管理员搜索响应。
// 说明：admin_user 表当前没有 email 字段，Email 先保留为空字符串，便于前端统一展示列结构。
type AdminUserSearchResponse struct {
	Total int64           `json:"total"`
	List  []AdminUserItem `json:"list"`
}

// AdminUserItem 管理员搜索结果项。
type AdminUserItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	NickName string `json:"nickName"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Status   int    `json:"status"`
}

// AssignUserRolesRequest 分配用户角色请求
type AssignUserRolesRequest struct {
	UserID  string   `json:"userId"`
	RoleIDs []string `json:"roleIds"`
}

// UserRoleResponse 用户角色响应
type UserRoleResponse struct {
	UserID string     `json:"userId"`
	Roles  []RoleItem `json:"roles"`
}

// CreateMenuRequest 创建菜单请求
type CreateMenuRequest struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	ParentID string `json:"parentId"`
	Sort     int    `json:"sort"`
	Icon     string `json:"icon"`
}

// CreateMenuResponse 创建菜单响应
type CreateMenuResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	ParentID string `json:"parentId"`
	Sort     int    `json:"sort"`
	Icon     string `json:"icon"`
}

// MenuListResponse 菜单列表响应
type MenuListResponse struct {
	List []MenuItem `json:"list"`
}

// MenuItem 菜单项
type MenuItem struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	ParentID string     `json:"parentId"`
	Sort     int        `json:"sort"`
	Icon     string     `json:"icon"`
	Children []MenuItem `json:"children,omitempty"`
}

// AssignRolePermissionsRequest 分配角色权限请求
type AssignRolePermissionsRequest struct {
	RoleID      string   `json:"roleId"`
	MenuIDs     []string `json:"menuIds"`
	Permissions []string `json:"permissions"`
}

// RolePermissionResponse 角色权限响应
type RolePermissionResponse struct {
	RoleID      string   `json:"roleId"`
	MenuIDs     []string `json:"menuIds"`
	Permissions []string `json:"permissions"`
}

// UploadFileResponse 上传文件响应
type UploadFileResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	Total int64      `json:"total"`
	List  []FileItem `json:"list"`
}

// FileItem 文件项
type FileItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}
