package hgerrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestHGRegistryRejectsCollisionsAndWrongDomain(t *testing.T) {
	base := HGMallStockInsufficient.Definition()
	for _, mutate := range []func(*Definition){
		func(d *Definition) { d.Key = "MALL_OTHER" }, // 重复数字
		func(d *Definition) { d.Code++ },             // 重复 Key
		func(d *Definition) { d.Code = 1202001 },
		func(d *Definition) { d.Code = 1402000 },
		func(d *Definition) { d.Code = 10000000 },
		func(d *Definition) { d.Code = 1501004; d.Key = "PAYMENT_NEW" },
		func(d *Definition) { d.Code++; d.Key = "MALL_OTHER"; d.HTTPStatus = 202 },
		func(d *Definition) { d.Code++; d.Key = "MALL_OTHER"; d.Message = " " },
	} {
		other := base
		mutate(&other)
		if ValidateRegistry([]Definition{base, other}) == nil {
			t.Fatalf("接受了无效定义 %+v", other)
		}
	}
}

func TestHGErrorChain(t *testing.T) {
	cause := errors.New("内部诊断")
	err := fmt.Errorf("下单失败：%w", Wrap(HGMallStockInsufficient, cause))
	var app *AppError
	if !errors.Is(err, cause) || !errors.Is(err, HGMallStockInsufficient) || !errors.As(err, &app) || Resolve(err).Code != 1402004 {
		t.Fatal("错误链或分类丢失")
	}
	if Resolve(cause).Code != 1000001 || Resolve(New(Code(-1))).Code != 1000001 {
		t.Fatal("未知错误必须降级")
	}
	var typedNil *AppError
	if Resolve(typedNil).Code != 1000001 || !errors.Is(err, New(HGMallStockInsufficient)) {
		t.Fatal("空错误或同原因比较失败")
	}
}

func TestHGPublishedSubsetCannotChange(t *testing.T) {
	frozen := HGDefinitions()
	if err := HGValidateCompatibility(frozen[:1]); err != nil {
		t.Fatal(err)
	} // 旧版本可以不含后来新增的错误。
	frozen[0].Key = "COMMON_RECYCLED"
	if HGValidateCompatibility(frozen) == nil {
		t.Fatal("历史 Key 被改义却通过了门禁")
	}
	if HGValidateCompatibility([]Definition{{Code: 1000999, Key: "COMMON_REMOVED", Message: "曾发布", HTTPStatus: 500}}) == nil {
		t.Fatal("历史编号被删除却通过了门禁")
	}
}
