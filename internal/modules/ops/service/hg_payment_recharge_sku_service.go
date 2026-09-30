package OpsServicePackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsModelPackage "MLC_GO/internal/modules/ops/model"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrHGRechargeSKUInvalid = errors.New("invalid recharge SKU parameters")
var hgRechargeSKUCode = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const hgRechargeSKUMaxSafe = uint64(9007199254740991)

func (s *Service) hgAuthorizeRechargeSKU(ctx context.Context, operator, permission string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(operator) == "" {
		return ErrHGOperationsForbidden
	}
	allowed, err := s.repo.HasAssetPermission(ctx, operator, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrHGOperationsForbidden
	}
	return nil
}

func hgValidateRechargeSKU(req OpsDtoPackage.HGPaymentRechargeSKUCreateRequest, defaultStart time.Time) (OpsModelPackage.HGPaymentRechargeSKU, error) {
	item := OpsModelPackage.HGPaymentRechargeSKU{}
	if !hgRechargeSKUCode.MatchString(req.SKUCode) || !utf8.ValidString(req.Title) || strings.TrimSpace(req.Title) == "" || utf8.RuneCountInString(req.Title) > 128 || req.PayAmount == nil || req.CoinAmount == nil || req.BonusCoin == nil || req.Status == nil || req.SortOrder == nil {
		return item, ErrHGRechargeSKUInvalid
	}
	if *req.PayAmount == 0 || *req.PayAmount > hgRechargeSKUMaxSafe || *req.CoinAmount == 0 || *req.CoinAmount > hgRechargeSKUMaxSafe || *req.BonusCoin > hgRechargeSKUMaxSafe-*req.CoinAmount || (*req.Status != 0 && *req.Status != 1) || *req.SortOrder > math.MaxInt32 {
		return item, ErrHGRechargeSKUInvalid
	}
	start := defaultStart
	if req.StartTime != "" {
		var err error
		start, err = time.Parse(time.RFC3339Nano, req.StartTime)
		if err != nil {
			return item, ErrHGRechargeSKUInvalid
		}
	}
	start = start.UTC().Truncate(time.Millisecond)
	if start.Year() < 1000 || start.Year() > 9999 {
		return item, ErrHGRechargeSKUInvalid
	}
	var end *time.Time
	if req.EndTime != nil && *req.EndTime != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *req.EndTime)
		parsed = parsed.UTC().Truncate(time.Millisecond)
		if err != nil || parsed.Year() < 1000 || parsed.Year() > 9999 || !parsed.After(start) {
			return item, ErrHGRechargeSKUInvalid
		}
		end = &parsed
	}
	return OpsModelPackage.HGPaymentRechargeSKU{SKUCode: req.SKUCode, Title: req.Title, Currency: "CNY", PayAmount: *req.PayAmount, CoinAmount: *req.CoinAmount, BonusCoin: *req.BonusCoin, TotalCoin: *req.CoinAmount + *req.BonusCoin, Status: *req.Status, SortOrder: *req.SortOrder, StartTime: start, EndTime: end, Version: 1}, nil
}

// HGCreateRechargeSKU creates only a catalog definition, never a payment or asset mutation.
func (s *Service) HGCreateRechargeSKU(ctx context.Context, operator string, req OpsDtoPackage.HGPaymentRechargeSKUCreateRequest) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, OpsRepositoryPackage.HGRechargeSKUTimeout)
	defer cancel()
	if err := s.hgAuthorizeRechargeSKU(ctx, operator, "payment.recharge_sku.write"); err != nil {
		return "", err
	}
	item, err := hgValidateRechargeSKU(req, time.Now())
	if err != nil {
		return "", err
	}
	item.SKUID = UtilsPackage.GenerateBusinessID("RSKU")
	return item.SKUID, s.repo.HGCreateRechargeSKU(ctx, operator, item)
}

