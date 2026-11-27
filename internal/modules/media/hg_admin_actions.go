package media

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	Utils "MLC_GO/internal/pkg/utils"
	HGResponse "MLC_GO/internal/response"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type hgAdminPreviewResult struct {
	URL       string `json:"preview_url"` // 封存对象短期 GET URL，不可缓存或落日志。
	MIME      string `json:"mime_type"`   // 审核与申请均匹配的 MIME；前端仅用对应媒体元素播放。
	ExpiresIn int    `json:"expires_in"`  // 图片 30 秒，音视频 300 秒，以支持浏览器分段读取。
}

type hgAdminDeleteResult struct {
	AssetID string `json:"asset_id"` // 本次提交的新媒体 UUID。
	Status  string `json:"status"`   // deleting 表示已入队，deleted 表示此前已清理；不是同步删除。
}

func hgCanonicalAssetID(asset string) bool {
	id, err := uuid.Parse(asset)
	return err == nil && id != uuid.Nil && id.String() == asset
}

// 音视频草稿只允许原上传者处理；不把管理员角色作视频稿件、IM 会话或其他私有数据的业务授权。
func hgAdminAVDraftPurpose(purpose string) bool {
	switch purpose {
	case "video_source", "comment_video", "chat_audio", "chat_video":
		return true
	default:
		return false
	}
}

func (s *HGService) hgPreviewAdminAsset(ctx context.Context, actor, asset string) (hgAdminPreviewResult, error) {
	var result hgAdminPreviewResult
	if !hgCanonicalAssetID(asset) || actor == "" {
		return result, HGErrInvalid
	}
	if s.previewRead == nil {
		return result, errors.New("媒体预览未配置")
	}
	var owner, purpose, mime, status, bucket, session string
	var key sql.NullString
	var referenced bool
	err := s.db.QueryRowContext(ctx, SQL.HGMediaAdminPreviewSQL, asset).
		Scan(&owner, &purpose, &mime, &status, &bucket, &key, &session, &referenced)
	if errors.Is(err, sql.ErrNoRows) {
		return result, HGErrNotFound
	}
	if err != nil {
		return result, err
	}
	sealedKey := "sealed/" + strings.ReplaceAll(asset, "-", "")
	if owner == "" || bucket != s.bucket || status != "ready" || !key.Valid || key.String != sealedKey ||
		(session != "ready" && session != "attached") || hgPolicies[purpose].mimes[mime] == "" {
		return result, HGErrNotFound
	}
	var ttl time.Duration
	switch {
	case strings.HasPrefix(mime, "image/") && hgOwnerReadPurpose(purpose):
		ttl = 30 * time.Second
	case (strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "audio/")) &&
		hgAdminAVDraftPurpose(purpose) && actor == owner && session == "ready" && !referenced:
		ttl = 5 * time.Minute
	default:
		return result, HGErrNotFound
	}
	link, err := s.previewRead.hgSignRead(ctx, sealedKey, ttl)
	if err != nil {
		return result, err
	}
	return hgAdminPreviewResult{URL: link, MIME: mime, ExpiresIn: int(ttl / time.Second)}, nil
}

