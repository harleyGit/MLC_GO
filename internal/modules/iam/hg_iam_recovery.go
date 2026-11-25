package iam

import (
	UserService "MLC_GO/internal/modules/user/service"
	Config "MLC_GO/internal/pkg/config"
	Mail "MLC_GO/internal/pkg/email"
	"MLC_GO/internal/pkg/logHG"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	Redis "MLC_GO/internal/pkg/redis"
	Utils "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	hgRecoveryBind  = 1
	hgRecoveryReset = 2
)

var (
	hgErrRecoveryMFA      = errors.New("请先绑定并启用 MFA")
	hgErrRecoveryEmail    = errors.New("尚无已验证的恢复邮箱")
	hgErrRecoveryCooldown = errors.New("恢复邮箱尚在冷却期")
	hgErrRecoveryCode     = errors.New("邮件验证码错误、已过期或已使用")
	hgErrRecoveryFactor   = errors.New("MFA 动态码无效或已使用，请使用新的动态码")
	hgErrRecoveryPassword = errors.New("当前登录密码不正确")
	hgErrRecoveryTaken    = errors.New("此邮箱无法绑定，请使用其他邮箱")
	hgErrRecoveryChanged  = errors.New("安全设置已变化，请重新登录或重试")
	hgErrRecoveryDisabled = errors.New("MFA 找回暂未开放，请联系管理员")
)

// HGRecoveryStatus 是本人安全中心响应，仅包含邮箱掩码和 UTC 生效时间。
type HGRecoveryStatus struct {
	MFAEnabled    bool       `json:"mfaEnabled"`    // 是否启用了当前 MFA 因子。
	EmailVerified bool       `json:"emailVerified"` // 是否已有验证成功的恢复邮箱。
	EmailMasked   string     `json:"emailMasked"`   // 邮箱掩码，未绑定时为空。
	AvailableAt   *time.Time `json:"availableAt"`   // UTC 冷却期截止时间，未绑定时为 null。
	ResetEnabled  bool       `json:"resetEnabled"`  // 部署已确认所有实例校验代际时才启用找回。
}

type hgRecoveryEmail struct {
	address   string    // 已验证收件地址。
	available time.Time // UTC 生效时间，同时将找回码绑定到本次邮箱验证。
	ready     bool      // 由 MySQL 权威时间判断是否过了冷却期。
}

func hgReadRecoveryEmail(row *sql.Row) (hgRecoveryEmail, error) {
	var current hgRecoveryEmail
	err := row.Scan(&current.address, &current.available, &current.ready)
	return current, err
}

func (s *HGService) hgRecoveryMFAEnabled(ctx context.Context, user string) (bool, error) {
	var enabled bool
	err := s.repo.db.QueryRowContext(ctx, SQL.HGSelectIAMMFAEnabled, user).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return enabled, err
}

// HGOwnRecoveryStatus 只读取本人状态，不依赖发信服务或恢复限流。
func (s *HGService) HGOwnRecoveryStatus(ctx context.Context, user string) (HGRecoveryStatus, error) {
	status := HGRecoveryStatus{ResetEnabled: hgRecoveryResetEnabled()}
	if s == nil || s.repo == nil || s.repo.db == nil || user == "" {
		return status, ErrHGForbidden
	}
	var err error
	if status.MFAEnabled, err = s.hgRecoveryMFAEnabled(ctx, user); err != nil {
		return status, err
	}
	current, err := hgReadRecoveryEmail(s.repo.db.QueryRowContext(ctx, SQL.HGSelectRecoveryEmail, user))
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	_, domain, _ := strings.Cut(current.address, "@")
	available := current.available.UTC()
	status.EmailVerified, status.EmailMasked, status.AvailableAt = true, "***@"+domain, &available
	return status, nil
}

func (s *HGService) hgRecoveryContext(ctx context.Context, user string) (uint64, error) {
	generation, ok := UserService.HGSessionGenerationFromContext(ctx)
	if !ok || user == "" || s == nil || s.repo == nil || s.repo.db == nil {
		return 0, ErrHGForbidden
	}
	return generation, nil
}

// hgLockRecoveryAccount 与 IM 设备登记按同一 users 点锁串行；在 MFA 或恢复表之前获得。
func hgLockRecoveryAccount(ctx context.Context, tx *sql.Tx, user string, generation uint64) (hash, salt sql.NullString, err error) {
	if err = tx.QueryRowContext(ctx, SQL.HGLockRecoveryAccount, user).Scan(&hash, &salt); err != nil {
		return
	}
	var current uint64
	if err = tx.QueryRowContext(ctx, SQL.HGSelectRecoveryGeneration, user).Scan(&current); err == nil && current != generation {
		err = hgErrRecoveryChanged
	}
	return
}

// hgRecoveryDigest 的域隔离绑定账号、代际、用途和邮箱；数据库不保存可直接使用的短验证码。
func hgRecoveryDigest(user string, generation uint64, purpose int, email hgRecoveryEmail, code string) ([]byte, error) {
	key, err := hgMFAKey()
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = fmt.Fprintf(mac, "mlc-mfa-recovery-v1\x00%s\x00%d\x00%d\x00%s\x00%d\x00%s",
		user, generation, purpose, email.address, email.available.UnixMilli(), code)
	return mac.Sum(nil), nil
}

