package service

import (
	hgcoin "MLC_GO/internal/modules/coin/service"
	"MLC_GO/internal/modules/wallet/dto"
	"MLC_GO/internal/modules/wallet/model"
	hgconfig "MLC_GO/internal/pkg/config"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type hgRepository interface {
	HGListCandidates(context.Context, uint64) ([]model.HGSKU, error)
	HGCreate(context.Context, string, string, string, time.Time) (model.HGOrder, error)
	HGDetail(context.Context, string, string) (model.HGOrder, error)
	HGPayDebug(context.Context, string, string) (model.HGOrder, error)
}

// HGService 编排钱包协议；debug入账由仓储与订单同事务完成。
type HGService struct {
	hgRepo hgRepository
	hgCoin hgBalanceReader
	hgNow  func() time.Time
}
type hgBalanceReader interface {
	Balance(context.Context, string) (uint64, error)
}

// HGNewService 组装用户钱包读写服务；coin 仅用于读取已有权威余额。
func HGNewService(hgRepo hgRepository, hgCoin hgBalanceReader) *HGService {
	return &HGService{hgRepo: hgRepo, hgCoin: hgCoin, hgNow: time.Now}
}

// HGBalance 返回已有 coin 钱包余额，不创建充值或变更资产。
func (hgService *HGService) HGBalance(hgCtx context.Context, hgUser string) (dto.HGBalance, error) {
	hgCtx, hgCancel := context.WithTimeout(hgCtx, 5*time.Second)
	defer hgCancel()
	if strings.TrimSpace(hgUser) == "" || hgService == nil || hgService.hgCoin == nil {
		return dto.HGBalance{}, model.HGErrInvalid
	}
	hgBalance, hgErr := hgService.hgCoin.Balance(hgCtx, hgUser)
	return dto.HGBalance{HGBalance: strconv.FormatUint(hgBalance, 10)}, hgErr
}

// HGListSKUs 只返回服务端确认启用且有效的档位，使用有界内部游标过滤。
func (hgService *HGService) HGListSKUs(hgCtx context.Context, hgCursorText, hgSizeText string) (dto.HGSKUPage, error) {
	hgCtx, hgCancel := context.WithTimeout(hgCtx, 5*time.Second)
	defer hgCancel()
	hgCursor, hgLimit, hgErr := hgPage(hgCursorText, hgSizeText)
	if hgErr != nil {
		return dto.HGSKUPage{}, hgErr
	}
	hgItems, hgErr := hgService.hgRepo.HGListCandidates(hgCtx, hgCursor)
	if hgErr != nil {
		return dto.HGSKUPage{}, hgErr
	}
	hgNow := hgService.hgNow().UTC()
	hgPageResult := dto.HGSKUPage{HGList: make([]dto.HGSKU, 0, hgLimit)}
	for hgIndex, hgItem := range hgItems {
		hgCursor = hgItem.HGID
		if !hgItem.HGStartTime.After(hgNow) && (hgItem.HGEndTime == nil || hgItem.HGEndTime.After(hgNow)) {
			hgPageResult.HGList = append(hgPageResult.HGList, hgSKUResponse(hgItem))
			if len(hgPageResult.HGList) == hgLimit {
				hgPageResult.HGHasMore = hgIndex+1 < len(hgItems) || len(hgItems) == 201
				break
			}
		}
	}
	if len(hgItems) == 201 && len(hgPageResult.HGList) < hgLimit {
		hgPageResult.HGHasMore = true
	}
	if hgPageResult.HGHasMore {
		hgPageResult.HGNextCursor = strconv.FormatUint(hgCursor, 10)
	}
	return hgPageResult, nil
}

func hgSKUResponse(hgItem model.HGSKU) dto.HGSKU {
	hgAvailable := []string{}
	hgMode := "unavailable"
	if hgconfig.IsWalletDebugPaymentEnabled() {
		hgMode = "platform_debug"
	}
	if hgconfig.IsWalletDebugPaymentEnabled() && hgItem.HGTotalCoin > 0 && hgItem.HGTotalCoin <= hgcoin.HGMaxMutationAmount {
		hgAvailable = append(hgAvailable, "platform_debug")
	}
	return dto.HGSKU{HGSKUID: hgItem.HGSKUID, HGTitle: hgItem.HGTitle, HGDescription: "购买后到账平台币", HGCurrency: hgItem.HGCurrency, HGPayAmount: hgItem.HGPayAmount, HGCoinAmount: hgItem.HGCoinAmount, HGBonusCoin: hgItem.HGBonusCoin, HGTotalCoin: hgItem.HGTotalCoin, HGSupported: hgItem.HGTotalCoin > 0 && hgItem.HGTotalCoin <= hgcoin.HGMaxMutationAmount, HGPaymentAvailable: len(hgAvailable) > 0, HGPaymentMode: hgMode, HGAvailableMethods: hgAvailable}
}

// HGCreateOrder 校验客户端幂等键，快照完全由仓储从服务端目录和用户表取得。
func (hgService *HGService) HGCreateOrder(hgCtx context.Context, hgUser string, hgRequest dto.HGCreateRequest) (dto.HGOrder, error) {
	hgCtx, hgCancel := context.WithTimeout(hgCtx, 5*time.Second)
	defer hgCancel()
	if hgUser == "" || !hgIdentifier.MatchString(hgRequest.HGSKUID) || !hgIdentifier.MatchString(hgRequest.HGRequestID) {
		return dto.HGOrder{}, model.HGErrInvalid
	}
	hgOrder, hgErr := hgService.hgRepo.HGCreate(hgCtx, hgUser, hgRequest.HGSKUID, hgRequest.HGRequestID, hgService.hgNow().UTC().Truncate(time.Millisecond))
	return hgOrderResponse(hgOrder, hgService.hgNow().UTC()), hgErr
}

// HGOrderDetail 只查询本人快照，等于expiresAt时即为expired。
func (hgService *HGService) HGOrderDetail(hgCtx context.Context, hgUser, hgOrderID string) (dto.HGOrder, error) {
	hgCtx, hgCancel := context.WithTimeout(hgCtx, 5*time.Second)
	defer hgCancel()
	if hgUser == "" || !hgIdentifier.MatchString(hgOrderID) {
		return dto.HGOrder{}, model.HGErrNotFound
	}
	hgOrder, hgErr := hgService.hgRepo.HGDetail(hgCtx, hgUser, hgOrderID)
	return hgOrderResponse(hgOrder, hgService.hgNow().UTC()), hgErr
}

// HGPay 严格校验渠道；微信和支付宝即使请求合法也明确拒绝，不退化为模拟支付。
func (hgService *HGService) HGPay(hgCtx context.Context, hgUser, hgOrderID, hgPaymentMethod string) (dto.HGOrder, error) {
	hgCtx, hgCancel := context.WithTimeout(hgCtx, 5*time.Second)
	defer hgCancel()
	if !hgIdentifier.MatchString(hgOrderID) || (hgPaymentMethod != "platform_debug" && hgPaymentMethod != "wechat" && hgPaymentMethod != "alipay") {
		return dto.HGOrder{}, model.HGErrInvalid
	}
	if hgUser == "" {
		return dto.HGOrder{}, model.HGErrNotFound
	}
	if hgPaymentMethod != "platform_debug" {
		if _, hgErr := hgService.HGOrderDetail(hgCtx, hgUser, hgOrderID); hgErr != nil {
			return dto.HGOrder{}, hgErr
		}
		return dto.HGOrder{}, model.HGErrPaymentUnavailable
	}
	hgOrder, hgErr := hgService.hgRepo.HGPayDebug(hgCtx, hgUser, hgOrderID)
	return hgOrderResponse(hgOrder, hgService.hgNow().UTC()), hgErr
}

func hgOrderResponse(hgOrder model.HGOrder, hgNow time.Time) dto.HGOrder {
	hgStatus := "pending"
	if !hgOrder.HGExpiresAt.After(hgNow) {
		hgStatus = "expired"
	}
	if hgOrder.HGStatus == "paid" {
		hgStatus = "paid"
	}
	hgAvailable := []string{}
	if hgStatus == "pending" && hgOrder.HGPaymentMode == "platform_debug" && hgconfig.IsWalletDebugPaymentEnabled() {
		hgAvailable = append(hgAvailable, "platform_debug")
	}
	hgResponse := dto.HGOrder{HGOrderID: hgOrder.HGOrderID, HGDisplayName: hgOrder.HGDisplayName, HGTitle: hgOrder.HGTitle, HGDescription: hgOrder.HGDescription, HGPayAmount: hgOrder.HGPayAmount, HGTotalCoin: hgOrder.HGTotalCoin, HGStatus: hgStatus, HGExpiresAt: hgOrder.HGExpiresAt.UTC().Format(time.RFC3339Nano), HGServerTime: hgNow.UTC().Format(time.RFC3339Nano), HGPaymentAvailable: hgOrder.HGPaymentMode == "platform_debug" && hgconfig.IsWalletDebugPaymentEnabled(), HGPaymentMode: hgOrder.HGPaymentMode, HGAvailableMethods: hgAvailable, HGCurrency: hgOrder.HGCurrency}
	if hgOrder.HGPaidAt != nil {
		hgResponse.HGPaidAt = hgOrder.HGPaidAt.UTC().Format(time.RFC3339Nano)
	}
	hgResponse.HGPaymentAvailable = len(hgAvailable) > 0
	if hgOrder.HGStatus == "paid" {
		hgResponse.HGBalanceAfter = strconv.FormatUint(hgOrder.HGBalanceAfter, 10)
	}
	return hgResponse
}

func hgPage(hgCursorText, hgSizeText string) (uint64, int, error) {
	hgParse := func(hgText string) (uint64, error) {
		if hgText == "" {
			return 0, nil
		}
		for _, hgRune := range hgText {
			if hgRune < '0' || hgRune > '9' {
				return 0, model.HGErrInvalid
			}
		}
		return strconv.ParseUint(hgText, 10, 64)
	}
	hgCursor, hgErr := hgParse(hgCursorText)
	if hgErr != nil {
		return 0, 0, model.HGErrInvalid
	}
	hgSize := uint64(20)
	if hgSizeText != "" {
		hgSize, hgErr = hgParse(hgSizeText)
		if hgErr != nil || hgSize == 0 {
			return 0, 0, model.HGErrInvalid
		}
	}
	if hgSize > 100 {
		hgSize = 100
	}
	return hgCursor, int(hgSize), nil
}

var hgIdentifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
