/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:54:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-11 10:11:37
 * @FilePath: /MLC_GO/internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package SQLQueriesPackage

// 数据库
const (
	// 数据库dsn
	DB_DSN = `root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?charset=utf8mb4&parseTime=True&loc=UTC`
)

// 用户模块
const (
	// InsertUserSQL 新增用户基础账号信息，写入业务 user_id、用户名、邮箱、手机号、密码哈希和盐。
	InsertUserSQL = `
	INSERT INTO users (user_id, user_name, email, phone, password_hash, salt) 
	VALUES (?, ?, ?, ?, ?, ?)`
	// GetUserByEmailOrPhoneSQL 登录时按邮箱或手机号查找用户认证信息。
	GetUserByEmailOrPhoneSQL = `SELECT user_id, user_name, email, phone, password_hash, salt 
	FROM users 
	WHERE email = ? OR phone = ?`
	// UserTotalNumSQL 统计 users 表总行数，供 offset 分页和 total 缓存回源使用。
	UserTotalNumSQL = `SELECT COUNT(*) FROM users`
	// InsertUserInfoSQL 仅插入邮箱、手机号、密码哈希和盐的历史 SQL，保留给旧调用兼容。
	InsertUserInfoSQL = `INSERT INTO users(email, phone, password_hash, salt) VALUES (?, ?, ?, ?)`
	// GetUserByIDSQL 按数据库自增主键 id 查询用户资料。
	GetUserByIDSQL = "SELECT id, user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE id = ?"
	// UpdateUserInfoByIDSQL 按数据库自增主键 id 更新用户邮箱和手机号。
	UpdateUserInfoByIDSQL = `UPDATE users SET email = ?, phone = ? WHERE id = ?`
	// UpdateUserInfoByUserIDSQL 按业务 user_id 更新用户邮箱和手机号，是对外用户接口优先使用的更新条件。
	UpdateUserInfoByUserIDSQL = `UPDATE users SET email = ?, phone = ? WHERE user_id = ?`
	// GetUserByUserIDSQL 按业务 user_id 查询用户资料。
	GetUserByUserIDSQL = "SELECT id, user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE user_id = ?"
	// SelectUserInfoByPhoneSQL 按手机号查询登录签发 token 所需的用户 id、业务 user_id 和密码字段。
	SelectUserInfoByPhoneSQL = `SELECT id, user_id, email, phone, password_hash, salt
	FROM users WHERE phone = ?`
	// QueryUserPageSQL 使用 LIMIT/OFFSET 做传统分页；数据量大或页码很深时会扫描较多行。
	QueryUserPageSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ? OFFSET ?`
	// QueryUserPageFirstSQL 游标分页首屏查询，按 id 倒序取最新用户列表。
	QueryUserPageFirstSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ?`
	// QueryUserPageV2SQL 游标分页下一页查询，取 id 小于 cursor 的数据，避免深分页 offset 扫描。
	QueryUserPageV2SQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users WHERE id < ? ORDER BY id DESC LIMIT ?`
	// GetUserByUsernameSQL 按用户名查询认证信息；注意当前 SQL 使用 username 字段名，需与表结构保持一致。
	GetUserByUsernameSQL = "SELECT user_id, username, email, phone, password_hash, salt FROM users WHERE username = ?"
	// UpdateUserPasswordSQL 按业务 user_id 更新密码哈希和盐。
	UpdateUserPasswordSQL = "UPDATE users SET password_hash = ?, salt = ? WHERE user_id = ?"
	// DeleteUserSQL 按业务 user_id 删除用户记录。
	DeleteUserSQL = "DELETE FROM users WHERE user_id = ?"
)

// 安全模块
const (
	// SelectUserSecurityBaseForUpdateSQL 在事务内锁定 users 行，并读取初始化 user_security 所需的认证字段。 FOR UPDATE 给查出来的行加锁（排他锁/X锁）
	SelectUserSecurityBaseForUpdateSQL = `SELECT user_id, email, phone, password_hash, salt FROM users WHERE user_id = ? FOR UPDATE`
	// SelectUserSecurityIDForUpdateSQL 在事务内锁定 user_security 行，用于判断安全记录是否已存在并防止并发写冲突。
	SelectUserSecurityIDForUpdateSQL = `SELECT id FROM user_security WHERE user_id = ? FOR UPDATE`
	// InsertUserSecuritySQL 创建用户安全记录，保存邮箱、手机号、密码哈希、盐、QQ 和微信等安全资料。
	InsertUserSecuritySQL = `INSERT INTO user_security (user_id, email, phone, password_hash, salt, qq, wechat) VALUES (?, ?, ?, ?, ?, ?, ?)`
	// SelectUserSecurityByUserIDSQL 按业务 user_id 读取账号安全表全部字段。
	SelectUserSecurityByUserIDSQL = `SELECT id, user_id, email, phone, password_hash, salt, qq, wechat, created_at, updated_at
	FROM user_security WHERE user_id = ?`
)

// 朋友圈模块
const ()

// 视频模块
const (
	// InsertOrUpdateVideoSubmissionSQL 创建或更新稿件记录。
	// 第一个分 P 会创建 video_submissions；后续分 P 复用 submission_id 并累加数量和大小。
	InsertOrUpdateVideoSubmissionSQL = `
INSERT INTO video_submissions (
    submission_id, user_id, title, video_count, total_size, status
) VALUES (?, ?, ?, 1, ?, 'draft')
ON DUPLICATE KEY UPDATE
    video_count = video_count + 1,
    total_size = total_size + VALUES(total_size),
    updated_at = CURRENT_TIMESTAMP`

	// InsertOrUpdateVideoFileSQL 创建或更新视频文件记录。
	// 上传完成后写入文件信息，包括路径、大小、MD5 等。
	InsertOrUpdateVideoFileSQL = `
INSERT INTO video_files (
    video_id, submission_id, user_id, part_number, title, file_name, file_path,
    file_size, mime_type, md5, upload_status, upload_progress, transcode_status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'completed', 100.00, 'pending')
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    file_name = VALUES(file_name),
    file_path = VALUES(file_path),
    file_size = VALUES(file_size),
    mime_type = VALUES(mime_type),
    md5 = VALUES(md5),
    upload_status = 'completed',
    upload_progress = 100.00,
    updated_at = CURRENT_TIMESTAMP`

	// SaveSubmissionSQL 保存完整稿件配置。
	// 因为本模块不使用外键，所有写入都带 userID/submissionID 限定，避免误更新其他用户数据。
	SaveSubmissionSQL = `
INSERT INTO video_submissions (
    submission_id, user_id, title, cover_url, category, video_type, source_url, description,
    allow_secondary_creation, watermark, visibility, declaration, card_config, dolby_audio,
    hires_audio, close_danmaku, close_comment, featured_comment, dynamic_description,
    hide_from_profile, video_count, total_size, status, submit_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    cover_url = VALUES(cover_url),
    category = VALUES(category),
    video_type = VALUES(video_type),
    source_url = VALUES(source_url),
    description = VALUES(description),
    allow_secondary_creation = VALUES(allow_secondary_creation),
    watermark = VALUES(watermark),
    visibility = VALUES(visibility),
    declaration = VALUES(declaration),
    card_config = VALUES(card_config),
    dolby_audio = VALUES(dolby_audio),
    hires_audio = VALUES(hires_audio),
    close_danmaku = VALUES(close_danmaku),
    close_comment = VALUES(close_comment),
    featured_comment = VALUES(featured_comment),
    dynamic_description = VALUES(dynamic_description),
    hide_from_profile = VALUES(hide_from_profile),
    video_count = VALUES(video_count),
    total_size = VALUES(total_size),
    status = VALUES(status),
    submit_time = VALUES(submit_time),
    updated_at = CURRENT_TIMESTAMP`

	// GetSubmissionTotalsSQL 根据已上传的视频文件重新计算稿件总数和总大小。
	// 这样前端不需要可信地上报 video_count/total_size，避免被篡改。
	GetSubmissionTotalsSQL = `
SELECT COUNT(*), COALESCE(SUM(file_size), 0)
FROM video_files
WHERE submission_id = ? AND user_id = ?`

	// UpdateVideoFileConfigSQL 更新单个分 P 的表单配置。
	UpdateVideoFileConfigSQL = `
UPDATE video_files
SET part_number = ?, title = ?, cover_url = ?, video_type = ?, source_url = ?, category = ?, description = ?, updated_at = CURRENT_TIMESTAMP
WHERE video_id = ? AND submission_id = ? AND user_id = ?`

	// DeleteVideoTagsByVideoIDSQL 删除视频的所有标签。
	DeleteVideoTagsByVideoIDSQL = `DELETE FROM video_tags WHERE video_id = ?`

	// InsertVideoTagSQL 插入视频标签。
	InsertVideoTagSQL = `INSERT INTO video_tags (video_id, tag_name) VALUES (?, ?)`

	// DeleteScheduledPublishSQL 删除稿件的定时发布配置。
	DeleteScheduledPublishSQL = `DELETE FROM video_scheduled_publish WHERE submission_id = ? AND user_id = ?`

	// InsertOrUpdateScheduledPublishSQL 创建或更新定时发布配置。
	InsertOrUpdateScheduledPublishSQL = `
INSERT INTO video_scheduled_publish (submission_id, user_id, scheduled_time, status)
VALUES (?, ?, ?, 'pending')
ON DUPLICATE KEY UPDATE
    scheduled_time = VALUES(scheduled_time),
    status = 'pending',
    updated_at = CURRENT_TIMESTAMP`

	// DeleteCommercialPromotionSQL 删除稿件的商业推广配置。
	DeleteCommercialPromotionSQL = `DELETE FROM video_commercial_promotion WHERE submission_id = ? AND user_id = ?`

	// InsertOrUpdateCommercialPromotionSQL 创建或更新商业推广配置。
	InsertOrUpdateCommercialPromotionSQL = `
INSERT INTO video_commercial_promotion (submission_id, user_id, promotion_type, promotion_name, promotion_form)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    promotion_type = VALUES(promotion_type),
    promotion_name = VALUES(promotion_name),
    promotion_form = VALUES(promotion_form),
    updated_at = CURRENT_TIMESTAMP`

	// GetVideoListSQL 获取已提交审核的视频列表。
	// 使用延迟关联优化深分页，先查主键再回表，避免千万级数据量下 OFFSET 扫描过多行。
	// 索引要求：video_submissions 表需要 (status, submit_time) 联合索引。
	GetVideoListSQL = `
SELECT
    vs.submission_id,
    vs.user_id,
    vs.title,
    vs.cover_url,
    vs.category,
    vs.video_type,
    vs.description,
    vs.visibility,
    vs.status,
    vs.video_count,
    vs.total_size,
    vs.submit_time,
    vs.created_at,
    vf.video_id,
    vf.file_path,
    vf.file_name,
    vf.file_size,
    vf.mime_type,
    vf.part_number
FROM video_submissions vs
INNER JOIN (
    SELECT submission_id
    FROM video_submissions
    WHERE status IN ('reviewing', 'published')
    ORDER BY submit_time DESC
    LIMIT ? OFFSET ?
) AS vs_page ON vs.submission_id = vs_page.submission_id
LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
ORDER BY vs.submit_time DESC`

	// GetVideoListTotalSQL 获取已提交审核的视频总数。
	// 使用近似值优化：当数据量超过一定阈值时，可改用 SHOW TABLE STATUS 或缓存。
	GetVideoListTotalSQL = `
SELECT COUNT(*)
FROM video_submissions
WHERE status IN ('reviewing', 'published')`

	// CreateVideoSubmissionStatusTimeIndexSQL 创建视频稿件状态和提交时间联合索引。
	// 用于优化 GetVideoListSQL 的查询性能。
	CreateVideoSubmissionStatusTimeIndexSQL = `
CREATE INDEX IF NOT EXISTS idx_video_submissions_status_submit_time
ON video_submissions (status, submit_time DESC)`

	// GetVideoListByCursorFirstSQL 游标分页首页查询。
	// 使用 (status, submit_time DESC) 联合索引覆盖排序，避免 filesort。
	// 多查一条用于判断是否还有下一页。
	GetVideoListByCursorFirstSQL = `
SELECT
    vs.submission_id,
    vs.user_id,
    vs.title,
    vs.cover_url,
    vs.category,
    vs.video_type,
    vs.description,
    vs.visibility,
    vs.status,
    vs.video_count,
    vs.total_size,
    vs.submit_time,
    vs.created_at,
    vf.video_id,
    vf.file_path,
    vf.file_name,
    vf.file_size,
    vf.mime_type,
    vf.part_number
FROM video_submissions vs
INNER JOIN (
    SELECT submission_id
    FROM video_submissions
    WHERE status IN ('reviewing', 'published')
    ORDER BY submit_time DESC, submission_id DESC
    LIMIT ?
) AS vs_page ON vs.submission_id = vs_page.submission_id
LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
ORDER BY vs.submit_time DESC, vs.submission_id DESC`

	// GetVideoListByCursorSQL 游标分页翻页查询。
	// 使用 (submit_time, submission_id) 复合游标定位，避免 OFFSET 扫描。
	// submit_time 相同时用 submission_id 保证分页结果稳定不丢不重。
	GetVideoListByCursorSQL = `
SELECT
    vs.submission_id,
    vs.user_id,
    vs.title,
    vs.cover_url,
    vs.category,
    vs.video_type,
    vs.description,
    vs.visibility,
    vs.status,
    vs.video_count,
    vs.total_size,
    vs.submit_time,
    vs.created_at,
    vf.video_id,
    vf.file_path,
    vf.file_name,
    vf.file_size,
    vf.mime_type,
    vf.part_number
FROM video_submissions vs
INNER JOIN (
    SELECT submission_id
    FROM video_submissions
    WHERE status IN ('reviewing', 'published')
      AND (submit_time < ? OR (submit_time = ? AND submission_id < ?))
    ORDER BY submit_time DESC, submission_id DESC
    LIMIT ?
) AS vs_page ON vs.submission_id = vs_page.submission_id
LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
ORDER BY vs.submit_time DESC, vs.submission_id DESC`
)

// 聊天模块
const ()
