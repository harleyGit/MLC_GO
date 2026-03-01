/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 15:43:02
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-25 20:59:20
 * @FilePath: /MLC_GO/internal/repository/hg_base_repo.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package RepositoryPackage

import (
	"context"
	"database/sql"
)

type HGBaseRepo struct {
	db *sql.DB
}

func NewBaseRepo(db *sql.DB) *HGBaseRepo {
	return  &HGBaseRepo{db:db}
}

func (r *HGBaseRepo) Exec(
	ctx context.Context,
	query string,
	args ...any,
) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

/* 返回一行查询结果 */
func (r *HGBaseRepo) QueryRow(
	ctx context.Context,
	query string,
	args ...any,
) *sql.Row {
	return r.db.QueryRowContext(ctx, query, args...)
}

/* 返回多行查询结果 */
func (r *HGBaseRepo) QueryContext(
	ctx context.Context,
	query string,
	args ...any,
) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}