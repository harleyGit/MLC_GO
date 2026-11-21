package notification

import (
	Config "MLC_GO/internal/pkg/config"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestHGDebugRoleTemplatePublish 仅在显式授权的本机 debug 执行；复制而非覆盖现有草稿，再发布新版本。
func TestHGDebugRoleTemplatePublish(t *testing.T) {
	if os.Getenv("MLC_IAM_TEMPLATE_PUBLISH_INTEGRATION") != "1" {
		t.Skip("未启用本机 debug 角色模板发布验收")
	}
	if err := Config.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Config.GetMySQLConfig()
	if err != nil {
		t.Fatal(err)
	}
	host := net.ParseIP(cfg.Host)
	if host == nil || !host.IsLoopback() {
		t.Fatal("只允许本机 debug 数据库执行角色模板验收")
	}
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp",
		Addr: net.JoinHostPort(cfg.Host, cfg.Port), DBName: cfg.Database,
		ParseTime: true}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("无法打开本机 debug 数据库")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var id uint64
	if err = db.QueryRowContext(ctx, SQL.HGIAMRoleTemplateIDSQL).Scan(&id); err != nil {
		t.Fatal("现有角色更新模板不存在")
	}
	repo := HGNewTemplateRepository(db)
	detail, err := repo.HGDetail(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PublishedVersion != nil && *detail.PublishedVersion == detail.Version {
		if err := HGValidateRolePublished(detail.TitleTemplate, detail.BodyTemplate, detail.VariablesSchema, detail.Channels); err != nil {
			t.Fatal("已发布的角色模板缺少实时渠道")
		}
		return
	}
	if detail.Version != 1 || detail.PublishedVersion != nil || detail.Status != "draft" {
		t.Fatal("已有其他管理员版本或发布记录，禁止自动覆盖")
	}
	form := HGTemplateForm{
		Name: detail.Name, Category: detail.Category,
		TitleTemplate: detail.TitleTemplate, BodyTemplate: detail.BodyTemplate,
		VariablesSchema: detail.VariablesSchema, ActionType: detail.ActionType,
		ActionConfig: detail.ActionConfig, Channels: []byte(`["in_app","realtime"]`),
		Priority: detail.Priority, Locale: detail.Locale, Version: detail.Version,
	}
	if err := HGValidateTemplateForm(&form, false); err != nil {
		t.Fatal("现有草稿内容不满足角色通知约束")
	}
	if err := HGValidateRolePublished(form.TitleTemplate, form.BodyTemplate, form.VariablesSchema, form.Channels); err != nil {
		t.Fatal("现有草稿内容不能安全启用实时渠道")
	}
	if err := repo.HGUpdate(ctx, id, detail.Version, form, "system_debug_iam"); err != nil {
		t.Fatal(err)
	}
	if err := repo.HGPublish(ctx, id, detail.Version+1, "system_debug_iam"); err != nil {
		t.Fatal(err)
	}
}
