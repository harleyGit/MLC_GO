package SQLQueriesPackage

// 六类业务实体按主键点查；用户/组织/环境/区域的 ID 必须经服务端可信来源复核。
// 创建事务先锁 IAM 类型，再写 IAM 资源和对应业务表；事务失败全部回滚，不在事务内做外部 IO。
const (
	HGInsertCreatorSpace         = `INSERT INTO hg_creator_spaces(scope_id,owner_user_id,name,created_by) VALUES(?,?,?,?)`
	HGInsertBusinessOrganization = `INSERT INTO hg_business_organizations(scope_id,name,created_by) VALUES(?,?,?)`
	HGInsertOperationEnvironment = `INSERT INTO hg_operation_environments(scope_id,environment_code,name,created_by) VALUES(?,?,?,?)`
	HGInsertCrawlerProject       = `INSERT INTO hg_crawler_projects(scope_id,organization_id,name,created_by) VALUES(?,?,?,?)`
	HGInsertContentChannel       = `INSERT INTO hg_content_channels(scope_id,organization_id,name,created_by) VALUES(?,?,?,?)`
	HGInsertBusinessRegion       = `INSERT INTO hg_business_regions(scope_id,region_code,name,created_by) VALUES(?,?,?,?)`
	// 创建项目/频道时共享锁住父组织的 IAM 与业务行，避免停用与子资源建立之间产生孤儿。
	HGSelectBusinessOrganization     = `SELECT 1 FROM hg_business_organizations b JOIN iam_scope_resources r ON r.scope_type='organization' AND r.scope_id=b.scope_id AND r.status=1 AND r.business_verified=1 JOIN iam_scope_types t ON t.scope_code=r.scope_type AND t.status=1 WHERE b.scope_id=? AND b.status=1 LIMIT 1 FOR SHARE`
	HGSelectBusinessCreator          = `SELECT owner_user_id,'' AS code,'' AS organization_id FROM hg_creator_spaces WHERE scope_id=? LIMIT 1`
	HGSelectBusinessOrganizationMeta = `SELECT '' AS owner_user_id,'' AS code,'' AS organization_id FROM hg_business_organizations WHERE scope_id=? LIMIT 1`
	HGSelectBusinessEnvironment      = `SELECT '' AS owner_user_id,environment_code,'' AS organization_id FROM hg_operation_environments WHERE scope_id=? LIMIT 1`
	HGSelectBusinessProject          = `SELECT '' AS owner_user_id,'' AS code,COALESCE(organization_id,'') FROM hg_crawler_projects WHERE scope_id=? LIMIT 1`
	HGSelectBusinessChannel          = `SELECT '' AS owner_user_id,'' AS code,COALESCE(organization_id,'') FROM hg_content_channels WHERE scope_id=? LIMIT 1`
	HGSelectBusinessRegion           = `SELECT '' AS owner_user_id,region_code,'' AS organization_id FROM hg_business_regions WHERE scope_id=? LIMIT 1`
	// 停用时先锁 IAM 类型及资源，再按主键停用业务实体；每次修改只影响一个确定 ID，不遍历历史绑定。
	HGSelectIAMBusinessScopeForUpdate   = `SELECT status,business_verified FROM iam_scope_resources WHERE scope_type=? AND scope_id=? LIMIT 1 FOR UPDATE`
	HGDisableIAMBusinessScope           = `UPDATE iam_scope_resources SET status=0 WHERE scope_type=? AND scope_id=? AND status=1 AND business_verified=1`
	HGDisableCreatorSpace               = `UPDATE hg_creator_spaces SET status=0 WHERE scope_id=? AND status=1`
	HGDisableBusinessOrganization       = `UPDATE hg_business_organizations SET status=0 WHERE scope_id=? AND status=1`
	HGDisableOperationEnvironment       = `UPDATE hg_operation_environments SET status=0 WHERE scope_id=? AND status=1`
	HGDisableCrawlerProject             = `UPDATE hg_crawler_projects SET status=0 WHERE scope_id=? AND status=1`
	HGDisableContentChannel             = `UPDATE hg_content_channels SET status=0 WHERE scope_id=? AND status=1`
	HGDisableBusinessRegion             = `UPDATE hg_business_regions SET status=0 WHERE scope_id=? AND status=1`
	HGSelectActiveProjectByOrganization = `SELECT 1 FROM hg_crawler_projects WHERE organization_id=? AND status=1 LIMIT 1`
	HGSelectActiveChannelByOrganization = `SELECT 1 FROM hg_content_channels WHERE organization_id=? AND status=1 LIMIT 1`
)
