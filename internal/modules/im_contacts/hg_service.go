package im_contacts

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	HGErrInvalid = errors.New("联系人参数无效")
	HGErrDenied  = errors.New("联系人操作无权限")
	HGErrAbsent  = errors.New("联系人或申请不存在")
	HGErrState   = errors.New("联系人关系状态冲突")
)

// HGProfile 是对已登录用户可见的公开资料；userId 为内部 JWT 标识，不是公开系统编号。
type HGProfile struct {
	UserID       string `json:"userId"`                 // 内部 JWT 用户 ID，只能用于已授权业务请求。
	PublicID     string `json:"publicId"`               // 系统生成的不可修改公开编号。
	Username     string `json:"username"`               // 当前账号的自定义 ID，空表示未设置。
	Nickname     string `json:"nickname"`               // 用户昵称，空表示未填写。
	Signature    string `json:"signature"`              // 用户签名，空表示未填写。
	AvatarURL    string `json:"avatarUrl"`              // 用户资料中的头像访问 URL，空表示未设置；不返回账号私密身份字段。
	FriendStatus string `json:"friendStatus,omitempty"` // 搜索结果的本人关系：none/outgoing_pending/incoming_pending/friend/blocked；其他视图省略。
}

// HGFriend 为仅本人可读取的好友资料和私人备注；phone 是联系人备忘而非账号验证手机号。
type HGFriend struct {
	HGProfile
	Remark      string   `json:"remark"`      // 仅本人可见的备注，空时展示昵称。
	Tags        []string `json:"tags"`        // 仅本人可见的标签，最多六项。
	Phone       string   `json:"phone"`       // 私人联系号码备忘，不是账号手机号码。
	Description string   `json:"description"` // 仅本人可见的描述。
}

// HGRequest 是当前登录用户收到的待处理申请，历史申请不作为待办返回。
type HGRequest struct {
	HGProfile
	RequestID string    `json:"requestId"` // 不可猜测的申请业务 ID。
	Message   string    `json:"message"`   // 申请人填写的附言，最多 128 字符。
	CreatedAt time.Time `json:"createdAt"` // 申请创建时间，毫秒精度。
}

// HGBlock 是本人主动拉黑的用户记录；不暴露对方拉黑自己的名单。
type HGBlock struct {
	HGProfile
	BlockNote string    `json:"blockNote"` // 本人填写的原因备注。
	BlockedAt time.Time `json:"blockedAt"` // 本人拉黑时间，毫秒精度。
}

// HGPatch 是仅作用于本人好友备注的编辑协议，不修改对方账号资料。
type HGPatch struct {
	Remark      string   `json:"remark"`      // 单向备注，最多 64 字符。
	Tags        []string `json:"tags"`        // 单向标签，最多六项，每项最多 32 字符。
	Phone       string   `json:"phone"`       // 私人电话备忘，最多 32 字符。
	Description string   `json:"description"` // 单向描述，最多 128 字符。
}

// HGService 使用共享 MySQL 连接池管理联系人权威关系，不持有请求状态。
type HGService struct{ db *sql.DB }

func HGNewService(db *sql.DB) *HGService { return &HGService{db: db} }

func hgPair(a, b string) (string, string) {
	if a > b {
		return b, a
	}
	return a, b
}

func hgValidID(id string) bool { return len(id) > 0 && len(id) <= 128 }

// 遗留 users.user_name 未按公开 ID 协议约束；手机号/邮箱形态不得以 username 类型检索或作为公开字段展示。
func hgPublicUsername(value string) string {
	if len(value) < 4 || len(value) > 32 {
		return ""
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if i == 0 && !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return ""
		}
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && ((c >= '0' && c <= '9') || c == '_'))) {
			return ""
		}
	}
	return value
}

// HGList 读取有界第一页；用户规模超过百人时调用方需使用 after 游标逐页获取。
func (s *HGService) HGList(ctx context.Context, user string, after uint64) ([]HGFriend, error) {
	rows, err := s.db.QueryContext(ctx, SQL.HGIMContactFriendsSQL, user, user, user, user, after, 100)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGFriend, 0)
	for rows.Next() {
		var v HGFriend
		var tags string
		if err = rows.Scan(&v.UserID, &v.PublicID, &v.Username, &v.Nickname, &v.Signature, &v.AvatarURL, &v.Remark, &tags, &v.Phone, &v.Description); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(tags), &v.Tags); err != nil {
			return nil, err
		}
		v.Username = hgPublicUsername(v.Username)
		items = append(items, v)
	}
	return items, rows.Err()
}

