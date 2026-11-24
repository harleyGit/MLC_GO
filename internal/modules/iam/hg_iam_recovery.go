package iam

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"
)

// HGRecoveryStatus 是本人 MFA 恢复状态协议；不返回完整邮箱或验证码。
type HGRecoveryStatus struct {
	MFAEnabled    bool       `json:"mfaEnabled"`    // true 表示当前因子已激活；尚未绑定时为 false。
	EmailVerified bool       `json:"emailVerified"` // true 表示邮箱码已通过绑定，非邮件发送成功。
	EmailMasked   string     `json:"emailMasked"`   // 仅供本人识别的邮箱掩码，未绑定时为空。
	AvailableAt   *time.Time `json:"availableAt"`   // 恢复能力生效时间；未绑定时为 null。
}

func hgRecoveryEmailValid(email string) bool {
	if email == "" || len(email) > 255 || strings.ContainsAny(email, "\r\n ,<>") {
		return false
	}
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

func hgRecoveryMasked(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0][:1] + "***@" + parts[1]
}

func (s *HGService) hgRecoveryDeps(user string) error {
	if s == nil || s.repo == nil || s.repo.db == nil || s.repo.cache == nil ||
		s.repo.cache.Client() == nil || !hgValidText(user, 64) {
		return ErrHGForbidden
	}
	return nil
}

// HGOwnRecoveryStatus 只查询本人 MFA 与已验证邮箱，按两个主键点查，不发送任何邮件。
func (s *HGService) HGOwnRecoveryStatus(ctx context.Context, user string) (HGRecoveryStatus, error) {
	var status HGRecoveryStatus
	if err := s.hgRecoveryDeps(user); err != nil {
		return status, err
	}
	var enabled bool
	err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMMFAEnabled, user).Scan(&enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return status, err
	}
	status.MFAEnabled = err == nil && enabled
	var email string
	var verified, available time.Time
	err = s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectRecoveryEmail, user).Scan(&email, &verified, &available)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.EmailVerified, status.EmailMasked, status.AvailableAt = true, hgRecoveryMasked(email), &available
	return status, nil
}

func (s *HGService) hgSendRecoveryCode(ctx context.Context, user, email, purpose string) error {
	if s.hgRecoverySender == nil {
		return errors.New("邮件服务暂不可用")
	}
	rdb := s.repo.cache.Client()
	limit := PersistenceRedisPackage.HGRecoveryCodeKey(user, "send-limit")
	allowed, err := rdb.SetNX(ctx, limit, "1", time.Minute).Result()
	if err != nil {
		return err
	}
	if !allowed {
		return ErrHGForbidden
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	key := PersistenceRedisPackage.HGRecoveryCodeKey(user, purpose+"|"+strings.ToLower(email))
	if err := rdb.Set(ctx, key, code, 5*time.Minute).Err(); err != nil {
		return err
	}
	if err := s.hgRecoverySender.SendCode(ctx, email, code); err != nil {
		_ = rdb.Eval(ctx, PersistenceRedisPackage.HGEmailDiscardCodeLua, []string{key}, code).Err()
		return errors.New("邮件发送失败")
	}
	return nil
}

// HGSendOwnRecoveryBindCode 为本人新邮箱发送验证码；只有已启用 MFA 的账号可以绑定。
func (s *HGService) HGSendOwnRecoveryBindCode(ctx context.Context, user, email string) error {
	if err := s.hgRecoveryDeps(user); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !hgRecoveryEmailValid(email) {
		return ErrHGInvalid
	}
	var enabled bool
	if err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMMFAEnabled, user).Scan(&enabled); err != nil || !enabled {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return ErrHGForbidden
	}
	return s.hgSendRecoveryCode(ctx, user, email, "bind")
}

