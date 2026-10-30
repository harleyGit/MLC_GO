package im

import (
	"MLC_GO/internal/modules/im/transport"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// HGPrivateMedia 管理私有附件；审核器是受信的独立计算服务，未得到真实验证结果一律不开放。
type HGPrivateMedia struct {
	db            *sql.DB                // 权威附件元数据。
	storage       *transport.HGSeaweedFS // 私有 S3 对象适配器。
	reviewer      *transport.HGReviewer  // 内容/病毒/格式验证与不可变封存服务。
	bucket        string                 // 受信私有桶名称，不能由客户端指定。
	downloadSlots chan struct{}          // 每进程最多十六条媒体读取，满载立即拒绝，不累积等待队列。
}

func HGNewPrivateMedia(db *sql.DB, storage *transport.HGSeaweedFS, reviewer *transport.HGReviewer, bucket string) *HGPrivateMedia {
	return &HGPrivateMedia{db: db, storage: storage, reviewer: reviewer, bucket: bucket, downloadSlots: make(chan struct{}, 16)}
}

// HGUploadCommand 是上传初始化协议；SHA256/MIME/大小均须由审核器重新验证，不能凭声明通过。
type HGUploadCommand struct {
	ConversationID string        `json:"conversationId"` // 必须是上传者的单聊。
	Type           HGMessageType `json:"messageType"`    // image/video/audio/file。
	MIME           string        `json:"mime"`           // 允许列表内的 MIME。
	Size           int64         `json:"size"`           // 申请字节数，1..128MiB，图片最多10MiB。
	SHA256         string        `json:"sha256"`         // 小写 SHA256，用于不可变内容绑定。
}

type hgAttachment struct {
	owner        string        // 上传者。
	conversation uint64        // 绑定会话。
	key          string        // 隔离对象。
	sealed       string        // 审核后不可变对象。
	kind         HGMessageType // 媒体种类。
	mime         string        // 真实格式要求。
	size         int64         // 字节数。
	hash         []byte        // 真实审核要求的 SHA256。
	state        string        // uploading/verifying/ready/rejected/deleted。
	width        int           // 审核宽，像素。
	height       int           // 审核高，像素。
	duration     int64         // 审核时长，毫秒。
}

func hgScanAttachment(row *sql.Row) (hgAttachment, error) {
	var a hgAttachment
	err := row.Scan(&a.owner, &a.conversation, &a.key, &a.sealed, &a.kind, &a.mime, &a.size, &a.hash, &a.state, &a.width, &a.height, &a.duration)
	return a, err
}

func hgMediaMIME(kind HGMessageType, mime string) bool {
	switch kind {
	case HGImage:
		return mime == "image/jpeg" || mime == "image/png" || mime == "image/webp"
	case HGVideo:
		return mime == "video/mp4" || mime == "video/quicktime"
	case HGAudio:
		return mime == "audio/mpeg" || mime == "audio/ogg" || mime == "audio/mp4" || mime == "audio/aac"
	case HGFile:
		return mime == "application/pdf" || mime == "application/zip" || mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return false
}

// HGInitUpload 最多预留 100 个对象且累计申请不超过 512MiB；配额不因失败自动释放，清理需独立审计。
func (s *HGPrivateMedia) HGInitUpload(ctx context.Context, user string, cmd HGUploadCommand) (map[string]any, error) {
	conversation, err := hgIMID(cmd.ConversationID)
	if err != nil {
		return nil, err
	}
	hash, err := hex.DecodeString(cmd.SHA256)
	if err != nil || len(hash) != 32 || cmd.SHA256 != hex.EncodeToString(hash) || !hgMediaMIME(cmd.Type, cmd.MIME) || cmd.Size < 1 || cmd.Size > 128<<20 || (cmd.Type == HGImage && cmd.Size > 10<<20) {
		return nil, HGErrInvalid
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	key := "quarantine/" + hex.EncodeToString(id[:])
	link, err := s.storage.HGUploadURL(key, cmd.MIME, cmd.Size)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var found int
	if err = tx.QueryRowContext(ctx, SQL.HGIMLockDeviceOwnerSQL, user).Scan(&found); err != nil {
		return nil, HGErrForbidden
	}
	if err = tx.QueryRowContext(ctx, SQL.HGIMConversationMemberSQL, conversation, user).Scan(&found); err != nil {
		return nil, HGErrForbidden
	}
	rows, err := tx.QueryContext(ctx, SQL.HGIMMediaQuotaSQL, user)
	if err != nil {
		return nil, err
	}
	var total int64
	count := 0
	for rows.Next() {
		var size int64
		if err = rows.Scan(&size); err != nil {
			break
		}
		total += size
		count++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count >= 100 || total+cmd.Size > 512<<20 {
		return nil, HGErrConflict
	}
	if _, err = tx.ExecContext(ctx, SQL.HGIMMediaInsertSQL, id[:], user, conversation, key, cmd.Type, cmd.MIME, cmd.Size, hash); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"mediaId": hex.EncodeToString(id[:]), "uploadUrl": link, "method": "PUT", "expiresIn": 600, "headers": map[string]string{"Content-Type": cmd.MIME, "x-amz-acl": "private"}, "size": cmd.Size}, nil
}

// HGCompleteUpload 仅接受上传完成声明，不代表通过审核；过期上传不得重新延长凭证。
func (s *HGPrivateMedia) HGCompleteUpload(ctx context.Context, user, raw string) error {
	id, err := hgRandomID(raw)
	if err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, SQL.HGIMMediaCompleteSQL, id, user); err != nil {
		return err
	}
	a, err := hgScanAttachment(s.db.QueryRowContext(ctx, SQL.HGIMMediaGetSQL, id))
	if err != nil {
		return err
	}
	if a.owner != user {
		return HGErrForbidden
	}
	if a.state != "verifying" && a.state != "ready" {
		return HGErrConflict
	}
	return nil
}

func (s *HGPrivateMedia) HGAuthorizeSend(ctx context.Context, user, raw string, kind HGMessageType) error {
	id, err := hgRandomID(raw)
	if err != nil {
		return err
	}
	a, err := hgScanAttachment(s.db.QueryRowContext(ctx, SQL.HGIMMediaGetSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		return HGErrForbidden
	}
	if err != nil {
		return err
	}
	if a.owner != user || a.kind != kind || a.state != "ready" {
		return HGErrForbidden
	}
	return nil
}

// HGAttachTx 与消息在同一事务锁定审核状态并建立关联，避免授权检查与提交之间的撤销竞态。
func (s *HGPrivateMedia) HGAttachTx(ctx context.Context, tx *sql.Tx, user string, conversation, message uint64, kind HGMessageType, content []byte) error {
	var value HGMediaContent
	if json.Unmarshal(content, &value) != nil {
		return HGErrInvalid
	}
	id, err := hgRandomID(value.MediaID)
	if err != nil {
		return err
	}
	a, err := hgScanAttachment(tx.QueryRowContext(ctx, SQL.HGIMMediaLockSQL, id))
	if err != nil {
		return err
	}
	if a.owner != user || a.conversation != conversation || a.state != "ready" || a.kind != kind || a.width != value.Width || a.height != value.Height || a.duration != value.DurationMS {
		return HGErrForbidden
	}
	_, err = tx.ExecContext(ctx, SQL.HGIMMediaLinkSQL, message, id, conversation)
	return err
}

// HGReviewOnce 单次一件有界审核；崩溃后租约到期重试，十二次后 fail-closed，不自动通过。
func (s *HGPrivateMedia) HGReviewOnce(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, hash []byte
	var key, mime string
	var size int64
	var kind HGMessageType
	var attempts int
	err = tx.QueryRowContext(ctx, SQL.HGIMMediaClaimSQL).Scan(&id, &key, &mime, &size, &hash, &kind, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var lease [16]byte
	if _, err = rand.Read(lease[:]); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGIMMediaLeaseSQL, lease[:], id); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	state := "verifying"
	sealed := ""
	var result transport.HGReviewResult
	var reviewErr error
	if attempts >= 12 {
		state = "rejected"
	} else {
		result, err = s.reviewer.HGReview(ctx, hex.EncodeToString(id), s.bucket, key, hex.EncodeToString(hash), mime, size)
		reviewErr = err
		if err == nil {
			switch result.Status {
			case "ready":
				valid := result.SHA256 == hex.EncodeToString(hash) && result.Size == size && result.MIME == mime && result.SealedKey == "sealed/"+hex.EncodeToString(id)
				if kind == HGImage || kind == HGVideo {
					valid = valid && result.Width > 0 && result.Width <= 16384 && result.Height > 0 && result.Height <= 16384
				} else {
					valid = valid && result.Width == 0 && result.Height == 0
				}
				if kind == HGVideo || kind == HGAudio {
					valid = valid && result.DurationMS > 0 && result.DurationMS <= 3600000
				} else {
					valid = valid && result.DurationMS == 0
				}
				if valid {
					state = "ready"
					sealed = result.SealedKey
				} else {
					state = "rejected"
				}
			case "rejected":
				state = "rejected"
			case "pending":
			default:
				state = "rejected"
			}
		}
		if state == "verifying" && attempts+1 >= 12 {
			state = "rejected"
		}
	}
	if state != "ready" {
		result = transport.HGReviewResult{}
	}
	_, err = s.db.ExecContext(ctx, SQL.HGIMMediaReviewedSQL, state, sealed, result.Width, result.Height, result.DurationMS, id, lease[:])
	return errors.Join(err, reviewErr)
}

// HGMediaStatus 只向上传者返回审核元数据；接收方应通过已关联消息访问。
func (s *HGPrivateMedia) HGMediaStatus(ctx context.Context, user, raw string) (map[string]any, error) {
	id, err := hgRandomID(raw)
	if err != nil {
		return nil, err
	}
	a, err := hgScanAttachment(s.db.QueryRowContext(ctx, SQL.HGIMMediaGetSQL, id))
	if err != nil {
		return nil, err
	}
	if a.owner != user {
		return nil, HGErrForbidden
	}
	return map[string]any{"mediaId": raw, "state": a.state, "width": a.width, "height": a.height, "durationMs": a.duration}, nil
}

func (s *HGPrivateMedia) hgReadable(ctx context.Context, user, raw string) (hgAttachment, error) {
	id, err := hgRandomID(raw)
	if err != nil {
		return hgAttachment{}, err
	}
	a, err := hgScanAttachment(s.db.QueryRowContext(ctx, SQL.HGIMMediaGetSQL, id))
	if err != nil {
		return a, err
	}
	if a.state != "ready" || a.sealed == "" {
		return a, HGErrForbidden
	}
	var found int
	if err = s.db.QueryRowContext(ctx, SQL.HGIMConversationMemberSQL, a.conversation, user).Scan(&found); err != nil {
		return a, HGErrForbidden
	}
	if user != a.owner {
		if err = s.db.QueryRowContext(ctx, SQL.HGIMMediaReadableSQL, id, a.conversation).Scan(&found); err != nil {
			return a, HGErrForbidden
		}
	}
	return a, nil
}

var _ HGMediaAuthorizer = (*HGPrivateMedia)(nil)

// HGDelete 撤销本人媒体的未来读取与发送；物理对象由审核后的独立保留策略回收，配额不自动释放。
func (s *HGPrivateMedia) HGDelete(ctx context.Context, user, raw string) error {
	id, err := hgRandomID(raw)
	if err != nil {
		return err
	}
	a, err := hgScanAttachment(s.db.QueryRowContext(ctx, SQL.HGIMMediaGetSQL, id))
	if err != nil {
		return err
	}
	if a.owner != user {
		return HGErrForbidden
	}
	_, err = s.db.ExecContext(ctx, SQL.HGIMMediaDeleteSQL, id, user)
	return err
}
