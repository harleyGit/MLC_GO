package main

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBuildMySQLMigrationURLUsesEscapedConfig(t *testing.T) {
	cfg := ConfigPackage.HGMySQLConfig{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "root",
		Password: "p@ss:word",
		Database: "HG_MLC_DB",
	}

	got := buildMySQLMigrationURL(cfg)
	want := "mysql://root:p%40ss%3Aword@tcp(127.0.0.1:3306)/HG_MLC_DB"
	if got != want {
		t.Fatalf("migration URL mismatch: got %q want %q", got, want)
	}
}

func TestHGCheckNotificationSendSchema(t *testing.T) {
	for _, test := range []struct {
		name      string
		indexes   int
		wantError bool
	}{
		{name: "全部就绪", indexes: 3},
		{name: "索引缺失", indexes: 2, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationSendSchemaColumnsSQL)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationSendSchemaIndexesSQL)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(test.indexes))
			if !test.wantError {
				mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationSendSchemaAuditTablesSQL)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationSendSchemaPermissionSQL)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			}
			err = hgCheckNotificationSendSchema(context.Background(), db)
			if (err != nil) != test.wantError {
				t.Fatalf("结构检查结果异常: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGCheckNotificationSendSchemaReadFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationSendSchemaColumnsSQL)).
		WillReturnError(errors.New("连接异常"))
	if err := hgCheckNotificationSendSchema(context.Background(), db); err == nil || err.Error() != "通知发送结构检查不可用" {
		t.Fatalf("不应泄露底层连接信息: %v", err)
	}
}

func TestHGCheckNotificationKafkaSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationKafkaSchemaColumnsSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationKafkaSchemaIndexesSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	if err := hgCheckNotificationKafkaSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCheckNotificationKafkaQueriesNeverWrites(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	for _, query := range []string{
		SQLQueriesPackage.HGNotificationKafkaReadySQL,
		SQLQueriesPackage.HGNotificationKafkaQueuedExpiredSQL,
		SQLQueriesPackage.HGNotificationKafkaLeasedExpiredSQL,
	} {
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(sql.ErrNoRows)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGNotificationKafkaClaimSQL)).
		WithArgs(0, 0, 0).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	if err := hgCheckNotificationKafkaQueries(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
