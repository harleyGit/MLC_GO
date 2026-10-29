package iam

import (
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"context"
	"strings"
)

// HGAssetAuthorizer 将现有 ops 调用点接入 IAM 权威权限表；调用方仍传旧编码。
type HGAssetAuthorizer struct {
	service *HGService // IAM 授权服务；nil 时默认拒绝。
}

// NewHGAssetAuthorizer 创建旧调用点的只读兼容适配器。
func NewHGAssetAuthorizer(service *HGService) *HGAssetAuthorizer {
	return &HGAssetAuthorizer{service: service}
}

// HasAssetPermission 将已知旧编码转换为 IAM 三段编码，并执行全局作用域授权。
func (a *HGAssetAuthorizer) HasAssetPermission(ctx context.Context, userID, oldCode string) (bool, error) {
	if a == nil || a.service == nil {
		return false, ErrHGForbidden
	}
	code := hgLegacyPermission(oldCode)
	if !hgPermissionCodePattern.MatchString(code) {
		return false, ErrHGInvalid
	}
	active, err := a.IsActiveAdmin(ctx, userID)
	if err != nil || !active {
		return false, err
	}
	decision, err := a.service.Authorize(ctx, userID, code, HGResource{ScopeType: "global"})
	return decision.Allowed, err
}

// IsActiveAdmin 只验证数据库中的活动管理员身份，供管理端导航使用。
func (a *HGAssetAuthorizer) IsActiveAdmin(ctx context.Context, userID string) (bool, error) {
	if a == nil || a.service == nil || a.service.repo == nil {
		return false, ErrHGForbidden
	}
	return a.service.repo.hgActiveAdmin(ctx, userID)
}

// ListAssetPermissions 返回旧管理界面使用的编码；授权判断仍由服务端 IAM 执行。
func (a *HGAssetAuthorizer) ListAssetPermissions(ctx context.Context, userID string) ([]string, error) {
	if a == nil || a.service == nil || a.service.repo == nil {
		return nil, ErrHGForbidden
	}
	active, err := a.IsActiveAdmin(ctx, userID)
	if err != nil || !active {
		return nil, err
	}
	return a.service.repo.hgAssetPermissions(ctx, userID)
}

func hgLegacyPermission(code string) string {
	switch code {
	case "asset.coin.balance.read":
		return string(HGPermissionPackage.HGAssetCoinBalanceRead)
	case "asset.coin.transaction.read":
		return string(HGPermissionPackage.HGAssetCoinTransactionRead)
	case "asset.coin.correction.request":
		return string(HGPermissionPackage.HGAssetCoinCorrectionRequest)
	case "asset.coin.correction.approve":
		return string(HGPermissionPackage.HGAssetCoinCorrectionApprove)
	case "asset.coin.correction.apply":
		return string(HGPermissionPackage.HGAssetCoinCorrectionApply)
	default:
		return strings.ReplaceAll(code, ".", ":")
	}
}
