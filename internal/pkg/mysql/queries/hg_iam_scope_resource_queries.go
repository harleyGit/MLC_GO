package SQLQueriesPackage

// 候选列表按 (org_type,status,id) 与 (status,id) 索引倒序游标读取，单页最多 51 条；
// 商家仅包含审核通过记录；已登记 shop 与商家可能共用 ID，调用方按 ID 去重展示。
const (
	HGSelectIAMScopeOrgsFirst  = `SELECT id,org_name FROM iam_organizations WHERE org_type=? AND status=1 ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeOrgsAfter  = `SELECT id,org_name FROM iam_organizations WHERE org_type=? AND status=1 AND id<? ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeShopsFirst = `SELECT id,merchant_id,shop_name FROM mall_merchant WHERE status=1 ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeShopsAfter = `SELECT id,merchant_id,shop_name FROM mall_merchant WHERE status=1 AND id<? ORDER BY id DESC LIMIT ?`
	// uk_iam_scope_resource 用于授权点查；候选页依赖 idx_iam_scope_resource_page，固定类型和启用状态后按 id 倒序读取 51 行。
	HGSelectIAMReadableScopeType    = `SELECT requires_scope_id FROM iam_scope_types WHERE scope_code=? AND status=1 LIMIT 1`
	HGSelectIAMScopeResourceActive  = `SELECT 1 FROM iam_scope_resources r JOIN iam_scope_types t ON t.scope_code=r.scope_type AND t.status=1 WHERE r.scope_type=? AND r.scope_id=? AND r.status=1 LIMIT 1`
	HGSelectIAMScopeResourcesFirst  = `SELECT id,scope_id,scope_name FROM iam_scope_resources WHERE scope_type=? AND status=1 ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeResourcesAfter  = `SELECT id,scope_id,scope_name FROM iam_scope_resources WHERE scope_type=? AND status=1 AND id<? ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeResourceRequest = `SELECT scope_id,scope_type,scope_name FROM iam_scope_resources WHERE created_by=? AND request_id=? LIMIT 1`
	HGInsertIAMScopeResource        = `INSERT INTO iam_scope_resources(scope_type,scope_id,scope_name,created_by,request_id) VALUES(?,?,?,?,?)`
)
