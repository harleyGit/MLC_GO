/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 19:35:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 19:41:53
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/qrcode/qrcode.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
 // 二维码库安装: go get -u github.com/boombuler/barcode
package qrcode

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/file"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/util"
	"image/jpeg"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

type QrCode struct {
	URL    string
	Width  int
	Height int
	Ext    string
	Level  qr.ErrorCorrectionLevel
	Mode   qr.Encoding
}

const (
	EXT_JPG = ".jpg"
)

func NewQrCode(url string, width, height int, level qr.ErrorCorrectionLevel, mode qr.Encoding) *QrCode {
	return &QrCode{
		URL:    url,
		Width:  width,
		Height: height,
		Level:  level,
		Mode:   mode,
		Ext:    EXT_JPG,
	}
}

func GetQrCodePath() string {
	return setting.AppSetting.QrCodeSavePath
}

func GetQrCodeFullPath() string {
	return setting.AppSetting.RuntimeRootPath + setting.AppSetting.QrCodeSavePath
}

func GetQrCodeFullUrl(name string) string {
	return setting.AppSetting.PrefixUrl + "/" + GetQrCodePath() + name
}

// GetQrCodeFileName get qr file name
func GetQrCodeFileName(value string) string {
	return util.EncodeMD5(value)
}

// GetQrCodeExt get qr file ext
func (q *QrCode) GetQrCodeExt() string {
	// 这是 QrCode 结构体的一个方法，用于获取二维码图片的扩展名（例如 "jpg"、"png" 等）
	return q.Ext
}

// Encode generate QR code
func (q *QrCode) Encode(path string) (string, string, error) {
	name := GetQrCodeFileName(q.URL) + q.GetQrCodeExt()
	src := path + name
	if file.CheckNotExist(src) == true {
		// 二维码生成：
		// 	使用 qr.Encode 方法对二维码进行编码，传入了三个参数：
		// 		q.URL: 这是二维码中需要包含的信息，通常为一个 URL 地址。
		//  	q.Level: 表示二维码的容错级别（错误纠正级别），不同级别允许二维码部分损坏但仍能被正确识别。
		//  	q.Mode: 表示二维码编码的模式，可能是数字、字母、二进制等模式。
		code, err := qr.Encode(q.URL, q.Level, q.Mode)
		if err != nil {
			return "", "", err
		}

		// 使用 barcode.Scale 方法将生成的二维码图像 code 按照指定的宽度和高度进行缩放
		code, err = barcode.Scale(code, q.Width, q.Height)
		if err != nil {
			return "", "", err
		}

		f, err := file.MustOpen(name, path)
		if err != nil {
			return "", "", err
		}
		defer f.Close()
		// 使用 jpeg.Encode 将二维码图像 code 以 JPEG 格式编码，并写入到目标 f 中
		err = jpeg.Encode(f, code, nil)
		if err != nil {
			return "", "", err
		}
	}

	return name, path, nil
}
