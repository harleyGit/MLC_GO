package app

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"net/http"

	"github.com/astaxie/beego/validation"
	"github.com/gin-gonic/gin"
)


// 用于将 HTTP 请求中的数据绑定到传入的 form 结构体中，并对数据进行验证。它利用了 Gin 框架的 Context 对象和一个验证器（可能是来自 github.com/astaxie/beego/validation）来确保请求数据符合预期格式和规则。
func BindAndValid(c *gin.Context, form interface{}) (int, int) {
	// 将 HTTP 请求中的数据绑定到 form 对象上。绑定过程会根据请求中的数据类型（如 JSON、表单数据等）自动解析赋值给 form 结构体
	err := c.Bind(form)
	if err != nil {
		return http.StatusBadRequest, e.INVALID_PARAMS
	}

	// 初始化了一个验证器实例，用于对数据进行规则校验
	valid := validation.Validation{}
	// 对绑定后的 form 进行验证
	check, err := valid.Valid(form)
	if err != nil {
		return http.StatusInternalServerError, e.ERROR
	}
	if !check {
		MarkErrors(valid.Errors)
		return http.StatusBadRequest, e.INVALID_PARAMS
	}

	return http.StatusOK, e.SUCCESS
}