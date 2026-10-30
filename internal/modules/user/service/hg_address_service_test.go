package UserServicePackage

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"

	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	userrepository "MLC_GO/internal/modules/user/repository"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"github.com/DATA-DOG/go-sqlmock"
)

type hgAddressIDMatcher struct{ value string }

func (m *hgAddressIDMatcher) Match(v driver.Value) bool {
	id, ok := v.(string)
	if ok && regexp.MustCompile(`^ADDR_[0-9A-HJKMNP-TV-Z]{26}$`).MatchString(id) {
		m.value = id
		return true
	}
	return false
}

func TestHGCreateAddressGeneratesBusinessID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	idMatcher := &hgAddressIDMatcher{}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressLockUserSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressCountSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressInsertSQL)).WithArgs(idMatcher, "owner", "张三", "13800138000", "北京", "110000", "110100", "测试路", true).WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressGetSQL)).WithArgs(int64(5), "owner").WillReturnRows(sqlmock.NewRows([]string{"id", "address_id", "receiver_name", "receiver_phone", "region", "province_code", "city_code", "detail_address", "is_default"}).AddRow(5, "ADDR_01JZ4M9T5P4P4CH7B4Y4QXAK8B", "张三", "13800138000", "北京", "110000", "110100", "测试路", true))
	svc := HGNewAddressService(userrepository.HGNewAddressRepository(db))
	a, err := svc.HGCreateAddress(context.Background(), "owner", UserDtoPackage.HGAddressInput{ReceiverName: "张三", ReceiverPhone: "13800138000", Region: "北京", ProvinceCode: "110000", CityCode: "110100", DetailAddress: "测试路"})
	if err != nil {
		t.Fatal(err)
	}
	if idMatcher.value == "" || a.AddressID != "ADDR_01JZ4M9T5P4P4CH7B4Y4QXAK8B" {
		t.Fatalf("业务 ID 未生成或未返回：插入ID=%q，响应ID=%q", idMatcher.value, a.AddressID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAddressValidation(t *testing.T) {
	valid := UserDtoPackage.HGAddressInput{ReceiverName: "张三", ReceiverPhone: "13800138000", Region: "北京市 朝阳区", ProvinceCode: "110000", CityCode: "110100", DetailAddress: "测试路1号"}
	for _, tc := range []struct {
		name string
		edit func(*UserDtoPackage.HGAddressInput)
		want bool
	}{
		{"有效", func(*UserDtoPackage.HGAddressInput) {}, true},
		{"省市不一致", func(a *UserDtoPackage.HGAddressInput) { a.CityCode = "310100" }, false},
		{"省级代码无效", func(a *UserDtoPackage.HGAddressInput) { a.ProvinceCode = "110001" }, false},
		{"电话无效", func(a *UserDtoPackage.HGAddressInput) { a.ReceiverPhone = "12345" }, false},
		{"详细地址超长", func(a *UserDtoPackage.HGAddressInput) { a.DetailAddress = strings.Repeat("中", 513) }, false},
		{"姓名为空", func(a *UserDtoPackage.HGAddressInput) { a.ReceiverName = "  " }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := valid
			tc.edit(&a)
			if got := hgValidAddress(&a); got != tc.want {
				t.Fatalf("校验结果=%v，期望=%v", got, tc.want)
			}
		})
	}
}