// hgDeleteAdminAsset 只登记一项清理申请；复用现有 worker，不在事务中删除对象。
// 锁住会话及资源后再查引用，避免业务绑定与删除先后穿插。
func (s *HGService) hgDeleteAdminAsset(ctx context.Context, actor, asset string) (hgAdminDeleteResult, error) {
	var result hgAdminDeleteResult
	if !hgCanonicalAssetID(asset) || actor == "" || len(actor) > 64 {
		return result, HGErrInvalid
	}
	if s.sealed == nil {
		return result, errors.New("媒体后台清理未配置")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	var upload, sessionOwner, sessionPurpose, sessionKey, sessionStatus string
	var protected bool
	err = tx.QueryRowContext(ctx, SQL.HGMediaAdminDeleteSessionSQL, asset).
		Scan(&upload, &sessionOwner, &sessionPurpose, &sessionKey, &sessionStatus, &protected)
	if errors.Is(err, sql.ErrNoRows) {
		return result, HGErrNotFound
	}
	if err != nil {
		return result, err
	}
	var owner, purpose, key, bucket, status string
	var sealed sql.NullString
	err = tx.QueryRowContext(ctx, SQL.HGMediaAdminDeleteAssetSQL, asset).
		Scan(&owner, &purpose, &key, &bucket, &status, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return result, HGErrNotFound
	}
	if err != nil {
		return result, err
	}
	suffix := strings.ReplaceAll(asset, "-", "")
	sealedKey := "sealed/" + suffix
	if owner == "" || owner != sessionOwner || purpose != sessionPurpose || key != sessionKey ||
		bucket != s.bucket || key != "quarantine/"+suffix || sealed.Valid && sealed.String != sealedKey ||
		!hgOwnerReadPurpose(purpose) && !(hgAdminAVDraftPurpose(purpose) && owner == actor) {
		return result, HGErrNotFound
	}
	if status == sessionStatus && (status == "deleting" || status == "deleted") {
		return hgAdminDeleteResult{AssetID: asset, Status: status}, nil
	}
	if protected || status != sessionStatus || (status != "ready" && status != "failed") {
		return result, HGErrConflict
	}
	var found int
	err = tx.QueryRowContext(ctx, SQL.HGMediaHasRefSQL, asset).Scan(&found)
	if err == nil {
		return result, HGErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if err = hgAdminDeleteTransition(ctx, tx, asset, upload); err != nil {
		return result, err
	}
	for _, object := range [...]string{key, sealedKey} {
		if _, err = tx.ExecContext(ctx, SQL.HGMediaAdminDeleteQueueSQL, asset, bucket, object); err != nil {
			return result, err
		}
	}
	trace := Utils.From(ctx)
	if trace == "" || len(trace) > 64 {
		trace = Utils.GenerateTID()
	}
	if _, err = tx.ExecContext(ctx, SQL.HGInsertIAMAudit, actor, "media.asset.delete", "media_asset", asset, trace, `{"status":"deleting"}`); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return hgAdminDeleteResult{AssetID: asset, Status: "deleting"}, nil
}

func hgAdminDeleteTransition(ctx context.Context, tx *sql.Tx, asset, upload string) error {
	for _, change := range [...]struct {
		query string // 状态更新 SQL，由已锁定的状态和主键限制。
		id    string // 将要推进状态的资源或会话主键。
	}{{SQL.HGMediaExpireAssetSQL, asset}, {SQL.HGMediaExpireSessionSQL, upload}} {
		res, err := tx.ExecContext(ctx, change.query, change.id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return HGErrConflict
		}
	}
	return nil
}

func (h *HGHandler) hgAdminPreview(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.hgUser(w, r)
	if !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["asset_id"]) != 1 {
		hgFail(w, r, HGErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	result, err := h.service.hgPreviewAdminAsset(ctx, actor, query.Get("asset_id"))
	if err != nil {
		hgAdminActionFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	HGResponse.SuccessResult(w, r, result)
}

func (h *HGHandler) hgAdminDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.hgUser(w, r)
	if !ok {
		return
	}
	var req struct {
		AssetID string `json:"asset_id"` // 新媒体 UUID；不接收来源、对象键或他人身份。
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		hgFail(w, r, HGErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	result, err := h.service.hgDeleteAdminAsset(ctx, actor, req.AssetID)
	if err != nil {
		hgAdminActionFail(w, r, err)
		return
	}
	HGResponse.SuccessResult(w, r, result)
}

func hgAdminActionFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, HGErrNotFound):
		HGResponse.HGFailStatus(w, r, http.StatusNotFound, HGResponse.HGErrorResult{Code: HGResponse.InvalidParam.Code, Message: "资源不存在或不允许该操作"})
	case errors.Is(err, HGErrConflict):
		HGResponse.HGFailStatus(w, r, http.StatusConflict, HGResponse.HGErrorResult{Code: HGResponse.InvalidParam.Code, Message: "资源有业务引用、处于保护期或状态不允许删除"})
	default:
		hgFail(w, r, err)
	}
}
