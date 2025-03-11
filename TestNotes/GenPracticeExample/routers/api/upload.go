/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2025-03-11 19:49:18
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-11 19:58:11

* @FilePath: /MLC_GO/TestNotes/GenPracticeExample/routers/api/upload.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* title:  上传图片的业务逻辑
*/
package api

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/upload"
	"net/http"

	"github.com/gin-gonic/gin"
)


func UploadImage(c *gin.Context) {
	code := e.SUCCESS
	data := make(map[string]string)  // 用于存储返回的数据

	file, image, err := c.Request.FormFile("image") // 获取上传的文件(（返回提供的表单键的第一个文件）
	if err != nil {
		logging.ErrInfo("获取上传图片错误：", err)
		code = e.ERROR
		c.JSON(http.StatusOK, gin.H {
			"code": code,
			"msg": e.GetMsg(code),
			"data": data,
		})
	}

	if image == nil {
		code = e.INVALID_PARAMS
	}else {
		imageName := upload.GetImageName(image.Filename)
		fullPath := upload.GetImageFullPath()
		savePath := upload.GetImagePath()

		src := fullPath + imageName
		if ! upload.CheckImageExt(imageName) || ! upload.CheckImageSize(file) {
			code = e.ERROR_UPLOAD_CHECK_IMAGE_FAIL
		}else {
			err := upload.CheckImage(fullPath)
			if err != nil {
				logging.ErrInfo("上传图片不存在err:", err)
				code = e.ERROR_UPLOAD_CHECK_IMAGE_FAIL
			}else if err := c.SaveUploadedFile(image, src); err != nil { //SaveUploadedFile 是 Gin 框架 提供的一个便捷方法，用于直接保存上传的文件到指定路径
				logging.ErrInfo("保存上传图片失败 err: ", err)
				code = e.ERROR_UPLOAD_SAVE_IMAGE_FAIL
			}else {
				data["image_url"] = upload.GetImageFullUrl(imageName)
				data["image_save_url"] = savePath + imageName
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg":  e.GetMsg(code),
		"data": data,
	})
}