package OpsDtoPackage

import (
	"errors"
	"time"
)

var ErrHGAssetCorrectionInvalidApprover = errors.New("correction approver must differ from applicant")

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

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateRoleResponse 更新角色响应
type UpdateRoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}

// DeleteRoleRequest 删除角色请求
type DeleteRoleRequest struct {
	ID string `json:"id"`
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

// BilibiliTagRequest 创建 Bilibili 动画标签请求。
// Status 取值：1 启用、2 停用；传 0 时创建接口默认启用。
type BilibiliTagRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Status    int    `json:"status"`
}

// UpdateBilibiliTagRequest 更新 Bilibili 动画标签请求。
type UpdateBilibiliTagRequest struct {
	TagID     string `json:"tagId"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Status    int    `json:"status"`
}

// DeleteBilibiliTagRequest 删除 Bilibili 动画标签请求。
type DeleteBilibiliTagRequest struct {
	TagID string `json:"tagId"`
}

// BilibiliTagListResponse Bilibili 动画标签列表响应。
type BilibiliTagListResponse struct {
	Total      int64             `json:"total"`
	List       []BilibiliTagItem `json:"list"`
	NextCursor string            `json:"nextCursor"`
	HasMore    bool              `json:"hasMore"`
}

// BilibiliTagItem Bilibili 动画标签项。
type BilibiliTagItem struct {
	TagID     string `json:"tagId"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Status    int    `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// AdminUserSearchResponse 管理员搜索响应。
// 说明：admin_user 表当前没有 email 字段，Email 先保留为空字符串，便于前端统一展示列结构。
type AdminUserSearchResponse struct {
	Total int64           `json:"total"`
	List  []AdminUserItem `json:"list"`
}

// AdminUserListResponse 管理员列表响应。
// 调用场景：运维后台管理员列表页默认展示；使用 admin_user.id cursor 分页，Total=-1 表示不做实时 COUNT。
type AdminUserListResponse struct {
	Total      int64           `json:"total"`
	List       []AdminUserItem `json:"list"`
	NextCursor string          `json:"nextCursor"`
	HasMore    bool            `json:"hasMore"`
}

// AdminCandidateSearchResponse 添加管理员候选用户搜索响应。
// 调用场景：运维后台添加管理员时，先从注册用户 users 表搜索候选，再确认提升为 admin_user。
type AdminCandidateSearchResponse struct {
	Total int64                `json:"total"`
	List  []AdminCandidateItem `json:"list"`
}

// AdminCandidateItem 添加管理员候选用户项。
// 字段来源：users 表，ID 对应 users.id；UserID 对应业务 users.user_id。
type AdminCandidateItem struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	NickName string `json:"nickName"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// AddAdminRequest 添加管理员请求。
// UserID 对应候选用户 users.id，后端按该主键读取用户资料并写入 admin_user，避免前端传入可篡改的管理员字段。
type AddAdminRequest struct {
	UserID string `json:"userId"`
}

// AddAdminResponse 添加管理员响应。
// ID 是新创建或已存在的 admin_user.id。
type AddAdminResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	NickName string `json:"nickName"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Status   int    `json:"status"`
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

// HGCoinGrantRequest 是人工赠币请求；Amount 使用十进制字符串避免 JavaScript uint64 精度损失。
type HGCoinGrantRequest struct {
	UserID      string     `json:"userId"`
	RequestID   string     `json:"requestId"`
	Amount      string     `json:"amount"`
	Reason      string     `json:"reason"`
	BusinessKey string     `json:"businessKey"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

// HGCoinRefundRequest 必须引用同一用户的原 debit transaction。
type HGCoinRefundRequest struct {
	UserID                 string `json:"userId"`
	RequestID              string `json:"requestId"`
	Amount                 string `json:"amount"`
	Reason                 string `json:"reason"`
	ReferenceTransactionID string `json:"referenceTransactionId"`
}

// HGCoinCorrectionRequest 通过不可变 correction 流水修正资产，禁止直接覆盖余额。
type HGCoinCorrectionRequest struct {
	UserID      string `json:"userId"`
	RequestID   string `json:"requestId"`
	TicketID    string `json:"ticketId"`
	WorkOrderID string `json:"workOrderId,omitempty"`
	Delta       string `json:"delta"`
	Reason      string `json:"reason"`
}

// HGCoinCorrectionApproveRequest approves and applies a pending correction by its immutable request ID.
type HGCoinCorrectionApproveRequest struct {
	CorrectionID string `json:"correctionId"`
}

// HGCoinCorrectionResponse describes the durable two-step correction workflow state.
type HGCoinCorrectionResponse struct {
	CorrectionID  string `json:"correctionId"`
	UserID        string `json:"userId"`
	RequestID     string `json:"requestId"`
	TicketID      string `json:"ticketId"`
	WorkOrderID   string `json:"workOrderId,omitempty"`
	Delta         string `json:"delta"`
	Reason        string `json:"reason"`
	ApplicantID   string `json:"applicantId"`
	ApproverID    string `json:"approverId,omitempty"`
	Status        string `json:"status"`
	TransactionID string `json:"transactionId,omitempty"`
	BalanceAfter  string `json:"balanceAfter,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// HGCoinCorrectionListResponse uses an ID cursor and a bounded page size.
type HGCoinCorrectionListResponse struct {
	List       []HGCoinCorrectionResponse `json:"list"`
	NextCursor string                     `json:"nextCursor"`
	HasMore    bool                       `json:"hasMore"`
}

// HGAssetPermissionsResponse exposes the current JWT operator's database-backed asset permissions.
type HGAssetPermissionsResponse struct {
	Permissions []string `json:"permissions"`
}

// HGAssetOperator contains trusted request metadata populated outside the JSON body.
type HGAssetOperator struct {
	ID       string
	SourceIP string
	TID      string
}

// HGAssetAuditRecord is the repository input for an immutable asset audit row.
type HGAssetAuditRecord struct {
	EventKey     string
	OperatorID   string
	Action       string
	TargetUserID string
	SourceIP     string
	RequestID    string
	TID          string
	OldBalance   uint64
	NewBalance   uint64
	ApplicantID  string
	ApproverID   string
	Outcome      string
	ErrorMessage string
}

// HGCoinAccountResponse 返回 MySQL 权威余额。
type HGCoinAccountResponse struct {
	UserID    string `json:"userId"`
	Balance   string `json:"balance"`
	Authority string `json:"authority"`
}

// HGCoinMutationResponse 返回一次幂等资产命令的权威结果。
type HGCoinMutationResponse struct {
	Committed        bool   `json:"committed"`
	IdempotentReplay bool   `json:"idempotentReplay"`
	TransactionID    string `json:"transactionId"`
	BalanceAfter     string `json:"balanceAfter"`
}

// HGCoinTransactionItem 是运维端只读资产流水。
type HGCoinTransactionItem struct {
	TransactionID          string `json:"transactionId"`
	RequestID              string `json:"requestId"`
	Operation              string `json:"operation"`
	Amount                 string `json:"amount"`
	SignedDelta            string `json:"signedDelta"`
	BalanceAfter           string `json:"balanceAfter"`
	Reason                 string `json:"reason"`
	BusinessType           string `json:"businessType"`
	BusinessKey            string `json:"businessKey"`
	ReferenceTransactionID string `json:"referenceTransactionId,omitempty"`
	CreatedAt              string `json:"createdAt"`
}

// HGCoinTransactionListResponse 使用不透明复合游标分页，不返回实时总数。
type HGCoinTransactionListResponse struct {
	UserID     string                  `json:"userId"`
	List       []HGCoinTransactionItem `json:"list"`
	NextCursor string                  `json:"nextCursor"`
	HasMore    bool                    `json:"hasMore"`
}

// HGInteractionStreamStatus 返回固定重投影流的 checkpoint 和低基数累计指标。
type HGInteractionStreamStatus struct {
	Stream        string `json:"stream"`
	Checkpoint    string `json:"checkpoint"`
	Runs          string `json:"runs"`
	Rows          string `json:"rows"`
	Failures      string `json:"failures"`
	LeaseSkips    string `json:"leaseSkips"`
	DurationNanos string `json:"durationNanos"`
}

// HGKafkaLagItem 返回 group/topic 聚合 lag；不暴露 partition。
type HGKafkaLagItem struct {
	Group      string `json:"group"`
	Topic      string `json:"topic"`
	LagRecords string `json:"lagRecords"`
}

// HGKafkaStatus 描述应用已观察到的处理 lag，而非 committed-offset lag。
type HGKafkaStatus struct {
	Measurement        string           `json:"measurement"`
	AssignedPartitions string           `json:"assignedPartitions"`
	Items              []HGKafkaLagItem `json:"items"`
}

// HGAssetPipelineStatusResponse 聚合低成本进程内快照和四个固定 Redis checkpoint。
type HGAssetPipelineStatusResponse struct {
	ObservedAt               string                      `json:"observedAt"`
	CoinInitializerCursor    string                      `json:"coinInitializerCursor"`
	CoinReconciliationDrifts string                      `json:"coinReconciliationDrifts"`
	InteractionStreams       []HGInteractionStreamStatus `json:"interactionStreams"`
	Kafka                    HGKafkaStatus               `json:"kafka"`
}
