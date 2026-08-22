package SQLQueriesPackage

const (
	// UpsertCrawlerExternalContentsPrefix 与 Suffix 由 crawler repository 组合固定上限的批量 VALUES。
	// 唯一键 (platform, content_id) 保证周期任务和失败重试不会产生重复内容。
	UpsertCrawlerExternalContentsPrefix = `
INSERT INTO crawler_external_contents (
    platform, content_id, title, author_id, author_name, cover_url, target_url,
    duration_seconds, view_count, like_count, comment_count, published_at, last_seen_at
) VALUES `
	UpsertCrawlerExternalContentsValue = `(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// InsertCrawlerExternalContentsNoopSuffix inserts missing unique keys without changing existing rows.
	// The repository reads RowsAffected before the full upsert so the transaction returns the exact new-row count.
	InsertCrawlerExternalContentsNoopSuffix = `
ON DUPLICATE KEY UPDATE id = id`
	UpsertCrawlerExternalContentsSuffix = `
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    author_id = VALUES(author_id),
    author_name = VALUES(author_name),
    cover_url = VALUES(cover_url),
    target_url = VALUES(target_url),
    duration_seconds = VALUES(duration_seconds),
    view_count = VALUES(view_count),
    like_count = VALUES(like_count),
    comment_count = VALUES(comment_count),
    published_at = VALUES(published_at),
    last_seen_at = VALUES(last_seen_at),
    updated_at = CURRENT_TIMESTAMP(6)`

	// UpsertCrawlerTaskExternalContentsPrefix associates one task/run with the globally deduplicated batch.
	// The repository appends a bounded list of (platform,content_id) predicates; one INSERT SELECT avoids N+1 lookups.
	UpsertCrawlerTaskExternalContentsPrefix = `
INSERT INTO crawler_task_external_contents (task_definition_id, external_content_id, last_run_id)
SELECT ?, id, ?
FROM crawler_external_contents
WHERE `
	UpsertCrawlerTaskExternalContentsKey    = `(platform = ? AND content_id = ?)`
	UpsertCrawlerTaskExternalContentsSuffix = `
ON DUPLICATE KEY UPDATE
    last_run_id = GREATEST(last_run_id, VALUES(last_run_id)),
    updated_at = CURRENT_TIMESTAMP(6)`

	// GetCrawlerExternalContentsFirstSQL 使用 recent 索引读取外部内容首页，多查一条判断 hasMore。
	GetCrawlerExternalContentsFirstSQL = `
SELECT id, platform, content_id, title, author_id, author_name, cover_url, target_url,
       duration_seconds, view_count, like_count, comment_count, published_at, last_seen_at
FROM crawler_external_contents
ORDER BY last_seen_at DESC, id DESC
LIMIT ?`

	// GetCrawlerExternalContentsByCursorSQL 使用 (last_seen_at, id) 复合游标，避免 OFFSET 深分页。
	GetCrawlerExternalContentsByCursorSQL = `
SELECT id, platform, content_id, title, author_id, author_name, cover_url, target_url,
       duration_seconds, view_count, like_count, comment_count, published_at, last_seen_at
FROM crawler_external_contents
WHERE last_seen_at < ? OR (last_seen_at = ? AND id < ?)
ORDER BY last_seen_at DESC, id DESC
LIMIT ?`

	GetCrawlerExternalContentByIDSQL = `
SELECT id, platform, content_id, title, author_id, author_name, cover_url, target_url,
       duration_seconds, view_count, like_count, comment_count, published_at, last_seen_at
FROM crawler_external_contents
WHERE platform = ? AND content_id = ?
LIMIT 1`

	GetCrawlerExternalContentCountSQL = `SELECT COUNT(*) FROM crawler_external_contents`

	InsertCrawlerTaskDefinitionSQL = `
INSERT INTO crawler_task_definitions (
    name, platform, enabled, cron, parser_type, item_path, max_items, configuration,
    version, created_by, updated_by
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`

	// UpdateCrawlerTaskDefinitionSQL uses the primary key plus version as the optimistic write boundary.
	UpdateCrawlerTaskDefinitionSQL = `
UPDATE crawler_task_definitions
SET name = ?, platform = ?, enabled = ?, cron = ?, parser_type = ?, item_path = ?,
    max_items = ?, configuration = ?, version = version + 1, updated_by = ?
WHERE id = ? AND version = ?`

	GetCrawlerTaskDefinitionByIDSQL = `
SELECT id, name, platform, enabled, cron, parser_type, item_path, max_items, configuration,
       last_run_id, last_run_status, last_run_started_at, last_run_finished_at,
       last_run_item_count, last_run_error, version, created_by, updated_by, created_at, updated_at
FROM crawler_task_definitions
WHERE id = ?
LIMIT 1`

	// ListCrawlerTaskDefinitionsSQL scans the primary key in ascending order without OFFSET.
	ListCrawlerTaskDefinitionsSQL = `
SELECT id, name, platform, enabled, cron, parser_type, item_path, max_items, configuration,
       last_run_id, last_run_status, last_run_started_at, last_run_finished_at,
       last_run_item_count, last_run_error, version, created_by, updated_by, created_at, updated_at
FROM crawler_task_definitions
WHERE id > ?
ORDER BY id ASC
LIMIT ?`

	// ListCrawlerTaskExternalContentsFirstSQL uses (task_definition_id,id) for a stable newest-first page.
	ListCrawlerTaskExternalContentsFirstSQL = `
SELECT a.id, a.task_definition_id, a.last_run_id,
       c.id, c.platform, c.content_id, c.title, c.author_id, c.author_name, c.cover_url, c.target_url,
       c.duration_seconds, c.view_count, c.like_count, c.comment_count, c.published_at,
       c.first_seen_at, c.last_seen_at, c.created_at, c.updated_at,
       a.created_at, a.updated_at
FROM crawler_task_external_contents a
JOIN crawler_external_contents c ON c.id = a.external_content_id
WHERE a.task_definition_id = ?
ORDER BY a.id DESC
LIMIT ?`

	// ListCrawlerTaskExternalContentsByCursorSQL continues the indexed reverse scan without OFFSET.
	ListCrawlerTaskExternalContentsByCursorSQL = `
SELECT a.id, a.task_definition_id, a.last_run_id,
       c.id, c.platform, c.content_id, c.title, c.author_id, c.author_name, c.cover_url, c.target_url,
       c.duration_seconds, c.view_count, c.like_count, c.comment_count, c.published_at,
       c.first_seen_at, c.last_seen_at, c.created_at, c.updated_at,
       a.created_at, a.updated_at
FROM crawler_task_external_contents a
JOIN crawler_external_contents c ON c.id = a.external_content_id
WHERE a.task_definition_id = ? AND a.id < ?
ORDER BY a.id DESC
LIMIT ?`

	// ListEnabledCrawlerTaskDefinitionsSQL uses (enabled,id) and always receives a repository-capped limit.
	ListEnabledCrawlerTaskDefinitionsSQL = `
SELECT id, name, platform, enabled, cron, parser_type, item_path, max_items, configuration,
       last_run_id, last_run_status, last_run_started_at, last_run_finished_at,
       last_run_item_count, last_run_error, version, created_by, updated_by, created_at, updated_at
FROM crawler_task_definitions
WHERE enabled = 1
ORDER BY id ASC
LIMIT ?`

	InsertCrawlerTaskRunSQL = `
INSERT INTO crawler_task_runs (task_definition_id, status, configuration, started_at)
VALUES (?, ?, ?, ?)`

	// CompleteCrawlerTaskRunSQL permits one running-to-terminal transition by run and definition IDs.
	CompleteCrawlerTaskRunSQL = `
UPDATE crawler_task_runs
SET status = ?, finished_at = ?, item_count = ?, error_message = ?
WHERE id = ? AND task_definition_id = ? AND status = 'running'`

	// UpdateCrawlerTaskDefinitionLastRunSQL prevents an older run from replacing a newer summary.
	UpdateCrawlerTaskDefinitionLastRunSQL = `
UPDATE crawler_task_definitions
SET last_run_id = ?, last_run_status = ?, last_run_started_at = ?, last_run_finished_at = ?,
    last_run_item_count = ?, last_run_error = ?
WHERE id = ? AND (last_run_id IS NULL OR last_run_id < ?)`

	// GetVideoListItemByIDSQL 点查原生视频或外部内容，供详情页刷新和直达链接恢复数据。
	GetVideoListItemByIDSQL = `
SELECT
    submission_id, user_id, title, cover_url, category, video_type, description,
    visibility, status, video_count, total_size, submit_time, created_at,
    video_id, file_path, file_name, file_size, mime_type, part_number,
    playback_type, source_platform, external_content_id, target_url, author_name,
    duration_seconds, view_count, like_count, comment_count
FROM (
    SELECT
        vs.submission_id, vs.user_id, vs.title, vs.cover_url, vs.category, vs.video_type,
        vs.description, vs.visibility, vs.status, vs.video_count, vs.total_size,
        vs.submit_time, vs.created_at, vf.video_id, vf.file_path, vf.file_name,
        vf.file_size, vf.mime_type, vf.part_number, 'native_file' AS playback_type,
        '' AS source_platform, '' AS external_content_id, '' AS target_url,
        '' AS author_name, COALESCE(vf.duration, 0) AS duration_seconds,
        0 AS view_count, 0 AS like_count, 0 AS comment_count
    FROM video_submissions vs
    LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
    WHERE (vf.video_id = ? OR vs.submission_id = ?) AND vs.status IN ('reviewing', 'published')
    LIMIT 1
) native_item
UNION ALL
SELECT
    CONCAT('external:', platform, ':', content_id), author_id, title, cover_url,
    'Bilibili', '转载', '', 'public', 'published', 1, 0,
    last_seen_at, created_at, CONCAT('external:', platform, ':', content_id),
    '', '', 0, '', 1, 'external_link', platform, content_id, target_url,
    author_name, duration_seconds, view_count, like_count, comment_count
FROM crawler_external_contents
WHERE CONCAT('external:', platform, ':', content_id) = ?
LIMIT 1`
)
