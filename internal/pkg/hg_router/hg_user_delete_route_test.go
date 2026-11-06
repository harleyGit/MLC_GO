package HGRouterPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hgDeleteRouteAuthorizer struct{ allowed bool }

func (s hgDeleteRouteAuthorizer) HasAssetPermission(_ context.Context, _, permission string) (bool, error) {
	return s.allowed && permission == string(HGPermissionPackage.HGUserDirectoryDelete), nil
}

func TestHGUserDeleteRequiresDirectoryWriteAndOtherTarget(t *testing.T) {
	var spec *RouteSpec
	for _, item := range userRoutes(&UserHandlerPackage.HGUserHandler{}) {
		if item.SubPath == "/delete" {
			current := item
			spec = &current
			break
		}
	}
	if spec == nil || spec.Method != http.MethodDelete || !spec.NeedAuth || len(spec.Permissions) != 1 || spec.Permissions[0] != HGPermissionPackage.HGUserDirectoryDelete {
		t.Fatalf("用户删除路由未声明专用写权限: %+v", spec)
	}
	for _, tc := range []struct {
		name, target string
		allowed      bool
		want         int
	}{
		{"拒绝无权限", "target", false, http.StatusForbidden},
		{"禁止删除本人", "actor", true, http.StatusForbidden},
		{"缺少目标", "", true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/delete?user_id="+tc.target, nil)
			req = req.WithContext(context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "actor"}))
			rec := httptest.NewRecorder()
			hgPermissionHandler(*spec, hgDeleteRouteAuthorizer{allowed: tc.allowed}).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("状态=%d, want=%d", rec.Code, tc.want)
			}
		})
	}
}
