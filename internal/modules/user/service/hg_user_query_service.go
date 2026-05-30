package UserServicePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserMapperPackage "MLC_GO/internal/modules/user/mapper"
	"MLC_GO/internal/pkg/logHG"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
)

// GetUserList 获取用户列表，使用 cursor 分页降低深分页对 MySQL 的扫描压力。
// 缓存 key 包含 cursor 和 size，避免不同分页请求互相覆盖。
func (s *UserService) GetUserList(
	ctx context.Context,
	cursor int64,
	size int,
) (HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO], error) {
	if s.userCache != nil {
		cacheValue, err := s.userCache.GetUserListCache(ctx, cursor, size)
		if err != nil {
			logHG.DebugFInfo("GetUserListCache err: %v", err)
		}
		if cacheValue != nil {
			return *cacheValue, nil
		}
	}

	users, nextCursor, hasMore, err := s.repo.FindByCursor(ctx, cursor, size)
	if err != nil {
		return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	}

	total, err := s.getUserListTotal(ctx)
	if err != nil {
		return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	}

	dtoList := make([]*UserDtoPackage.HGCreateUserDTO, 0, len(users))
	for _, user := range users {
		dtoList = append(dtoList, UserMapperPackage.UserModelToDTO(&user))
	}

	page := 1
	if cursor > 0 {
		page = 0
	}
	resp := HGResponsePakcage.NewPageResponse[*UserDtoPackage.HGCreateUserDTO](dtoList,
		HGResponsePakcage.WithPagesize(size),
		HGResponsePakcage.WithPage(page),
		HGResponsePakcage.WithTotal(total),
		HGResponsePakcage.WithNextCursor(nextCursor),
		HGResponsePakcage.WithHasMore(hasMore))

	if s.userCache != nil {
		if err := s.userCache.SetUserListCache(ctx, resp, cursor, size); err != nil {
			logHG.DebugFInfo("SetUserListCache err: %v", err)
		}
	}

	return resp, nil
}

// getUserListTotal 获取用户总数，优先读取 Redis 缓存。
// total 变更来自注册和资料更新等写操作，写成功后由 clearUserListCache 统一失效缓存。
func (s *UserService) getUserListTotal(ctx context.Context) (int, error) {
	if s.userCache == nil {
		return s.repo.CountUsers(ctx)
	}

	total, err := s.userCache.GetUserListTotalCache(ctx)
	if err != nil {
		logHG.DebugFInfo("GetUserListTotalCache err: %v", err)
		return s.repo.CountUsers(ctx)
	}
	if total > 0 {
		return total, nil
	}

	total, err = s.repo.CountUsers(ctx)
	if err != nil {
		return 0, err
	}
	if err = s.userCache.SetUserListTotalCache(ctx, total); err != nil {
		logHG.DebugFInfo("SetUserListTotalCache err: %v", err)
	}

	return total, nil
}

// clearUserListCache 在用户资料写操作后清理列表分页缓存和总数缓存。
// 使用 SCAN pattern 删除分页 key，避免 Redis KEYS 在高并发环境阻塞实例。
func (s *UserService) clearUserListCache(ctx context.Context) {
	if s == nil || s.redisService == nil {
		return
	}

	if err := s.redisService.DeleteFromRedis(PersistenceRedisPackage.UserListTotalKey, ctx); err != nil {
		logHG.DebugFInfo("Delete user list total cache err: %v", err)
	}
	if err := s.redisService.DeleteFromRedisByPattern(PersistenceRedisPackage.UserListPatternKey, ctx); err != nil {
		logHG.DebugFInfo("Delete user list page cache err: %v", err)
	}
}
