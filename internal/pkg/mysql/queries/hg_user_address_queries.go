package SQLQueriesPackage

const (
	// HGAddressLockUserSQL：所有地址写入先按 users.user_id 唯一索引锁定所属用户，串行化默认地址变更。
	HGAddressLockUserSQL = `SELECT id FROM users WHERE user_id = ? FOR UPDATE`
	// HGAddressCountSQL：依赖 idx_user_addresses_user_id，仅用于锁住用户行后限制每人最多20条。
	HGAddressCountSQL = `SELECT COUNT(*) FROM user_addresses WHERE user_id = ?`
	// HGAddressInsertSQL：只在所属用户行锁定后调用，第一条地址成为默认。
	HGAddressInsertSQL = `INSERT INTO user_addresses (address_id, user_id, receiver_name, receiver_phone, region, province_code, city_code, detail_address, is_default) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// HGAddressListSQL：使用 idx_user_addresses_user_id，最多20条，主键倒序稳定排序；无深 offset。
	HGAddressListSQL = `SELECT id, address_id, receiver_name, receiver_phone, region, province_code, city_code, detail_address, is_default FROM user_addresses WHERE user_id = ? ORDER BY id DESC LIMIT 20`
	// HGAddressGetSQL：按主键定位并校验所属用户，防止跨用户读取。
	HGAddressGetSQL = `SELECT id, address_id, receiver_name, receiver_phone, region, province_code, city_code, detail_address, is_default FROM user_addresses WHERE id = ? AND user_id = ?`
	// HGAddressUpdateSQL：按主键及归属更新；不覆盖默认状态。
	HGAddressUpdateSQL = `UPDATE user_addresses SET receiver_name = ?, receiver_phone = ?, region = ?, province_code = ?, city_code = ?, detail_address = ? WHERE id = ? AND user_id = ?`
	// HGAddressDeleteSQL：仅删除本人地址，事务内先锁所属用户行。
	HGAddressDeleteSQL = `DELETE FROM user_addresses WHERE id = ? AND user_id = ?`
	// HGAddressClearDefaultSQL：按归属清除旧默认，事务内在新默认设置之前执行。
	HGAddressClearDefaultSQL = `UPDATE user_addresses SET is_default = 0 WHERE user_id = ? AND is_default = 1`
	// HGAddressSetDefaultSQL：按主键与归属设置默认；事务内先确认该地址存在。
	HGAddressSetDefaultSQL = `UPDATE user_addresses SET is_default = 1 WHERE id = ? AND user_id = ?`
	// HGAddressPromoteSQL：删除默认地址后按主键倒序选一条作为新默认；仍在同一事务。
	HGAddressPromoteSQL = `UPDATE user_addresses SET is_default = 1 WHERE user_id = ? ORDER BY id DESC LIMIT 1`
)
