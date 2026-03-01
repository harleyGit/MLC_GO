/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-25 20:16:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-25 20:20:07
 * @FilePath: /MLC_GO/internal/repository/hg_base_repo_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package RepositoryPackage

import "context"

/* 定义分页查询通用接口 */
type HGPageQuery struct {
	Page     int
	PageSize int
}

type HGPageResult[T any] struct {
	List     []T
	Total    int
	Page     int
	Pagesize int
}

/* 定义泛型 Repository 接口 */
type HGIRepository[T any] interface {

	FindPage(ctx context.Context, query HGPageQuery) (HGPageResult[T], error)

	/* Insert(ctx context.Context, t T) error
	Update(ctx context.Context, t T) error
	Delete(ctx context.Context, t T) error
	Get(ctx context.Context, id int64) (T, error)
	GetAll(ctx context.Context) ([]T, error)
	GetByPage(ctx context.Context, query HGPageQuery) (HGPageResult[T], error)
	*/


}
