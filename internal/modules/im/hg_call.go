package im

import (
	"MLC_GO/internal/events"
	IMEvents "MLC_GO/internal/events/im"
	"MLC_GO/internal/modules/im/transport"
	"MLC_GO/internal/outbox"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"time"
)

// HGCalls 是单聊呼叫权威服务；多设备接听由 MySQL 行锁裁决，不把 Token 签发视为媒体连接成功。
type HGCalls struct {
	service   *HGService           // 复用设备凭据验证、MySQL 和 Outbox topic。
	livekit   *transport.HGLiveKit // 外部房间控制与签名适配器。
	publicURL string               // 供客户端连接的 LiveKit ws(s) 地址。
}

func HGNewCalls(service *HGService, livekit *transport.HGLiveKit, publicURL string) *HGCalls {
	return &HGCalls{service: service, livekit: livekit, publicURL: publicURL}
}

// HGCallCreate 是幂等呼叫协议；caller 从 JWT 注入，设备必须验证随机 secret。
type HGCallCreate struct {
	HGDeviceCredential
	ConversationID string `json:"conversationId"` // 当前单聊会话。
	RequestID      string `json:"requestId"`      // 1..64 可打印 ASCII，重试保持不变。
	MediaType      string `json:"mediaType"`      // audio/video。
}

// HGCallView 为通话权威状态，accepted 表示业务接听而不是 WebRTC 已建连。
type HGCallView struct {
	CallID         string    `json:"callId"`         // 随机业务 ID。
	ConversationID string    `json:"conversationId"` // 单聊 ID。
	CallerID       string    `json:"callerId"`       // 主叫用户。
	CalleeID       string    `json:"calleeId"`       // 被叫用户。
	MediaType      string    `json:"mediaType"`      // audio/video。
	State          string    `json:"state"`          // ringing/accepted/ended/rejected/cancelled/timeout。
	Version        uint64    `json:"version"`        // 状态版本，客户端忽略旧版事件。
	ExpiresAt      time.Time `json:"expiresAt"`      // 振铃/通话硬上限。
	callerDevice   []byte    // 主叫绑定设备，不输出。
	calleeDevice   []byte    // 接听获胜设备，不输出。
}

func hgScanCall(row *sql.Row) (HGCallView, error) {
	var call HGCallView
	var id []byte
	var conversation uint64
	var expiryMS uint64
	err := row.Scan(&id, &conversation, &call.CallerID, &call.CalleeID, &call.callerDevice, &call.calleeDevice, &call.MediaType, &call.State, &call.Version, &expiryMS)
	call.CallID = hex.EncodeToString(id)
	call.ConversationID = strconv.FormatUint(conversation, 10)
	if err == nil {
		call.ExpiresAt = time.UnixMilli(int64(expiryMS)).UTC()
	}
	return call, err
}
func (s *HGCalls) hgEvent(ctx context.Context, tx *sql.Tx, call HGCallView) error {
	conversation, err := hgIMID(call.ConversationID)
	if err != nil {
		return err
	}
	return outbox.NewRepository(s.service.db, s.service.topic).SaveTx(ctx, tx, IMEvents.HGCallChangedEvent{EventMeta: events.NewEventMeta(ctx), CallID: call.CallID, ConversationID: conversation, VersionID: call.Version})
}

