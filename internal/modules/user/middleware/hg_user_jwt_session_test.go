package UserJWTMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type hgSessionCheckHook struct{ active bool }

func (h *hgSessionCheckHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *hgSessionCheckHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *hgSessionCheckHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() != "exists" {
			return errors.New("未预期 Redis 命令")
		}
		if h.active {
			cmd.(*redis.IntCmd).SetVal(1)
		}
		return nil
	}
}

func TestAuthMiddlewareRejectsEvictedSession(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	UserServicePackage.HGConfigureSessionDB(db)
	t.Cleanup(func() { UserServicePackage.HGConfigureSessionDB(nil) })
	oldClient := PersistenceRedisPackage.RDB
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	hook := &hgSessionCheckHook{active: true}
	client.AddHook(hook)
	PersistenceRedisPackage.RDB = client
	t.Cleanup(func() { PersistenceRedisPackage.RDB = oldClient; client.Close() })
	claims := &UserServicePackage.HGClaims{
		UserID: "user-1", SessionID: "abcdefabcdefabcdefabcdefabcdefab", TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), Issuer: "mlc-go", Subject: "user-token"},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(UserServicePackage.Secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tc := range []struct {
		active bool
		status int
	}{{true, http.StatusNoContent}, {false, http.StatusUnauthorized}} {
		hook.active = tc.active
		if tc.active {
			mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
		}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/im/conversations", nil)
		request.Header.Set("Authorization", "Bearer "+signed)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("active=%v, status=%d, want=%d", tc.active, response.Code, tc.status)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
