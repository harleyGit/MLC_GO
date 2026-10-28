package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

// hgMFAKey 从进程密钥读取 32 字节 AES-GCM KEK；缺失时 MFA 和审批均默认拒绝。
func hgMFAKey() ([]byte, error) {
	key, err := hex.DecodeString(os.Getenv("MLC_IAM_MFA_KEY_HEX"))
	if err != nil || len(key) != 32 {
		return nil, ErrHGForbidden
	}
	return key, nil
}

func hgEncryptMFA(secret []byte) ([]byte, error) {
	key, err := hgMFAKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, secret, nil), nil
}

func hgDecryptMFA(ciphertext []byte) ([]byte, error) {
	key, err := hgMFAKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) <= aead.NonceSize() {
		return nil, ErrHGForbidden
	}
	return aead.Open(nil, ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():], nil)
}

func hgTOTP(secret []byte, counter int64) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(counter))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := int(sum[len(sum)-1] & 15)
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000)
}

func hgVerifyTOTP(secret []byte, code string, last int64, now time.Time) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	step := now.Unix() / 30
	for counter := step - 1; counter <= step+1; counter++ {
		if counter > last && subtle.ConstantTimeCompare([]byte(hgTOTP(secret, counter)), []byte(code)) == 1 {
			return counter, true
		}
	}
	return 0, false
}

func (s *HGService) hgLimitMFA(ctx context.Context, userID string) error {
	if s == nil || s.repo == nil || s.repo.cache == nil || s.repo.cache.Client() == nil {
		return ErrHGForbidden
	}
	key := PersistenceRedisPackage.GetIAMMFAAttemptKey(userID)
	if err := s.repo.cache.Client().SetNX(ctx, key, 0, 5*time.Minute).Err(); err != nil {
		return err
	}
	count, err := s.repo.cache.Client().Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if count > 5 {
		return ErrHGForbidden
	}
	return nil
}

// EnrollMFA 先复核登录密码，再返回一次性 TOTP 秘钥；调用方禁止持久化或日志记录返回值。
func (s *HGService) EnrollMFA(ctx context.Context, userID, password string) (string, string, error) {
	if err := s.hgCanManage(ctx, userID, true); err != nil {
		return "", "", err
	}
	if len(password) == 0 || len(password) > 256 {
		return "", "", ErrHGInvalid
	}
	if err := s.hgLimitMFA(ctx, userID); err != nil {
		return "", "", err
	}
	var hash, salt sql.NullString
	err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPassword, userID).Scan(&hash, &salt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrHGForbidden
	}
	if err != nil {
		return "", "", err
	}
	computed := UtilsPackage.HashPassword(password, salt.String)
	if !hash.Valid || !salt.Valid || subtle.ConstantTimeCompare([]byte(computed), []byte(hash.String)) != 1 {
		return "", "", ErrHGForbidden
	}
	secret := make([]byte, 20)
	if _, err = rand.Read(secret); err != nil {
		return "", "", err
	}
	sealed, err := hgEncryptMFA(secret)
	if err != nil {
		return "", "", err
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var enabled bool
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMMFAFactor, userID).Scan(new([]byte), &enabled, new(int64))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	if err == nil && enabled {
		return "", "", ErrHGConflict
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMMFAFactor, userID, sealed); err != nil {
		return "", "", err
	}
	if err = hgAudit(ctx, tx, userID, "iam.mfa.enroll", "user", userID, "mfa-enroll", map[string]any{}); err != nil {
		return "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	base := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	uri := "otpauth://totp/MLC:" + url.PathEscape(userID) + "?secret=" + base + "&issuer=MLC&algorithm=SHA1&digits=6&period=30"
	return base, uri, nil
}

// ConfirmMFA 消费首次验证码并启用因子；同一个时间步不能再次用于审批。
func (s *HGService) ConfirmMFA(ctx context.Context, userID, code string) error {
	if err := s.hgCanManage(ctx, userID, true); err != nil {
		return err
	}
	if err := s.hgLimitMFA(ctx, userID); err != nil {
		return err
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var ciphertext []byte
	var enabled bool
	var last int64
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMMFAFactor, userID).Scan(&ciphertext, &enabled, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	if enabled {
		return ErrHGConflict
	}
	secret, err := hgDecryptMFA(ciphertext)
	if err != nil {
		return err
	}
	counter, ok := hgVerifyTOTP(secret, code, last, time.Now())
	if !ok {
		return ErrHGForbidden
	}
	res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGActivateIAMMFAFactor, counter, userID, counter)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	if err = hgAudit(ctx, tx, userID, "iam.mfa.activate", "user", userID, "mfa-activate", map[string]any{"counter": strconv.FormatInt(counter, 10)}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *HGService) hgConsumeMFA(ctx context.Context, tx *sql.Tx, userID, code string) error {
	if err := s.hgLimitMFA(ctx, userID); err != nil {
		return err
	}
	var ciphertext []byte
	var enabled bool
	var last int64
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMMFAFactor, userID).Scan(&ciphertext, &enabled, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGForbidden
	}
	if err != nil {
		return err
	}
	if !enabled {
		return ErrHGForbidden
	}
	secret, err := hgDecryptMFA(ciphertext)
	if err != nil {
		return err
	}
	counter, ok := hgVerifyTOTP(secret, code, last, time.Now())
	if !ok {
		return ErrHGForbidden
	}
	res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGConsumeIAMMFACounter, counter, userID, counter)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	return nil
}
