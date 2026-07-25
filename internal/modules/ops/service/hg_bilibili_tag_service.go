package OpsServicePackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	"context"
	"errors"
	"strings"
)

const (
	// maxBilibiliTagNameLen 与 bilibili_douga_tags.name VARCHAR(32) 保持一致，按 Unicode 字符而不是字节校验。
	maxBilibiliTagNameLen = 32
	// maxBilibiliTagSortOrder 限制异常大排序值，避免前端误传导致不可维护的数据跨度。
	maxBilibiliTagSortOrder = 1000000
)

// CreateBilibiliTag 创建动画标签并失效活跃列表缓存。
// 唯一键冲突由 Repository 转换为“标签名称已存在”，用于并发重复提交时稳定返回冲突语义。
func (s *Service) CreateBilibiliTag(ctx context.Context, operatorID string, req OpsDtoPackage.BilibiliTagRequest) (*OpsDtoPackage.BilibiliTagItem, error) {
	name, err := normalizeBilibiliTagName(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := normalizeBilibiliTagStatus(req.Status)
	if err != nil {
		return nil, err
	}
	if req.SortOrder < 0 || req.SortOrder > maxBilibiliTagSortOrder {
		return nil, errors.New("标签排序值无效")
	}
	item, err := s.repo.CreateBilibiliTag(ctx, operatorID, name, req.SortOrder, status)
	if err != nil {
		return nil, err
	}
	s.invalidateBilibiliTagCache(ctx)
	result := toBilibiliTagItem(item)
	return &result, nil
}

// UpdateBilibiliTag 更新动画标签并失效活跃列表缓存。
func (s *Service) UpdateBilibiliTag(ctx context.Context, operatorID string, req OpsDtoPackage.UpdateBilibiliTagRequest) (*OpsDtoPackage.BilibiliTagItem, error) {
	tagID, err := normalizeBilibiliTagID(req.TagID)
	if err != nil {
		return nil, err
	}
	name, err := normalizeBilibiliTagName(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := normalizeBilibiliTagStatus(req.Status)
	if err != nil {
		return nil, err
	}
	if req.SortOrder < 0 || req.SortOrder > maxBilibiliTagSortOrder {
		return nil, errors.New("标签排序值无效")
	}
	item, err := s.repo.UpdateBilibiliTag(ctx, operatorID, tagID, name, req.SortOrder, status)
	if err != nil {
		return nil, err
	}
	s.invalidateBilibiliTagCache(ctx)
	result := toBilibiliTagItem(item)
	return &result, nil
}

// DeleteBilibiliTag 软删除动画标签并失效活跃列表缓存。
// 删除只影响标签目录，历史 video_tags 标签快照不做级联更新，避免大表批量写和历史语义变化。
func (s *Service) DeleteBilibiliTag(ctx context.Context, operatorID string, req OpsDtoPackage.DeleteBilibiliTagRequest) error {
	tagID, err := normalizeBilibiliTagID(req.TagID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteBilibiliTag(ctx, operatorID, tagID); err != nil {
		return err
	}
	s.invalidateBilibiliTagCache(ctx)
	return nil
}

// GetBilibiliTagList 获取管理标签列表。
// activeOnly=true 用于动画展示页：优先读取 Redis 固定 key，未命中时回源最多 100 条启用标签。
// activeOnly=false 用于运维管理页：使用 id cursor 分页且 Total=-1，避免大表实时 COUNT 和 OFFSET 深分页。
func (s *Service) GetBilibiliTagList(ctx context.Context, cursor int64, pageSize int, activeOnly bool) (*OpsDtoPackage.BilibiliTagListResponse, error) {
	if activeOnly && s.cache != nil {
		// Redis 故障时降级回源 MySQL；标签读取不能因为缓存短暂不可用而整体失败。
		if resp, hit, err := s.cache.GetActiveBilibiliTags(ctx); err == nil && hit {
			return resp, nil
		}
	}
	if activeOnly {
		items, err := s.repo.GetActiveBilibiliTags(ctx)
		if err != nil {
			return nil, err
		}
		resp := buildBilibiliTagListResponse(items, false)
		if s.cache != nil {
			// 回填失败不影响本次数据库查询结果，后续请求仍可继续回源。
			_ = s.cache.SetActiveBilibiliTags(ctx, resp)
		}
		return resp, nil
	}
	if cursor < 0 {
		cursor = 0
	}
	if pageSize <= 0 {
		pageSize = defaultOpsPageSize
	}
	if pageSize > maxOpsPageSize {
		pageSize = maxOpsPageSize
	}
	items, hasMore, err := s.repo.GetBilibiliTagList(ctx, cursor, pageSize)
	if err != nil {
		return nil, err
	}
	return buildBilibiliTagListResponse(items, hasMore), nil
}

// normalizeBilibiliTagName 统一标签名称空白、保留字和数据库长度约束。
// “推荐”由前端映射为空 tagName，表示不按标签过滤，因此禁止作为普通目录标签入库。
func normalizeBilibiliTagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("标签名称不能为空")
	}
	if name == "推荐" {
		return "", errors.New("推荐为系统保留标签")
	}
	if len([]rune(name)) > maxBilibiliTagNameLen {
		return "", errors.New("标签名称不能超过32个字符")
	}
	return name, nil
}

// normalizeBilibiliTagStatus 校验状态枚举；创建请求省略 status 时默认启用。
func normalizeBilibiliTagStatus(status int) (int, error) {
	if status == 0 {
		return 1, nil
	}
	if status != 1 && status != 2 {
		return 0, errors.New("标签状态无效")
	}
	return status, nil
}

// normalizeBilibiliTagID 校验对外业务标签 ID，禁止客户端使用数据库自增主键操作标签。
func normalizeBilibiliTagID(tagID string) (string, error) {
	tagID = strings.TrimSpace(tagID)
	if len(tagID) != 32 || !strings.HasPrefix(tagID, "BLTAG_") {
		return "", errors.New("标签ID无效")
	}
	for _, char := range tagID[len("BLTAG_"):] {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", char) {
			return "", errors.New("标签ID无效")
		}
	}
	return tagID, nil
}

