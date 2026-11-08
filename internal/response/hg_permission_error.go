package HGResponsePakcage

import "strings"

// HGPermissionError 统一授权失败的角色/权限说明；角色绑定由 IAM 动态配置，不能伪称某个固定角色已获授权。
// prerequisite 只填写公开的前置条件，不透露当前用户角色、对象所有者或资源是否存在。
func HGPermissionError(prerequisite string, match string, permissions ...string) HGErrorResult {
	message := "无权执行此操作"
	if len(permissions) > 0 {
		message += "；所需角色：由 IAM 授予相应权限且处于生效状态的角色"
		separator := "、"
		if match == "any" {
			separator = " 或 "
		}
		message += "；所需权限：" + strings.Join(permissions, separator)
	}
	if prerequisite != "" {
		message += "；前置条件：" + prerequisite
	}
	return HGErrorResult{Code: Forbidden.Code, Message: message}
}
