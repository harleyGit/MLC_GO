package repository

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
)

// HGSpiderSwitchRepository 保存独立 Bilibili 推荐 worker 的期望状态与每次切换审计。
// 状态在数据库提交后生效，worker 异步观察并应用；不代表抓取已成功。
type HGSpiderSwitchRepository struct {
	db *sql.DB // 仅访问 crawler_spider_switches 和 crawler_spider_switch_events。
}

// NewHGSpiderSwitchRepository 复用已有连接池，不自行管理连接生命周期。
func NewHGSpiderSwitchRepository(db *sql.DB) *HGSpiderSwitchRepository {
	return &HGSpiderSwitchRepository{db: db}
}

// HGDesiredRunning 按主键读取权威期望值；缺失种子行按错误处理，不能擅自启动 worker。
func (r *HGSpiderSwitchRepository) HGDesiredRunning(ctx context.Context) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("爬虫开关数据库不可用")
	}
	var desired bool
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGGetSpiderSwitchSQL).Scan(&desired)
	return desired, err
}

// HGSetDesiredRunning 串行更新单条期望状态并写审计；重复目标状态幂等，不生成虚假切换记录。
func (r *HGSpiderSwitchRepository) HGSetDesiredRunning(ctx context.Context, desired bool, actor string) (bool, error) {
	if r == nil || r.db == nil || actor == "" || len(actor) > 128 {
		return false, errors.New("爬虫开关参数或数据库无效")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var previous bool
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGLockSpiderSwitchSQL).Scan(&previous); err != nil {
		return false, err
	}
	if previous == desired {
		return false, tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGUpdateSpiderSwitchSQL, desired, actor); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertSpiderSwitchEventSQL, desired, actor); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
