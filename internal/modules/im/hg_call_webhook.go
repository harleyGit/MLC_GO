package im

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// HGWebhookHandler 只挂管理面独立路由；使用 LiveKit 签名认证，不要求用户 JWT 或关闭业务 API Guard。
func (s *HGCalls) HGWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	event, err := s.livekit.HGVerifyWebhook(r.Header.Get("Authorization"), body)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if event.Event != "room_finished" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !strings.HasPrefix(event.Room.Name, "im_") || !hgIMASCII(event.ID) || event.Room.SID == "" || len(event.Room.SID) > 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err = s.hgRoomFinished(ctx, event.ID, strings.TrimPrefix(event.Room.Name, "im_"), event.Room.SID, body); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *HGCalls) hgRoomFinished(ctx context.Context, eventID, raw, sid string, body []byte) error {
	id, err := hgRandomID(raw)
	if err != nil {
		return err
	}
	tx, err := s.service.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hash := sha256.Sum256(body)
	if _, err = tx.ExecContext(ctx, SQL.HGIMCallWebhookInsertSQL, eventID, hash[:]); err != nil {
		return err
	}
	var stored []byte
	if err = tx.QueryRowContext(ctx, SQL.HGIMCallWebhookHashSQL, eventID).Scan(&stored); err != nil {
		return err
	}
	if !bytes.Equal(stored, hash[:]) {
		return HGErrConflict
	}
	call, err := hgScanCall(tx.QueryRowContext(ctx, SQL.HGIMCallLockSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	var currentSID string
	if err = tx.QueryRowContext(ctx, SQL.HGIMCallRoomSIDSQL, id).Scan(&currentSID); err != nil {
		return err
	}
	if call.State != "accepted" || currentSID != sid {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, SQL.HGIMCallEndSQL, "ended", id); err != nil {
		return err
	}
	users := []string{call.CallerID, call.CalleeID}
	sort.Strings(users)
	for _, user := range users {
		if _, err = tx.ExecContext(ctx, SQL.HGIMCallFreeSQL, user, id); err != nil {
			return err
		}
	}
	call.State = "ended"
	call.Version++
	if err = s.hgEvent(ctx, tx, call); err != nil {
		return err
	}
	return tx.Commit()
}