// HGIncoming 只读取本人的待处理申请，稳定按申请主键递增游标分页。
func (s *HGService) HGIncoming(ctx context.Context, user string, after uint64) ([]HGRequest, error) {
	rows, err := s.db.QueryContext(ctx, SQL.HGIMContactRequestsSQL, user, after, 100)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGRequest, 0)
	for rows.Next() {
		var v HGRequest
		if err = rows.Scan(&v.RequestID, &v.UserID, &v.PublicID, &v.Username, &v.Nickname, &v.AvatarURL, &v.Message, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Username = hgPublicUsername(v.Username)
		items = append(items, v)
	}
	return items, rows.Err()
}

// HGBlocks 只读取本人主动拉黑的用户，稳定按 users.id 游标分页。
func (s *HGService) HGBlocks(ctx context.Context, user string, after uint64) ([]HGBlock, error) {
	rows, err := s.db.QueryContext(ctx, SQL.HGIMContactBlocksSQL, user, after, 100)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGBlock, 0)
	for rows.Next() {
		var v HGBlock
		if err = rows.Scan(&v.UserID, &v.PublicID, &v.Username, &v.Nickname, &v.AvatarURL, &v.BlockNote, &v.BlockedAt); err != nil {
			return nil, err
		}
		v.Username = hgPublicUsername(v.Username)
		items = append(items, v)
	}
	return items, rows.Err()
}

// HGProfileFor 仅返回公开资料，不返回电话、邮箱或他人的备注。
func (s *HGService) HGProfileFor(ctx context.Context, id string) (HGProfile, error) {
	var v HGProfile
	err := s.db.QueryRowContext(ctx, SQL.HGIMContactProfileSQL, id).Scan(&v.UserID, &v.PublicID, &v.Username, &v.Nickname, &v.Signature, &v.AvatarURL)
	if errors.Is(err, sql.ErrNoRows) {
		err = HGErrAbsent
	}
	v.Username = hgPublicUsername(v.Username)
	return v, err
}

// HGSearch 只允许已登录者按完整标识点查；手机号、邮箱不进入响应，依赖 IM 网关限流防止批量枚举。
func (s *HGService) HGSearch(ctx context.Context, viewer, kind, keyword string) (HGProfile, error) {
	var v HGProfile
	var row *sql.Row
	if !hgValidID(viewer) || keyword == "" || keyword != strings.TrimSpace(keyword) {
		return v, HGErrInvalid
	}
	switch kind {
	case "public_id":
		if len(keyword) != 21 || keyword[0] != 'U' {
			return v, HGErrInvalid
		}
		id, err := strconv.ParseUint(keyword[1:], 10, 64)
		if err != nil || id == 0 {
			return v, HGErrInvalid
		}
		row = s.db.QueryRowContext(ctx, SQL.HGIMContactSearchPublicSQL, id)
	case "username":
		if hgPublicUsername(keyword) == "" {
			return v, HGErrInvalid
		}
		row = s.db.QueryRowContext(ctx, SQL.HGIMContactSearchUsernameSQL, keyword)
	case "targetUserId":
		if !hgValidID(keyword) {
			return v, HGErrInvalid
		}
		row = s.db.QueryRowContext(ctx, SQL.HGIMContactProfileSQL, keyword)
	case "phone":
		if len(keyword) != 11 || keyword[0] != '1' || keyword[1] < '3' || keyword[1] > '9' {
			return v, HGErrInvalid
		}
		for _, digit := range keyword {
			if digit < '0' || digit > '9' {
				return v, HGErrInvalid
			}
		}
		row = s.db.QueryRowContext(ctx, SQL.HGIMContactSearchPhoneSQL, keyword)
	case "email":
		if len(keyword) > 254 || strings.ContainsAny(keyword, "\r\n") {
			return v, HGErrInvalid
		}
		address, err := mail.ParseAddress(keyword)
		if err != nil || address.Address != keyword || !strings.Contains(keyword[strings.LastIndexByte(keyword, '@')+1:], ".") {
			return v, HGErrInvalid
		}
		row = s.db.QueryRowContext(ctx, SQL.HGIMContactSearchEmailSQL, keyword)
	default:
		return v, HGErrInvalid
	}
	err := row.Scan(&v.UserID, &v.PublicID, &v.Username, &v.Nickname, &v.Signature, &v.AvatarURL)
	if errors.Is(err, sql.ErrNoRows) {
		err = HGErrAbsent
	}
	if err != nil {
		return v, err
	}
	v.Username = hgPublicUsername(v.Username)
	if v.UserID == viewer {
		v.FriendStatus = "friend"
		return v, nil
	}
	low, high := hgPair(viewer, v.UserID)
	var friend, blocked, outgoing, incoming bool
	if err = s.db.QueryRowContext(ctx, SQL.HGIMContactStatusSQL, low, high, viewer, v.UserID, v.UserID, viewer, viewer, v.UserID, v.UserID, viewer).Scan(&friend, &blocked, &outgoing, &incoming); err != nil {
		return v, err
	}
	switch {
	case blocked:
		v.FriendStatus = "blocked"
	case friend:
		v.FriendStatus = "friend"
	case incoming:
		v.FriendStatus = "incoming_pending"
	case outgoing:
		v.FriendStatus = "outgoing_pending"
	default:
		v.FriendStatus = "none"
	}
	return v, nil
}