func (s *HGService) hgSendRecoveryCode(ctx context.Context, user string, generation uint64, email hgRecoveryEmail, purpose int) error {
	if s.hgRecoverySender == nil || s.repo.cache == nil || s.repo.cache.Client() == nil {
		return Mail.HGErrUnavailable
	}
	rdb := s.repo.cache.Client()
	ok, err := rdb.SetNX(ctx, Redis.HGRecoveryCodeKey(user, "send-limit"), "1", time.Minute).Result()
	if err != nil {
		return err
	}
	if !ok {
		return Mail.HGErrRateLimited
	}
	if err := Mail.HGAdmitCodeSend(ctx, rdb, email.address); err != nil {
		return err
	}
	code, err := Mail.HGNewCode()
	if err != nil {
		return err
	}
	digest, err := hgRecoveryDigest(user, generation, purpose, email, code)
	if err != nil {
		return err
	}
	if err := s.hgRecoverySender.SendCode(ctx, email.address, code); err != nil {
		return Mail.HGErrUnavailable
	}
	// 未获得 SMTP 接受确认时不使新码生效；任意外部 I/O 均在事务外。
	_, err = s.repo.db.ExecContext(ctx, SQL.HGSaveRecoveryChallenge, user, purpose, digest)
	return err
}

// HGSendOwnRecoveryBindCode 为本人新邮箱发送验证码；要求已启用 MFA。
func (s *HGService) HGSendOwnRecoveryBindCode(ctx context.Context, user, email string) error {
	generation, err := s.hgRecoveryContext(ctx, user)
	if err != nil {
		return err
	}
	address, err := Mail.HGNormalizeAddress(email)
	if err != nil {
		return ErrHGInvalid
	}
	enabled, err := s.hgRecoveryMFAEnabled(ctx, user)
	if err != nil {
		return err
	}
	if !enabled {
		return hgErrRecoveryMFA
	}
	return s.hgSendRecoveryCode(ctx, user, generation, hgRecoveryEmail{address: address}, hgRecoveryBind)
}

// HGSendOwnRecoveryResetCode 只发送到本人已验证且过了冷却期的邮箱。
func (s *HGService) HGSendOwnRecoveryResetCode(ctx context.Context, user string) error {
	generation, err := s.hgRecoveryContext(ctx, user)
	if err != nil {
		return err
	}
	if !hgRecoveryResetEnabled() {
		return hgErrRecoveryDisabled
	}
	enabled, err := s.hgRecoveryMFAEnabled(ctx, user)
	if err != nil {
		return err
	}
	if !enabled {
		return hgErrRecoveryMFA
	}
	email, err := hgReadRecoveryEmail(s.repo.db.QueryRowContext(ctx, SQL.HGSelectRecoveryEmail, user))
	if errors.Is(err, sql.ErrNoRows) {
		return hgErrRecoveryEmail
	}
	if err != nil {
		return err
	}
	if !email.ready {
		return hgErrRecoveryCooldown
	}
	return s.hgSendRecoveryCode(ctx, user, generation, email, hgRecoveryReset)
}

func hgConsumeRecoveryChallenge(ctx context.Context, tx *sql.Tx, user string, generation uint64, purpose int, email hgRecoveryEmail, code string) error {
	digest, err := hgRecoveryDigest(user, generation, purpose, email, code)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, SQL.HGConsumeRecoveryChallenge, user, purpose, digest)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return hgErrRecoveryCode
	}
	return err
}

func hgRecoveryCooldown() int {
	if Config.GetEnv() == Config.EnvDebug {
		return 60
	}
	return 3600
}

func hgRecoveryResetEnabled() bool {
	return Config.GetEnv() == Config.EnvDebug || os.Getenv("MLC_MFA_RECOVERY_RESET_ENABLED") == "1"
}