// HGUpdateRechargeSKU replaces editable fields; omitted startTime preserves the existing start.
func (s *Service) HGUpdateRechargeSKU(ctx context.Context, operator string, req OpsDtoPackage.HGPaymentRechargeSKUUpdateRequest) error {
	ctx, cancel := context.WithTimeout(ctx, OpsRepositoryPackage.HGRechargeSKUTimeout)
	defer cancel()
	if err := s.hgAuthorizeRechargeSKU(ctx, operator, "payment.recharge_sku.write"); err != nil {
		return err
	}
	if !hgRechargeSKUCode.MatchString(req.SKUID) || req.Version == nil || *req.Version == 0 || *req.Version == math.MaxUint32 {
		return ErrHGRechargeSKUInvalid
	}
	existing, err := s.repo.HGGetRechargeSKU(ctx, req.SKUID)
	if err != nil {
		return err
	}
	if existing.Version != *req.Version {
		return OpsRepositoryPackage.ErrHGRechargeSKUConflict
	}
	item, err := hgValidateRechargeSKU(OpsDtoPackage.HGPaymentRechargeSKUCreateRequest{SKUCode: req.SKUCode, Title: req.Title, PayAmount: req.PayAmount, CoinAmount: req.CoinAmount, BonusCoin: req.BonusCoin, Status: req.Status, SortOrder: req.SortOrder, StartTime: req.StartTime, EndTime: req.EndTime}, existing.StartTime)
	if err != nil {
		return err
	}
	item.SKUID, item.Version = req.SKUID, *req.Version
	return s.repo.HGUpdateRechargeSKU(ctx, operator, item)
}

// HGDeleteRechargeSKU reserves the code permanently and rejects stale versions.
func (s *Service) HGDeleteRechargeSKU(ctx context.Context, operator string, req OpsDtoPackage.HGPaymentRechargeSKUDeleteRequest) error {
	ctx, cancel := context.WithTimeout(ctx, OpsRepositoryPackage.HGRechargeSKUTimeout)
	defer cancel()
	if err := s.hgAuthorizeRechargeSKU(ctx, operator, "payment.recharge_sku.write"); err != nil {
		return err
	}
	if !hgRechargeSKUCode.MatchString(req.SKUID) || req.Version == nil || *req.Version == 0 || *req.Version == math.MaxUint32 {
		return ErrHGRechargeSKUInvalid
	}
	return s.repo.HGDeleteRechargeSKU(ctx, operator, req.SKUID, *req.Version)
}

// HGListRechargeSKUs returns a bounded management page, including disabled SKUs but excluding deleted rows.
func (s *Service) HGListRechargeSKUs(ctx context.Context, operator, cursorText, sizeText string) (*OpsDtoPackage.HGPaymentRechargeSKUListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, OpsRepositoryPackage.HGRechargeSKUTimeout)
	defer cancel()
	if err := s.hgAuthorizeRechargeSKU(ctx, operator, "payment.recharge_sku.read"); err != nil {
		return nil, err
	}
	cursor, limit, err := hgParseRechargeSKUPage(cursorText, sizeText)
	if err != nil {
		return nil, err
	}
	items, more, err := s.repo.HGListRechargeSKUs(ctx, cursor, limit)
	if err != nil {
		return nil, err
	}
	resp := &OpsDtoPackage.HGPaymentRechargeSKUListResponse{List: make([]OpsDtoPackage.HGPaymentRechargeSKUItem, 0, len(items)), HasMore: more, Total: -1}
	for _, item := range items {
		end := ""
		if item.EndTime != nil {
			end = item.EndTime.UTC().Format(time.RFC3339Nano)
		}
		resp.List = append(resp.List, OpsDtoPackage.HGPaymentRechargeSKUItem{SKUID: item.SKUID, SKUCode: item.SKUCode, Title: item.Title, Currency: item.Currency, PayAmount: item.PayAmount, CoinAmount: item.CoinAmount, BonusCoin: item.BonusCoin, TotalCoin: item.TotalCoin, Status: item.Status, SortOrder: item.SortOrder, StartTime: item.StartTime.UTC().Format(time.RFC3339Nano), EndTime: end, Version: item.Version, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	}
	if more {
		resp.NextCursor = strconv.FormatUint(items[len(items)-1].ID, 10)
	}
	return resp, nil
}

func hgParseRechargeSKUPage(cursorText, sizeText string) (uint64, int, error) {
	parse := func(text string) (uint64, error) {
		for _, c := range text {
			if c < '0' || c > '9' {
				return 0, ErrHGRechargeSKUInvalid
			}
		}
		v, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return 0, ErrHGRechargeSKUInvalid
		}
		return v, nil
	}
	var cursor uint64
	if cursorText != "" {
		var err error
		cursor, err = parse(cursorText)
		if err != nil {
			return 0, 0, err
		}
	}
	limit := uint64(20)
	if sizeText != "" {
		var err error
		limit, err = parse(sizeText)
		if err != nil || limit == 0 {
			return 0, 0, ErrHGRechargeSKUInvalid
		}
	}
	if limit > 100 {
		limit = 100
	}
	return cursor, int(limit), nil
}
