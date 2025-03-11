/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-11 18:14:58
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-11 19:02:56
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/file/file.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package file

import (
	"io/ioutil"
	// mime/multipart 包主要实现了 MIME 的 multipart 解析，主要适用于 HTTP 和常见浏览器生成的 multipart 主体
	// multipart 是一种HTTP 请求/响应的编码格式，常用于文件上传或包含多部分数据的 HTTP 请求。它的典型使用场景是 multipart/form-data，用于在 POST 请求中上传文件或表单数据。
	// RFC 2388 是 IETF（Internet Engineering Task Force）在 1998 年发布的标准文档，定义了 multipart/form-data 的格式和用法，主要用于 HTTP 文件上传和表单数据提交。
	
	/* multipart/form-data 是 Content-Type 之一，适用于 POST 请求，主要用于表单提交和文件上传
	关键点
		边界（boundary）

		multipart/form-data 使用 boundary 来分隔多个表单项（键值对或文件）。
		boundary 是一串唯一的字符串，用于标识不同的数据部分。
		Content-Disposition

		每个部分都会有 Content-Disposition 头，表明它是表单字段还是文件。
		Content-Type

		如果是文件，Content-Type 会指定文件的 MIME 类型（如 image/png）。
		普通文本字段没有 Content-Type，默认为 text/plain。

	HTTP POST请求举例：
		POST /upload HTTP/1.1
		Host: example.com
		Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryabc123

		------WebKitFormBoundaryabc123
		Content-Disposition: form-data; name="username"

		JohnDoe
		------WebKitFormBoundaryabc123
		Content-Disposition: form-data; name="file"; filename="example.png"
		Content-Type: image/png

		(binary file content)
		------WebKitFormBoundaryabc123--
	*/
	"mime/multipart"
	"os"
	"path"
)

/// 获取文件大小
func GetSize(f multipart.File) (int, error) {
	content, err := ioutil.ReadAll(f)

	return len(content), err
}
 
/// 获取文件后缀
func GetExt(fileName string) string {
	return path.Ext(fileName)
}

// 检查文件是否存在
func CheckNotExist(src string) bool {
	_, err := os.Stat(src)

	return os.IsNotExist(err)
}

// 检查文件权限
func CheckPermission(src string) bool {
	_, err := os.Stat(src)

	return os.IsPermission(err)
}

// 如果不存在则新建文件夹
func IsNotExistMkDir(src string) error {
	if notExist := CheckNotExist(src); notExist == true {
		if err := MkDir(src); err != nil {
			return err
		}
	}

	return nil
}

// 新建文件夹
func MkDir(src string) error {
	err := os.MkdirAll(src, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

// 打开文件
func Open(name string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	return f, nil
}