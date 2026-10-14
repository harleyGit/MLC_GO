package service

import (
	"MLC_GO/internal/modules/recharge/dto"
	"MLC_GO/internal/modules/recharge/model"
	"context"
	"encoding/base64"
	"encoding/binary"
	"strconv"
	"time"
)

type hgRepository interface {
	HGList(context.Context, string, *model.HGCursor, int) ([]model.HGRecord, error)
}

// HGService 处理充值记录分页与展示状态，不写订单或资产。
type HGService struct {
	hgRepo hgRepository     // 只读记录仓储。
	hgNow  func() time.Time // 请求级状态判断时钟。
}

// HGNewService 注入只读仓储。
func HGNewService(hgRepo hgRepository) *HGService { return &HGService{hgRepo: hgRepo, hgNow: time.Now} }

// HGRecords 返回本人记录。并发新增不会挤动续页边界，但不承诺跨请求快照。
func (hgService *HGService) HGRecords(hgCtx context.Context, hgUser, hgCursorText, hgSizeText string) (dto.HGPage, error) {
	hgCursor, hgLimit, hgErr := hgParsePage(hgCursorText, hgSizeText)
	if hgErr != nil || hgUser == "" {
		return dto.HGPage{}, model.HGErrInvalid
	}
	hgCtx, hgCancel := context.WithTimeout(hgCtx, 5*time.Second)
	defer hgCancel()
	hgItems, hgErr := hgService.hgRepo.HGList(hgCtx, hgUser, hgCursor, hgLimit)
	if hgErr != nil {
		return dto.HGPage{}, hgErr
	}
	hgPage := dto.HGPage{HGList: make([]dto.HGRecord, 0, hgLimit), HGHasMore: len(hgItems) > hgLimit}
	hgNow := hgService.hgNow()
	for hgIndex, hgItem := range hgItems {
		// 包括探测行在内，未知状态或无效金额一律关闭响应，不伪装成待支付。
		if hgItem.HGStatus < 0 || hgItem.HGStatus > 6 || hgItem.HGPayAmount < 0 || hgItem.HGTotalCoin < 0 {
			return dto.HGPage{}, model.HGErrInvalidRecord
		}
		if hgIndex >= hgLimit {
			continue
		}
		hgStatus := "pending"
		switch hgItem.HGStatus {
		case 0, 1:
			if !hgItem.HGExpiresAt.After(hgNow) {
				hgStatus = "expired"
			}
		case 2:
			hgStatus = "processing"
		case 3:
			hgStatus = "paid"
		case 4:
			hgStatus = "cancelled"
		case 5:
			hgStatus = "refunding"
		case 6:
			hgStatus = "refunded"
		}
		hgRecord := dto.HGRecord{HGOrderID: hgItem.HGOrderID, HGTitle: hgItem.HGTitle, HGCurrency: hgItem.HGCurrency, HGPayAmount: strconv.FormatInt(hgItem.HGPayAmount, 10), HGTotalCoin: strconv.FormatInt(hgItem.HGTotalCoin, 10), HGStatus: hgStatus, HGPaymentMode: hgItem.HGPaymentMode, HGCreatedAt: hgItem.HGCreatedAt.UTC().Format(time.RFC3339Nano)}
		if hgItem.HGPaidAt != nil {
			hgRecord.HGPaidAt = hgItem.HGPaidAt.UTC().Format(time.RFC3339Nano)
		}
		hgPage.HGList = append(hgPage.HGList, hgRecord)
	}
	if hgPage.HGHasMore {
		hgLast := hgItems[hgLimit-1]
		var hgBytes [16]byte
		binary.BigEndian.PutUint64(hgBytes[:8], uint64(hgLast.HGCreatedAt.UnixMilli()))
		binary.BigEndian.PutUint64(hgBytes[8:], hgLast.HGID)
		hgPage.HGNextCursor = base64.RawURLEncoding.EncodeToString(hgBytes[:])
	}
	return hgPage, nil
}

func hgParsePage(hgCursorText, hgSizeText string) (*model.HGCursor, int, error) {
	hgLimit := 50
	if hgSizeText != "" {
		if len(hgSizeText) > 2 {
			return nil, 0, model.HGErrInvalid
		}
		for _, hgChar := range hgSizeText {
			if hgChar < '0' || hgChar > '9' {
				return nil, 0, model.HGErrInvalid
			}
		}
		hgValue, hgErr := strconv.Atoi(hgSizeText)
		if hgErr != nil || hgValue < 1 || hgValue > 50 || strconv.Itoa(hgValue) != hgSizeText {
			return nil, 0, model.HGErrInvalid
		}
		hgLimit = hgValue
	}
	if hgCursorText == "" || hgCursorText == "0" {
		return nil, hgLimit, nil
	}
	if len(hgCursorText) != 22 {
		return nil, 0, model.HGErrInvalid
	}
	hgBytes, hgErr := base64.RawURLEncoding.Strict().DecodeString(hgCursorText)
	if hgErr != nil || len(hgBytes) != 16 || base64.RawURLEncoding.EncodeToString(hgBytes) != hgCursorText {
		return nil, 0, model.HGErrInvalid
	}
	hgMillis := int64(binary.BigEndian.Uint64(hgBytes[:8]))
	hgID := binary.BigEndian.Uint64(hgBytes[8:])
	hgTime := time.UnixMilli(hgMillis).UTC()
	if hgID == 0 || hgTime.Year() < 1000 || hgTime.Year() > 9999 {
		return nil, 0, model.HGErrInvalid
	}
	return &model.HGCursor{HGCreatedAt: hgTime, HGID: hgID}, hgLimit, nil
}
