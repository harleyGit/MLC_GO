package VideoUploadModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"database/sql"
	"reflect"
	"testing"
	"unsafe"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRegisterModulesDoesNotPanicWhenInitFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE INDEX").WillReturnError(sqlmock.ErrCancelled)

	HGHandlerPackage.ClearModules()
	t.Cleanup(HGHandlerPackage.ClearModules)

	redisService := &PersistenceRedisPackage.RedisService{}
	sqlManager := newTestSQLManager(db)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("RegisterModules() panic = %v", recovered)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
		modules := HGHandlerPackage.GetRegisteredModules()
		if len(modules) != 1 {
			t.Fatalf("registered modules count = %d, want 1", len(modules))
		}
		if modules[0].Name() != "video_upload" {
			t.Fatalf("registered module name = %q, want video_upload", modules[0].Name())
		}
	}()

	RegisterModules(redisService, sqlManager)
}

func newTestSQLManager(db *sql.DB) *PersistenceSQLPackage.HGSQLManager {
	manager := &PersistenceSQLPackage.HGSQLManager{}
	field := reflect.ValueOf(manager).Elem().FieldByName("db")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(db))
	return manager
}