func hgLockPair(ctx context.Context, tx *sql.Tx, a, b string) error {
	low, high := hgPair(a, b)
	for _, id := range []string{low, high} {
		var found int
		if err := tx.QueryRowContext(ctx, SQL.HGIMContactLockUserSQL, id).Scan(&found); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return HGErrAbsent
			}
			return err
		}
	}
	return nil
}

func hgCheckBlock(ctx context.Context, tx *sql.Tx, a, b string) error {
	var found int
	err := tx.QueryRowContext(ctx, SQL.HGIMContactBlockPairSQL, a, b, b, a).Scan(&found)
	if err == nil {
		return HGErrDenied
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func hgFriend(ctx context.Context, tx *sql.Tx, a, b string) error {
	low, high := hgPair(a, b)
	var found int
	err := tx.QueryRowContext(ctx, SQL.HGIMContactPairSQL, low, high).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return HGErrDenied
	}
	return err
}

// HGSendRequest 在按用户 ID 固定顺序加锁后创建申请；反向申请等价于双方同意。
func (s *HGService) HGSendRequest(ctx context.Context, user, target, message string) error {
	if !hgValidID(user) || !hgValidID(target) || user == target || utf8.RuneCountInString(message) > 128 {
		return HGErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = hgLockPair(ctx, tx, user, target); err != nil {
		return err
	}
	if err = hgCheckBlock(ctx, tx, user, target); err != nil {
		return err
	}
	if err = hgFriend(ctx, tx, user, target); err == nil {
		return HGErrState
	} else if !errors.Is(err, HGErrDenied) {
		return err
	}
	var id, requester, addressee string
	err = tx.QueryRowContext(ctx, SQL.HGIMContactPendingSQL, user, target, target, user).Scan(&id, &requester, &addressee)
	if err == nil {
		if requester == user {
			return HGErrState
		}
		if err = hgAcceptTx(ctx, tx, id, requester, addressee); err != nil {
			return err
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return err
	}
	key := hex.EncodeToString(raw[:])
	id = fmt.Sprintf("%s-%s-%s-%s-%s", key[:8], key[8:12], key[12:16], key[16:20], key[20:])
	if _, err = tx.ExecContext(ctx, SQL.HGIMContactInsertRequestSQL, id, user, target, message); err != nil {
		return err
	}
	return tx.Commit()
}

// HGDecide 仅申请接收者可接受/拒绝；重复提交及跨用户操作均失败关闭。
func (s *HGService) HGDecide(ctx context.Context, user, id string, accept bool) (HGProfile, error) {
	var empty HGProfile
	if len(id) != 36 || !hgValidID(user) {
		return empty, HGErrInvalid
	}
	// 先只读查找申请方向，再按用户固定顺序锁定，避免与申请/删除并发造成死锁。
	var a, b string
	var status int
	if err := s.db.QueryRowContext(ctx, SQL.HGIMContactRequestSQL, id).Scan(&a, &b, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return empty, HGErrAbsent
		}
		return empty, err
	}
	if b != user {
		return empty, HGErrDenied
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err = hgLockPair(ctx, tx, a, b); err != nil {
		return empty, err
	}
	if err = tx.QueryRowContext(ctx, SQL.HGIMContactLockRequestSQL, id).Scan(&a, &b, &status); err != nil {
		return empty, err
	}
	if b != user || status != 0 {
		return empty, HGErrState
	}
	if accept {
		if err = hgCheckBlock(ctx, tx, a, b); err != nil {
			return empty, err
		}
		err = hgAcceptTx(ctx, tx, id, a, b)
	} else {
		_, err = tx.ExecContext(ctx, SQL.HGIMContactDecideRequestSQL, 2, id)
	}
	if err != nil {
		return empty, err
	}
	if accept {
		if err = tx.QueryRowContext(ctx, SQL.HGIMContactProfileSQL, a).Scan(&empty.UserID, &empty.PublicID, &empty.Username, &empty.Nickname, &empty.Signature, &empty.AvatarURL); err != nil {
			return HGProfile{}, err
		}
		empty.Username = hgPublicUsername(empty.Username)
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return empty, nil
}

func hgAcceptTx(ctx context.Context, tx *sql.Tx, id, a, b string) error {
	low, high := hgPair(a, b)
	if _, err := tx.ExecContext(ctx, SQL.HGIMContactAcceptPairSQL, low, high); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, SQL.HGIMContactDecideRequestSQL, 1, id); err != nil {
		return err
	}
	// 延续旧会话主键及成员格式，接受申请与会话建立必须在同一事务。
	result, err := tx.ExecContext(ctx, SQL.HGIMCreateConversationSQL, low, high)
	if err != nil {
		return err
	}
	conversation, err := result.LastInsertId()
	if err != nil {
		return err
	}
	for _, member := range []string{low, high} {
		if _, err = tx.ExecContext(ctx, SQL.HGIMAddMemberSQL, conversation, member); err != nil {
			return err
		}
	}
	return nil
}

// HGEdit 只允许本人修改有效好友的私人备注，不修改对方登录身份或隐私字段。
func (s *HGService) HGEdit(ctx context.Context, user, target string, patch HGPatch) error {
	if !hgValidID(user) || !hgValidID(target) || user == target || utf8.RuneCountInString(patch.Remark) > 64 || utf8.RuneCountInString(patch.Description) > 128 || len(patch.Phone) > 32 || len(patch.Tags) > 6 {
		return HGErrInvalid
	}
	for _, tag := range patch.Tags {
		if utf8.RuneCountInString(tag) > 32 || strings.TrimSpace(tag) == "" {
			return HGErrInvalid
		}
	}
	tags, err := json.Marshal(patch.Tags)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = hgLockPair(ctx, tx, user, target); err != nil {
		return err
	}
	if err = hgFriend(ctx, tx, user, target); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGIMContactUpsertRemarkSQL, user, target, patch.Remark, tags, patch.Phone, patch.Description); err != nil {
		return err
	}
	return tx.Commit()
}

// HGRemove 删除当前关系但不删除历史消息及申请记录。
func (s *HGService) HGRemove(ctx context.Context, user, target string) error {
	if !hgValidID(user) || !hgValidID(target) || user == target {
		return HGErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = hgLockPair(ctx, tx, user, target); err != nil {
		return err
	}
	low, high := hgPair(user, target)
	result, err := tx.ExecContext(ctx, SQL.HGIMContactDeletePairSQL, low, high)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return HGErrAbsent
	}
	return tx.Commit()
}

// HGSetBlock 只设置本人朝向目标的黑名单；解除拉黑不恢复先前删除的好友关系。
func (s *HGService) HGSetBlock(ctx context.Context, user, target string, block bool) error {
	if !hgValidID(user) || !hgValidID(target) || user == target {
		return HGErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = hgLockPair(ctx, tx, user, target); err != nil {
		return err
	}
	if block {
		if _, err = tx.ExecContext(ctx, SQL.HGIMContactBlockSQL, user, target, "手动加入黑名单"); err != nil {
			return err
		}
		low, high := hgPair(user, target)
		if _, err = tx.ExecContext(ctx, SQL.HGIMContactDeletePairSQL, low, high); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, SQL.HGIMContactRejectPendingSQL, user, target, target, user); err != nil {
			return err
		}
	} else {
		if _, err = tx.ExecContext(ctx, SQL.HGIMContactUnblockSQL, user, target); err != nil {
			return err
		}
	}
	return tx.Commit()
}