// HGCreate 同事务建立忙线占位、呼叫记录与信令 Outbox；并发呼叫只有一个可占用双方。
func (s *HGCalls) HGCreate(ctx context.Context, user string, cmd HGCallCreate) (HGCallView, error) {
	var empty HGCallView
	device, err := s.service.hgDevice(ctx, user, cmd.HGDeviceCredential)
	if err != nil {
		return empty, err
	}
	conversation, err := hgIMID(cmd.ConversationID)
	if err != nil || len(cmd.RequestID) < 1 || len(cmd.RequestID) > 64 || !hgIMASCII(cmd.RequestID) || (cmd.MediaType != "audio" && cmd.MediaType != "video") {
		return empty, HGErrInvalid
	}
	tx, err := s.service.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	var low, high string
	var seq uint64
	if err = tx.QueryRowContext(ctx, SQL.HGIMLockConversationSQL, conversation).Scan(&low, &high, &seq); err != nil {
		return empty, HGErrForbidden
	}
	if user != low && user != high {
		return empty, HGErrForbidden
	}
	var member int
	if err = tx.QueryRowContext(ctx, SQL.HGIMConversationMemberSQL, conversation, user).Scan(&member); err != nil {
		return empty, HGErrForbidden
	}
	existing, err := hgScanCall(tx.QueryRowContext(ctx, SQL.HGIMCallRequestSQL, user, cmd.RequestID))
	if err == nil {
		if existing.ConversationID != cmd.ConversationID || existing.MediaType != cmd.MediaType || !bytes.Equal(existing.callerDevice, device) {
			return empty, HGErrConflict
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return empty, err
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return empty, err
	}
	peer := low
	if peer == user {
		peer = high
	}
	for _, target := range []string{low, high} {
		if _, err = tx.ExecContext(ctx, SQL.HGIMCallBusySQL, target, id[:]); err != nil {
			return empty, hgCallWriteError(err)
		}
	}
	if _, err = tx.ExecContext(ctx, SQL.HGIMCallInsertSQL, id[:], conversation, user, peer, cmd.RequestID, device, cmd.MediaType); err != nil {
		return empty, hgCallWriteError(err)
	}
	call, err := hgScanCall(tx.QueryRowContext(ctx, SQL.HGIMCallGetSQL, id[:]))
	if err != nil {
		return empty, err
	}
	if err = s.hgEvent(ctx, tx, call); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return call, nil
}

// HGGet 每次按当前会话成员重新授权，不凭可猜测的 callId 放行。
func (s *HGCalls) HGGet(ctx context.Context, user, raw string) (HGCallView, error) {
	id, err := hgRandomID(raw)
	if err != nil {
		return HGCallView{}, err
	}
	call, err := hgScanCall(s.service.db.QueryRowContext(ctx, SQL.HGIMCallGetSQL, id))
	if err != nil {
		return call, err
	}
	if user != call.CallerID && user != call.CalleeID {
		return HGCallView{}, HGErrForbidden
	}
	var member int
	if err = s.service.db.QueryRowContext(ctx, SQL.HGIMConversationMemberSQL, call.ConversationID, user).Scan(&member); err != nil {
		return HGCallView{}, HGErrForbidden
	}
	return call, nil
}

// HGChange accept/reject/cancel/end 采用单行状态机；同设备重试幂等，其他设备不能夺取已接听的呼叫。
func (s *HGCalls) HGChange(ctx context.Context, user, raw, action string, credential HGDeviceCredential) (HGCallView, error) {
	device, err := s.service.hgDevice(ctx, user, credential)
	if err != nil {
		return HGCallView{}, err
	}
	return s.hgChange(ctx, user, raw, action, device, false)
}
func (s *HGCalls) hgChange(ctx context.Context, user, raw, action string, device []byte, expiry bool) (HGCallView, error) {
	id, err := hgRandomID(raw)
	if err != nil {
		return HGCallView{}, err
	}
	tx, err := s.service.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGCallView{}, err
	}
	defer tx.Rollback()
	call, err := hgScanCall(tx.QueryRowContext(ctx, SQL.HGIMCallLockSQL, id))
	if err != nil {
		return call, err
	}
	if !expiry {
		if user != call.CallerID && user != call.CalleeID {
			return HGCallView{}, HGErrForbidden
		}
		var member int
		if err = tx.QueryRowContext(ctx, SQL.HGIMConversationMemberSQL, call.ConversationID, user).Scan(&member); err != nil {
			return HGCallView{}, HGErrForbidden
		}
	}
	terminal := call.State != "ringing" && call.State != "accepted"
	state := ""
	switch action {
	case "accept":
		if user != call.CalleeID {
			return call, HGErrForbidden
		}
		if call.State == "accepted" && bytes.Equal(call.calleeDevice, device) {
			return call, nil
		}
		if call.State != "ringing" {
			return call, HGErrConflict
		}
		result, err := tx.ExecContext(ctx, SQL.HGIMCallAcceptSQL, device, id)
		if err != nil {
			return call, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return call, err
		}
		if n != 1 {
			return call, HGErrConflict
		}
	case "reject":
		if user != call.CalleeID {
			return call, HGErrForbidden
		}
		state = "rejected"
	case "cancel":
		if user != call.CallerID || !bytes.Equal(device, call.callerDevice) {
			return call, HGErrForbidden
		}
		state = "cancelled"
	case "end":
		bound := call.callerDevice
		if user == call.CalleeID {
			bound = call.calleeDevice
		}
		if !bytes.Equal(device, bound) {
			return call, HGErrForbidden
		}
		state = "ended"
	case "timeout":
		if !expiry {
			return call, HGErrForbidden
		}
		var expired bool
		if err = tx.QueryRowContext(ctx, SQL.HGIMCallExpiredFlagSQL, id).Scan(&expired); err != nil {
			return call, err
		}
		if terminal || !expired {
			return call, nil
		}
		state = "timeout"
	default:
		return call, HGErrInvalid
	}
	if state != "" {
		if terminal {
			if call.State == state {
				return call, nil
			}
			return call, HGErrConflict
		}
		if (action == "reject" || action == "cancel") && call.State != "ringing" {
			return call, HGErrConflict
		}
		if action == "end" && call.State != "accepted" {
			return call, HGErrConflict
		}
		if _, err = tx.ExecContext(ctx, SQL.HGIMCallEndSQL, state, id); err != nil {
			return call, err
		}
		users := []string{call.CallerID, call.CalleeID}
		sort.Strings(users)
		for _, target := range users {
			if _, err = tx.ExecContext(ctx, SQL.HGIMCallFreeSQL, target, id); err != nil {
				return call, err
			}
		}
	}
	call, err = hgScanCall(tx.QueryRowContext(ctx, SQL.HGIMCallGetSQL, id))
	if err != nil {
		return call, err
	}
	if err = s.hgEvent(ctx, tx, call); err != nil {
		return call, err
	}
	if err = tx.Commit(); err != nil {
		return call, err
	}
	return call, nil
}