// HGBindOwnRecoveryEmail 在同一短事务消费邮件码、MFA 时间步并写入已验证邮箱。
func (s *HGService) HGBindOwnRecoveryEmail(ctx context.Context, user, email, emailCode, mfaCode string) error {
	generation, err := s.hgRecoveryContext(ctx, user)
	if err != nil {
		return err
	}
	address, err := Mail.HGNormalizeAddress(email)
	if err != nil || !Mail.HGValidCode(emailCode) || !Mail.HGValidCode(mfaCode) {
		return ErrHGInvalid
	}
	if err := s.hgLimitMFA(ctx, user); err != nil {
		return err
	}
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := hgLockRecoveryAccount(ctx, tx, user, generation); err != nil {
		return err
	}
	if err := hgConsumeRecoveryChallenge(ctx, tx, user, generation, hgRecoveryBind, hgRecoveryEmail{address: address}, emailCode); err != nil {
		return err
	}
	if err := s.hgConsumeMFATx(ctx, tx, user, mfaCode); err != nil {
		if errors.Is(err, ErrHGForbidden) || errors.Is(err, ErrHGConflict) {
			return hgErrRecoveryFactor
		}
		return err
	}
	current, err := hgReadRecoveryEmail(tx.QueryRowContext(ctx, SQL.HGSelectRecoveryEmailForUpdate, user))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, SQL.HGInsertRecoveryEmail, user, address, hgRecoveryCooldown())
	case err == nil && current.address != address:
		_, err = tx.ExecContext(ctx, SQL.HGUpdateRecoveryEmail, address, hgRecoveryCooldown(), user)
	}
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			return hgErrRecoveryTaken
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, SQL.HGClearRecoveryChallenges, user); err != nil {
		return err
	}
	if err := hgAudit(ctx, tx, user, "iam.mfa.recovery.bind", "user", user, "recovery-bind", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

// HGResetOwnMFA 复核当前密码并原子消费已验证邮箱码、删除旧因子、推进代际和撤销活跃 IM 设备。
func (s *HGService) HGResetOwnMFA(ctx context.Context, user, password, emailCode string) error {
	generation, err := s.hgRecoveryContext(ctx, user)
	if err != nil {
		return err
	}
	if !hgRecoveryResetEnabled() {
		return hgErrRecoveryDisabled
	}
	if len(password) == 0 || len(password) > 256 || !Mail.HGValidCode(emailCode) {
		return ErrHGInvalid
	}
	if err := s.hgLimitMFA(ctx, user); err != nil {
		return err
	}
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hash, salt, err := hgLockRecoveryAccount(ctx, tx, user, generation)
	if err != nil {
		return err
	}
	if !hash.Valid || !salt.Valid || subtle.ConstantTimeCompare([]byte(Utils.HashPassword(password, salt.String)), []byte(hash.String)) != 1 {
		return hgErrRecoveryPassword
	}
	email, err := hgReadRecoveryEmail(tx.QueryRowContext(ctx, SQL.HGSelectRecoveryEmailForUpdate, user))
	if errors.Is(err, sql.ErrNoRows) {
		return hgErrRecoveryEmail
	}
	if err != nil {
		return err
	}
	if !email.ready {
		return hgErrRecoveryCooldown
	}
	if err := hgConsumeRecoveryChallenge(ctx, tx, user, generation, hgRecoveryReset, email, emailCode); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, SQL.HGResetRecoveryMFA, user)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return hgErrRecoveryMFA
	}
	if _, err := tx.ExecContext(ctx, SQL.HGRevokeRecoverySessions, user); err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, SQL.HGRevokeRecoveryIMDevices, user)
	if err != nil {
		return err
	}
	n, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 5 {
		return fmt.Errorf("活跃 IM 设备超过预期上限")
	}
	if _, err := tx.ExecContext(ctx, SQL.HGClearRecoveryChallenges, user); err != nil {
		return err
	}
	if err := hgAudit(ctx, tx, user, "iam.mfa.recovery.reset", "user", user, "recovery-reset", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

type hgRecoverySocketBatch interface {
	HGDisconnectIAMSessions(context.Context, string, []string) error
}

// hgDisconnectRecoveredSessions 尽力关闭已建连的网关连接；权威撤销先提交，不依赖网关或 Redis 清理。
func (s *HGService) hgDisconnectRecoveredSessions(ctx context.Context, user, callerSID string) {
	gateway, ok := s.realtime.(hgRecoverySocketBatch)
	if !ok {
		return
	}
	generation, _ := UserService.HGSessionGenerationFromContext(ctx)
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 1200*time.Millisecond)
	defer cancel()
	sids := []string{callerSID} // 即使 Redis 索引不可读，也尽力关闭本次请求的连接。
	if s.repo.cache != nil && s.repo.cache.Client() != nil {
		rdb := s.repo.cache.Client()
		lookup, stop := context.WithTimeout(bounded, 300*time.Millisecond)
		indexed, err := rdb.ZRange(lookup, Redis.HGAuthSessionIndexKey+user, 0, 99).Result()
		keys := make([]string, 0, len(indexed))
		valid := make([]string, 0, len(indexed))
		for _, sid := range indexed {
			if decoded, decodeErr := hex.DecodeString(sid); decodeErr == nil && len(decoded) == 16 && sid != callerSID {
				valid = append(valid, sid)
				keys = append(keys, Redis.HGAuthSessionProofKey+user+":"+sid)
			}
		}
		if err == nil && len(keys) > 0 {
			values, err := rdb.MGet(lookup, keys...).Result()
			if err == nil {
				for i, value := range values {
					proof, _ := value.(string)
					current, parseErr := strconv.ParseUint(proof, 10, 64)
					if value == nil || (parseErr == nil && current <= generation) {
						sids = append(sids, valid[i])
					}
				}
			}
		}
		stop()
	}
	if err := gateway.HGDisconnectIAMSessions(bounded, user, sids); err != nil {
		logHG.ErrInfo("MFA 恢复后网关关闭未确认，旧会话已由权威代际撤销")
	}
}
