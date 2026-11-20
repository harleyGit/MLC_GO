package product

import (
	IAMPackage "MLC_GO/internal/modules/iam"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 仅覆盖该用例调用的可信状态读取；其他 HGStore 操作若被意外触发应使测试失败。
type hgShopScopeStore struct {
	HGStore
	state HGProductAuthState // 权威商品状态替身，商家 ID 不来自浏览器。
}

func (s hgShopScopeStore) HGProductAuthState(context.Context, string) (HGProductAuthState, error) {
	return s.state, nil
}

func TestHGMallProductUpdateUsesApprovedShopScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := hgShopScopeStore{state: HGProductAuthState{
		OwnerID: "SELLER", MerchantID: "MER_1", MerchantStatus: 1,
		ProductStatus: 1, ReviewStatus: 1,
	}}
	service := HGNewService(store, nil).HGWithAuthorizer(IAMPackage.NewHGService(IAMPackage.NewHGRepository(db)))
	// 已审核商家身份已存在时，幂等 onboarding 不写额外角色或审计。
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMMerchantBinding)).
		WithArgs("MER_1", "SELLER").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPermission)).
		WithArgs("SELLER", "shop", "MER_1", "mall:product:update").
		WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	if err := service.hgAuthorizeOwnProductAction(context.Background(), "SELLER", "PRD_1", []string{"published"}); err != nil {
		t.Fatalf("本人已上架商品的 shop 授权应通过最终业务校验: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
