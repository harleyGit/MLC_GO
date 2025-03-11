/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 21:13:48
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-23 21:18:13
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/e/code.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package e

const (
	SUCCESS = 200
	ERROR = 500
	INVALID_PARAMS = 400

	ERROR_EXIST_TAG = 10001
	ERROR_NOT_EXIST_TAG = 10002
	ERROR_NOT_EXIST_ARTICLE = 10003

	ERROR_AUTH_CHECK_TOKEN_FAIL = 20001
	ERROR_AUTH_CHECK_TOKEN_TIMEOUT =20002
	ERROR_AUTH_TOKEN = 20003
	ERROR_AUTH = 20004

	// 保存图片失败
	ERROR_UPLOAD_SAVE_IMAGE_FAIL = 30001
	// 检查图片失败
	ERROR_UPLOAD_CHECK_IMAGE_FAIL = 30002
	// 校验图片错误，图片格式或大小有问题
	ERROR_UPLOAD_CHECK_IMAGE_FORMAT = 30003
)