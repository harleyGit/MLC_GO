package SQLQueriesPackage

// HGErrorCodeCatalogInsertSQL 仅供离线目录迁移导出；小型静态字典不参与业务请求路径。
// 发布时在独立迁移中插入，不覆盖已发布编号；主键与唯一 Key 防止重复。
const HGErrorCodeCatalogInsertSQL = "INSERT INTO hg_error_code_catalog (code,error_key,module,submodule,http_status,default_message,retryable) VALUES\n"

// HGErrorCodeCatalogRowSQL 的字符串参数只能来自已校验的 Go 静态字典，文案单引号须转义。
// 这是离线 SQL 文件格式模板，严禁使用请求参数或把它当作动态执行 SQL。
const HGErrorCodeCatalogRowSQL = "(%d,'%s','%s',%d,%d,'%s',%t)"
