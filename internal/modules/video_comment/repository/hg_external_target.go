package VideoCommentRepositoryPackage

import (
	hgQueries "MLC_GO/internal/pkg/mysql/queries"
	"strings"
)

// hgCommentTargetSQL 仅外部命名空间走采集权威表，原生稿件保留可见性和关闭评论规则。
func hgCommentTargetSQL(hgID string) string {
	if strings.HasPrefix(hgID, "external:") {
		return hgQueries.HGExternalCommentTargetSQL
	}
	return hgQueries.SelectCommentableSubmissionSQL
}
