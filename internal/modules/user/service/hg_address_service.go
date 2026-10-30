package UserServicePackage

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	userrepository "MLC_GO/internal/modules/user/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
)

// ErrHGAddressInvalid 表示地址字段不符合请求协议。
var ErrHGAddressInvalid = errors.New("收货地址字段无效")

var hgPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
var hgRegionCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// HGAddressService 编排用户地址业务；不使用共享缓存，避免敏感信息跨用户泄漏。
type HGAddressService struct {
	repo *userrepository.HGAddressRepository // 注入的地址权威仓储。
}

// HGNewAddressService 使用模块注入的仓储创建地址服务。
func HGNewAddressService(repo *userrepository.HGAddressRepository) *HGAddressService {
	return &HGAddressService{repo: repo}
}

func hgValidAddress(input *UserDtoPackage.HGAddressInput) bool {
	input.ReceiverName = strings.TrimSpace(input.ReceiverName)
	input.ReceiverPhone = strings.TrimSpace(input.ReceiverPhone)
	input.Region = strings.TrimSpace(input.Region)
	input.ProvinceCode = strings.TrimSpace(input.ProvinceCode)
	input.CityCode = strings.TrimSpace(input.CityCode)
	input.DetailAddress = strings.TrimSpace(input.DetailAddress)
	for _, field := range []struct {
		value string
		max   int
	}{{input.ReceiverName, 64}, {input.Region, 128}, {input.DetailAddress, 512}} {
		if utf8.RuneCountInString(field.value) == 0 || utf8.RuneCountInString(field.value) > field.max {
			return false
		}
	}
	return hgPhonePattern.MatchString(input.ReceiverPhone) && hgRegionCodePattern.MatchString(input.ProvinceCode) && hgRegionCodePattern.MatchString(input.CityCode) && input.CityCode[:2] == input.ProvinceCode[:2] && input.ProvinceCode[2:] == "0000" && input.CityCode[2:] != "0000"
}

// HGCreateAddress 校验并新增地址，第一条自动成为默认地址。
func (s *HGAddressService) HGCreateAddress(ctx context.Context, userID string, input UserDtoPackage.HGAddressInput) (UserDtoPackage.HGAddress, error) {
	if !hgValidAddress(&input) {
		return UserDtoPackage.HGAddress{}, ErrHGAddressInvalid
	}
	id, err := s.repo.HGCreateAddress(ctx, userID, UtilsPackage.GenerateBusinessID("ADDR"), input)
	if err != nil {
		return UserDtoPackage.HGAddress{}, err
	}
	return s.HGGetAddress(ctx, userID, id)
}

// HGGetAddress 只返回当前用户指定地址。
func (s *HGAddressService) HGGetAddress(ctx context.Context, userID string, id int64) (UserDtoPackage.HGAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.repo.HGGetAddress(ctx, userID, id)
}

// HGListAddresses 返回当前用户全部地址，最多20条。
func (s *HGAddressService) HGListAddresses(ctx context.Context, userID string) ([]UserDtoPackage.HGAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.repo.HGListAddresses(ctx, userID)
}

// HGUpdateAddress 全量替换本人指定地址，保留默认状态。
func (s *HGAddressService) HGUpdateAddress(ctx context.Context, userID string, id int64, input UserDtoPackage.HGAddressInput) error {
	if !hgValidAddress(&input) {
		return ErrHGAddressInvalid
	}
	return s.repo.HGUpdateAddress(ctx, userID, id, input)
}

// HGDeleteAddress 删除本人地址，默认地址在仓储事务内重新选定。
func (s *HGAddressService) HGDeleteAddress(ctx context.Context, userID string, id int64) error {
	return s.repo.HGDeleteAddress(ctx, userID, id)
}

// HGSetDefaultAddress 切换本人默认地址。
func (s *HGAddressService) HGSetDefaultAddress(ctx context.Context, userID string, id int64) error {
	return s.repo.HGSetDefaultAddress(ctx, userID, id)
}
