package OpsServicePackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeBilibiliTagName(t *testing.T) {
	got, err := normalizeBilibiliTagName("  MMD·3D  ")
	if err != nil {
		t.Fatalf("normalizeBilibiliTagName() error = %v", err)
	}
	if got != "MMD·3D" {
		t.Fatalf("normalizeBilibiliTagName() = %q, want %q", got, "MMD·3D")
	}
}

func TestNormalizeBilibiliTagNameRejectsReservedRecommendation(t *testing.T) {
	if _, err := normalizeBilibiliTagName("推荐"); err == nil {
		t.Fatal("normalizeBilibiliTagName() error = nil, want reserved-name error")
	}
}

func TestNormalizeBilibiliTagStatus(t *testing.T) {
	if got, err := normalizeBilibiliTagStatus(0); err != nil || got != 1 {
		t.Fatalf("normalizeBilibiliTagStatus(0) = (%d, %v), want (1, nil)", got, err)
	}
	if _, err := normalizeBilibiliTagStatus(3); err == nil {
		t.Fatal("normalizeBilibiliTagStatus(3) error = nil, want validation error")
	}
}

func TestNormalizeBilibiliTagID(t *testing.T) {
	got, err := normalizeBilibiliTagID("  BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P  ")
	if err != nil {
		t.Fatalf("normalizeBilibiliTagID() error = %v", err)
	}
	if got != "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P" {
		t.Fatalf("normalizeBilibiliTagID() = %q", got)
	}
	if _, err := normalizeBilibiliTagID("101"); err == nil {
		t.Fatal("normalizeBilibiliTagID(101) error = nil, want invalid tagId error")
	}
}

func TestBilibiliTagMutationsRequireActiveAdmin(t *testing.T) {
	tests := []struct {
		name string
		call func(*Service) error
	}{
		{
			name: "create",
			call: func(service *Service) error {
				_, err := service.CreateBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", OpsDtoPackage.BilibiliTagRequest{Name: "MMD·3D", SortOrder: 20, Status: 1})
				return err
			},
		},
		{
			name: "update",
			call: func(service *Service) error {
				_, err := service.UpdateBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", OpsDtoPackage.UpdateBilibiliTagRequest{TagID: "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P", Name: "MMD·3D", SortOrder: 20, Status: 1})
				return err
			},
		},
		{
			name: "delete",
			call: func(service *Service) error {
				return service.DeleteBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", OpsDtoPackage.DeleteBilibiliTagRequest{TagID: "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New() error = %v", err)
			}
			defer db.Close()

			operatorID := "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P"
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsActiveAdminInternalIDByUserIDSQL)).
				WithArgs(operatorID).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))

			service := NewService(OpsRepositoryPackage.NewRepository(db), nil, nil)
			err = tt.call(service)
			if !errors.Is(err, ErrHGOperationsForbidden) {
				t.Fatalf("mutation error = %v, want ErrHGOperationsForbidden", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("sql expectations: %v", err)
			}
		})
	}
}

func TestBilibiliTagMutationsAllowActiveAdmin(t *testing.T) {
	tests := []struct {
		name   string
		expect func(sqlmock.Sqlmock, string)
		call   func(*Service) error
	}{
		{
			name: "create",
			expect: func(mock sqlmock.Sqlmock, operatorID string) {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOpsBilibiliTagSQL)).
					WithArgs(sqlmock.AnyArg(), "MMD·3D", 20, 1, operatorID, operatorID).
					WillReturnResult(sqlmock.NewResult(101, 1))
				mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsBilibiliTagByTagIDSQL)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"tag_id", "id", "name", "sort_order", "status", "created_at", "updated_at"}).
						AddRow("BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P", int64(101), "MMD·3D", 20, 1, time.Now(), time.Now()))
			},
			call: func(service *Service) error {
				_, err := service.CreateBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", OpsDtoPackage.BilibiliTagRequest{Name: "MMD·3D", SortOrder: 20, Status: 1})
				return err
			},
		},
		{
			name: "update",
			expect: func(mock sqlmock.Sqlmock, operatorID string) {
				tagID := "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P"
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateOpsBilibiliTagSQL)).
					WithArgs("MMD·3D", 20, 1, operatorID, tagID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsBilibiliTagByTagIDSQL)).
					WithArgs(tagID).
					WillReturnRows(sqlmock.NewRows([]string{"tag_id", "id", "name", "sort_order", "status", "created_at", "updated_at"}).
						AddRow(tagID, int64(101), "MMD·3D", 20, 1, time.Now(), time.Now()))
			},
			call: func(service *Service) error {
				_, err := service.UpdateBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", OpsDtoPackage.UpdateBilibiliTagRequest{TagID: "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P", Name: "MMD·3D", SortOrder: 20, Status: 1})
				return err
			},
		},
		{
			name: "delete",
			expect: func(mock sqlmock.Sqlmock, operatorID string) {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteOpsBilibiliTagSQL)).
					WithArgs(operatorID, "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			call: func(service *Service) error {
				return service.DeleteBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", OpsDtoPackage.DeleteBilibiliTagRequest{TagID: "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New() error = %v", err)
			}
			defer db.Close()

			operatorID := "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P"
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsActiveAdminInternalIDByUserIDSQL)).
				WithArgs(operatorID).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
			tt.expect(mock, operatorID)

			service := NewService(OpsRepositoryPackage.NewRepository(db), nil, nil)
			if err := tt.call(service); err != nil {
				t.Fatalf("mutation error = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("sql expectations: %v", err)
			}
		})
	}
}
