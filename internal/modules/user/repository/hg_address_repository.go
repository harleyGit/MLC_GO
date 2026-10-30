package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
)

// HGAddressRepository 对 user_addresses 执行归属隔离的读写；所有写事务先锁 users 行。
type HGAddressRepository struct {
	db *sql.DB // 用户地址权威 MySQL 连接池，不持有敏感字段缓存。
}

// HGNewAddressRepository 复用模块注入的 MySQL 连接池。
func HGNewAddressRepository(db *sql.DB) *HGAddressRepository { return &HGAddressRepository{db: db} }

func (hgRepo *HGAddressRepository) hgWrite(ctx context.Context, userID string, fn func(context.Context, *sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := hgRepo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启地址事务失败: %w", err)
	}
	defer tx.Rollback()
	var userKey int64
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGAddressLockUserSQL, userID).Scan(&userKey); err != nil {
		return err
	}
	if err = fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// HGCreateAddress 串行限制单用户20条，第一条自动设为默认地址。
func (hgRepo *HGAddressRepository) HGCreateAddress(ctx context.Context, userID, addressID string, input UserDtoPackage.HGAddressInput) (int64, error) {
	var id int64
	err := hgRepo.hgWrite(ctx, userID, func(ctx context.Context, tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGAddressCountSQL, userID).Scan(&count); err != nil {
			return err
		}
		if count >= 20 {
			return ErrHGAddressLimit
		}
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAddressInsertSQL, addressID, userID, input.ReceiverName, input.ReceiverPhone, input.Region, input.ProvinceCode, input.CityCode, input.DetailAddress, count == 0)
		if err != nil {
			return err
		}
		id, err = result.LastInsertId()
		return err
	})
	return id, err
}

// ErrHGAddressLimit 表示地址数达到单用户20条上限。
var ErrHGAddressLimit = errors.New("收货地址最多20条")

func hgScanAddress(row interface{ Scan(...any) error }) (UserDtoPackage.HGAddress, error) {
	var a UserDtoPackage.HGAddress
	err := row.Scan(&a.ID, &a.AddressID, &a.ReceiverName, &a.ReceiverPhone, &a.Region, &a.ProvinceCode, &a.CityCode, &a.DetailAddress, &a.IsDefault)
	return a, err
}

// HGGetAddress 只读取指定登录用户的地址。
func (hgRepo *HGAddressRepository) HGGetAddress(ctx context.Context, userID string, id int64) (UserDtoPackage.HGAddress, error) {
	return hgScanAddress(hgRepo.db.QueryRowContext(ctx, SQLQueriesPackage.HGAddressGetSQL, id, userID))
}

// HGListAddresses 返回按 id 倒序排列的最多20条本人地址。
func (hgRepo *HGAddressRepository) HGListAddresses(ctx context.Context, userID string) ([]UserDtoPackage.HGAddress, error) {
	rows, err := hgRepo.db.QueryContext(ctx, SQLQueriesPackage.HGAddressListSQL, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	addresses := make([]UserDtoPackage.HGAddress, 0)
	for rows.Next() {
		a, scanErr := hgScanAddress(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

// HGUpdateAddress 全量修改本人地址字段，不改变默认标识。
func (hgRepo *HGAddressRepository) HGUpdateAddress(ctx context.Context, userID string, id int64, input UserDtoPackage.HGAddressInput) error {
	return hgRepo.hgWrite(ctx, userID, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := hgScanAddress(tx.QueryRowContext(ctx, SQLQueriesPackage.HGAddressGetSQL, id, userID)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAddressUpdateSQL, input.ReceiverName, input.ReceiverPhone, input.Region, input.ProvinceCode, input.CityCode, input.DetailAddress, id, userID)
		return err
	})
}

func hgAddressAffected(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// HGDeleteAddress 删除本人地址；删除默认地址时事务内自动选择最新剩余地址。
func (hgRepo *HGAddressRepository) HGDeleteAddress(ctx context.Context, userID string, id int64) error {
	return hgRepo.hgWrite(ctx, userID, func(ctx context.Context, tx *sql.Tx) error {
		a, err := hgScanAddress(tx.QueryRowContext(ctx, SQLQueriesPackage.HGAddressGetSQL, id, userID))
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAddressDeleteSQL, id, userID)
		if err != nil {
			return err
		}
		if err = hgAddressAffected(result); err != nil {
			return err
		}
		if a.IsDefault {
			_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGAddressPromoteSQL, userID)
		}
		return err
	})
}

// HGSetDefaultAddress 在同一事务中切换本人默认地址；不存在则保持旧默认不变。
func (hgRepo *HGAddressRepository) HGSetDefaultAddress(ctx context.Context, userID string, id int64) error {
	return hgRepo.hgWrite(ctx, userID, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := hgScanAddress(tx.QueryRowContext(ctx, SQLQueriesPackage.HGAddressGetSQL, id, userID)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAddressClearDefaultSQL, userID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAddressSetDefaultSQL, id, userID)
		if err != nil {
			return err
		}
		return hgAddressAffected(result)
	})
}