// buildBilibiliTagListResponse 构建统一列表响应，并用当前页最后一条 id 作为下一页 cursor。
func buildBilibiliTagListResponse(items []map[string]interface{}, hasMore bool) *OpsDtoPackage.BilibiliTagListResponse {
	list := make([]OpsDtoPackage.BilibiliTagItem, 0, len(items))
	nextCursor := ""
	for _, item := range items {
		list = append(list, toBilibiliTagItem(item))
		nextCursor = toString(item["idInt"])
	}
	return &OpsDtoPackage.BilibiliTagListResponse{Total: -1, List: list, NextCursor: nextCursor, HasMore: hasMore}
}

// toBilibiliTagItem 隔离 Repository 内部 map 与对外 DTO，避免数据库字段细节泄漏到 Handler。
func toBilibiliTagItem(item map[string]interface{}) OpsDtoPackage.BilibiliTagItem {
	return OpsDtoPackage.BilibiliTagItem{
		TagID: toString(item["tagId"]), Name: toString(item["name"]), SortOrder: toInt(item["sortOrder"]),
		Status: toInt(item["status"]), CreatedAt: toString(item["createdAt"]), UpdatedAt: toString(item["updatedAt"]),
	}
}

// invalidateBilibiliTagCache 在数据库写成功后删除活跃列表缓存。
// 删除失败不回滚已提交的 MySQL 写入，最多形成一个 TTL 周期的最终一致性窗口。
func (s *Service) invalidateBilibiliTagCache(ctx context.Context) {
	if s.cache != nil {
		_ = s.cache.InvalidateActiveBilibiliTags(ctx)
	}
}