// HGSendOwnRecoveryResetCode 仅向权威已绑定邮箱发送，不接受客户端指定收件人。
func (s *HGService) HGSendOwnRecoveryResetCode(ctx context.Context, user string) error {
	if err := s.hgRecoveryDeps(user); err != nil {
		return err
	}
	var email string
	var verified, available time.Time
	if err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectRecoveryEmail, user).Scan(&email, &verified, &available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if time.Now().Before(available) {
		return ErrHGForbidden
	}
	var enabled bool
	if err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMMFAEnabled, user).Scan(&enabled); err != nil || !enabled {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return ErrHGForbidden
	}
	return s.hgSendRecoveryCode(ctx, user, email, "reset")
}

func (s *HGService) hgConsumeRecoveryCode(ctx context.Context, user, email, purpose, code string) error {
	if len(code) != 6 {
		return ErrHGInvalid
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return ErrHGInvalid
		}
	}
	key := PersistenceRedisPackage.HGRecoveryCodeKey(user, purpose+"|"+strings.ToLower(email))
	n, err := s.repo.cache.Client().Eval(ctx, PersistenceRedisPackage.HGEmailVerifyCodeLua,
		[]string{key, PersistenceRedisPackage.HGRecoveryCodeKey(user, purpose+"-attempt")}, code).Int()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGForbidden
	}
	return nil
}

func hgRecoveryCooldown() time.Duration {
	if ConfigPackage.GetEnv() == ConfigPackage.EnvDebug {
		return time.Minute
	}
	return time.Hour
}

// HGBindOwnRecoveryEmail 使用邮件码与已启用的本人 TOTP 同事务写入已验证状态；更换邮箱重置冷却期。
func (s *HGService) HGBindOwnRecoveryEmail(ctx context.Context, user, email, emailCode, mfaCode string) error {
	if err := s.hgRecoveryDeps(user); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !hgRecoveryEmailValid(email) || len(mfaCode) != 6 {
		return ErrHGInvalid
	}
	if err := s.hgLimitMFA(ctx, user); err != nil {
		return err
	}
	if err := s.hgConsumeRecoveryCode(ctx, user, email, "bind", emailCode); err != nil {
		return err
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.hgConsumeMFATx(ctx, tx, user, mfaCode); err != nil {
		return err
	}
	available := time.Now().Add(hgRecoveryCooldown())
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGUpsertRecoveryEmail, user, email, available); err != nil {
		return err
	}
	if err := hgAudit(ctx, tx, user, "iam.mfa.recovery.bind", "user", user, "recovery-bind", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

// HGResetOwnMFA 用登录密码及预绑定邮箱的一次性邮件码失效旧 MFA；旧会话在 MySQL 撤销水位处失败关闭。
func (s *HGService) HGResetOwnMFA(ctx context.Context, user, password, emailCode string) error {
	if err := s.hgRecoveryDeps(user); err != nil {
		return err
	}
	if len(password) == 0 || len(password) > 256 {
		return ErrHGInvalid
	}
	if err := s.hgLimitMFA(ctx, user); err != nil {
		return err
	}
	var hash, salt sql.NullString
	if err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPassword, user).Scan(&hash, &salt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if !hash.Valid || !salt.Valid || subtle.ConstantTimeCompare([]byte(UtilsPackage.HashPassword(password, salt.String)), []byte(hash.String)) != 1 {
		return ErrHGForbidden
	}
	var expectedEmail string
	var verified, expectedAvailable time.Time
	if err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectRecoveryEmail, user).Scan(&expectedEmail, &verified, &expectedAvailable); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if time.Now().Before(expectedAvailable) {
		return ErrHGForbidden
	}
	if err := s.hgConsumeRecoveryCode(ctx, user, expectedEmail, "reset", emailCode); err != nil {
		return err
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var email string
	var available time.Time
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectRecoveryEmailForUpdate, user).Scan(&email, &available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if email != expectedEmail || time.Now().Before(available) {
		return ErrHGForbidden
	}
	if res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGResetRecoveryMFA, []byte{}, user); err != nil {
		return err
	} else if n, err := res.RowsAffected(); err != nil || n != 1 {
		return ErrHGConflict
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGRevokeRecoverySessions, user); err != nil {
		return err
	}
	if err := hgAudit(ctx, tx, user, "iam.mfa.recovery.reset", "user", user, "recovery-reset", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}