// HGToken 房间创建在事务外；随后重新锁定业务状态与设备，签发一分钟最小权限 Token。
func (s *HGCalls) HGToken(ctx context.Context, user, raw string, credential HGDeviceCredential) (map[string]any, error) {
	device, err := s.service.hgDevice(ctx, user, credential)
	if err != nil {
		return nil, err
	}
	call, err := s.HGGet(ctx, user, raw)
	if err != nil {
		return nil, err
	}
	if call.State != "accepted" {
		return nil, HGErrConflict
	}
	boundDevice := call.callerDevice
	if user == call.CalleeID {
		boundDevice = call.calleeDevice
	}
	if !bytes.Equal(boundDevice, device) {
		return nil, HGErrForbidden
	}
	roomSID, err := s.livekit.HGCreateRoom(ctx, "im_"+raw)
	if err != nil {
		return nil, err
	}
	id, _ := hgRandomID(raw)
	tx, err := s.service.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	call, err = hgScanCall(tx.QueryRowContext(ctx, SQL.HGIMCallLockSQL, id))
	if err != nil {
		return nil, err
	}
	bound := call.callerDevice
	if user == call.CalleeID {
		bound = call.calleeDevice
	}
	if call.State != "accepted" || !bytes.Equal(bound, device) {
		return nil, HGErrConflict
	}
	var hgMember, hgActive int
	if err = tx.QueryRowContext(ctx, SQL.HGIMCallMemberSQL, call.ConversationID, user).Scan(&hgMember); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, HGErrForbidden
		}
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, SQL.HGIMCallDeviceActiveSQL, device, user).Scan(&hgActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, HGErrForbidden
		}
		return nil, err
	}
	var hgExpired bool
	if err = tx.QueryRowContext(ctx, SQL.HGIMCallExpiredFlagSQL, id).Scan(&hgExpired); err != nil {
		return nil, err
	}
	if hgExpired {
		return nil, HGErrConflict
	}
	// 已持有呼叫行锁并按数据库时钟校验；重复取令牌可能未改变 cleanup_until，零行变更也是成功。
	if _, err = tx.ExecContext(ctx, SQL.HGIMCallTokenWindowSQL, id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGIMCallSetRoomSIDSQL, roomSID, id); err != nil {
		return nil, err
	}
	token, err := s.livekit.HGJoinToken("im_"+raw, "im/"+hex.EncodeToString(device), call.MediaType)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"token": token, "serverUrl": s.publicURL, "expiresIn": 60, "room": "im_" + raw}, nil
}

// HGMaintainOnce 超时状态写 Outbox；终态在 cleanup_until 前重复清理房间。
// 该窗口仅覆盖 Go 初始令牌；LiveKit 自主续签可超过此窗口，尚不构成可靠撤权，必须保持上线门禁。
func (s *HGCalls) HGMaintainOnce(ctx context.Context) error {
	for _, state := range []string{"ringing", "accepted"} {
		var id []byte
		err := s.service.db.QueryRowContext(ctx, SQL.HGIMCallExpiredSQL, state).Scan(&id)
		if err == nil {
			if _, err = s.hgChange(ctx, "", hex.EncodeToString(id), "timeout", nil, true); err != nil {
				return err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	rows, err := s.service.db.QueryContext(ctx, SQL.HGIMCallCleanupSQL)
	if err != nil {
		return err
	}
	var ids [][]byte
	for rows.Next() {
		var id []byte
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	var first error
	for _, id := range ids {
		if err = s.livekit.HGDeleteRoom(ctx, "im_"+hex.EncodeToString(id)); err != nil {
			if first == nil {
				first = err
			}
			if _, dbErr := s.service.db.ExecContext(ctx, SQL.HGIMCallCleanupRetrySQL, id); dbErr != nil {
				return dbErr
			}
			continue
		}
		if _, err = s.service.db.ExecContext(ctx, SQL.HGIMCallCleanedSQL, id); err != nil {
			return err
		}
	}
	return first
}
