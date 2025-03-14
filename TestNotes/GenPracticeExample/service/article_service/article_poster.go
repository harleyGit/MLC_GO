/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 19:53:58
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 20:46:10
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/service/article_service/article_poster.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package article_service

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/file"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/qrcode"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"image"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"os"

	"github.com/golang/freetype"
)

type ArticlePoster struct {
	PosterName string
	*Article
	Qr *qrcode.QrCode
}

type Rect struct {
	Name string
	X0   int
	Y0   int
	X1   int
	Y1   int
}

type Pt struct {
	X int
	Y int
}

type ArticlePosterBg struct {
	Name string
	*ArticlePoster
	*Rect
	*Pt
}

type DrawText struct {
	JPG    draw.Image
	Merged *os.File

	Title string
	X0    int
	Y0    int
	Size0 float64

	SubTitle string
	X1       int
	Y1       int
	Size1    float64
}

func NewArticlePoster(posterName string, article *Article, qr *qrcode.QrCode) *ArticlePoster {
	return &ArticlePoster{
		PosterName: posterName,
		Article:    article,
		Qr:         qr,
	}
}


func GetPosterFlag() string {
	return "poster"
}

func NewArticlePosterBg(name string, ap *ArticlePoster, rect *Rect, pt *Pt) *ArticlePosterBg {
	return &ArticlePosterBg{
		Name:          name,
		ArticlePoster: ap,
		Rect:          rect,
		Pt:            pt,
	}
}

func (a *ArticlePoster) CheckMergedImage(path string) bool {
	if file.CheckNotExist(path+a.PosterName) == true {
		return false
	}

	return true
}

func (a *ArticlePoster) OpenMergedImage(path string) (*os.File, error) {
	f, err := file.MustOpen(a.PosterName, path)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (a *ArticlePosterBg) DrawPoster(d *DrawText, fontName string) error {
	fontSource := setting.AppSetting.RuntimeRootPath + setting.AppSetting.FontSavePath + fontName
	// 读取字体文件内容，获取字体数据（字节数组）。
	fontSourceBytes, err := ioutil.ReadFile(fontSource)
	if err != nil {
		return err
	}

	// 使用 freetype 库解析读取到的字体数据，得到 TrueType 字体对象，用于绘制文本。
	trueTypeFont, err := freetype.ParseFont(fontSourceBytes)
	if err != nil {
		return err
	}

	// 创建一个新的 freetype 绘图上下文，用于绘制文字
	fc := freetype.NewContext()
	// 设置每英寸点数（DPI），决定字体绘制的清晰度。
	fc.SetDPI(72)
	// 设置刚刚解析得到的字体。
	fc.SetFont(trueTypeFont)
	// 设置初始字体大小，这里使用 DrawText 结构体中的 Size0 字段。
	fc.SetFontSize(d.Size0)
	// 设置绘制区域为目标图像 d.JPG 的整个区域，防止文字绘制越界。
	fc.SetClip(d.JPG.Bounds())
	// 指定文字要绘制到哪个目标图像上，此处为海报图片。
	fc.SetDst(d.JPG)
	// 设置文字颜色为黑色
	fc.SetSrc(image.Black)
	

	// 定义一个起始绘制点，坐标由 DrawText 中的 X0 和 Y0 决定。
	pt := freetype.Pt(d.X0, d.Y0)
	// 在指定位置绘制标题文本 d.Title，并检查是否出错。
	_, err = fc.DrawString(d.Title, pt)
	if err != nil {
		return err
	}

	// 修改字体大小为副标题所需的尺寸（d.Size1）。
	fc.SetFontSize(d.Size1)
	// 在指定的新位置（由 d.X1、d.Y1 指定）绘制副标题 d.SubTitle。
	_, err = fc.DrawString(d.SubTitle, freetype.Pt(d.X1, d.Y1))
	if err != nil {
		return err
	}

	// 将最终绘制好的图像 d.JPG 以 JPEG 格式编码后输出到 d.Merged（通常是一个实现了 io.Writer 接口的文件或缓冲区）。
	err = jpeg.Encode(d.Merged, d.JPG, nil)
	if err != nil {
		return err
	}

	return nil
}


func (a *ArticlePosterBg) Generate() (string, string, error) {
	fullPath := qrcode.GetQrCodeFullPath()
	// 传入刚刚获取的路径，生成二维码
	fileName, path, err := a.Qr.Encode(fullPath)
	if err != nil {
		return "", "", err
	}

	if !a.CheckMergedImage(path) {
		mergedF, err := a.OpenMergedImage(path)
		if err != nil {
			return "", "", err
		}
		defer mergedF.Close()

		bgF, err := file.MustOpen(a.Name, path)
		if err != nil {
			return "", "", err
		}
		defer bgF.Close()

		qrF, err := file.MustOpen(fileName, path)
		if err != nil {
			return "", "", err
		}
		defer qrF.Close()

		// 从 bgF（通常是一个 io.Reader，比如文件句柄或内存流）解码 JPEG 格式的背景图片。
		// 得到的 bgImage 是一个 image.Image 对象，用于后续绘制到海报上。
		bgImage, err := jpeg.Decode(bgF)
		if err != nil {
			return "", "", err
		}
		qrImage, err := jpeg.Decode(qrF)
		if err != nil {
			return "", "", err
		}

		// 创建一个新的 RGBA 图像作为海报的画布。
		// image.Rect(a.Rect.X0, a.Rect.Y0, a.Rect.X1, a.Rect.Y1) 定义了海报的矩形区域，通常根据海报设计要求来设置海报的起始和结束坐标
		// draw.Draw(jpg, jpg.Bounds(), bgImage, bgImage.Bounds().Min, draw.Over)将解码后的背景图片绘制到新建的 jpg 画布上。
		// 		第一个参数：目标图像，即海报画布。
		// 		第二个参数：绘制区域（此处为整个画布）。
		// 		第三个参数：源图像，即背景图片。
		// 		第四个参数：源图像的起始绘制位置（通常取其最小坐标）。
		// 		第五个参数：混合模式，这里使用 draw.Over 表示直接覆盖。
		jpg := image.NewRGBA(image.Rect(a.Rect.X0, a.Rect.Y0, a.Rect.X1, a.Rect.Y1))

		draw.Draw(jpg, jpg.Bounds(), bgImage, bgImage.Bounds().Min, draw.Over)
		draw.Draw(jpg, jpg.Bounds(), qrImage, qrImage.Bounds().Min.Sub(image.Pt(a.Pt.X, a.Pt.Y)), draw.Over)

		err = a.DrawPoster(&DrawText{
			JPG:    jpg,
			Merged: mergedF,

			Title: "Golang 基础语法.md",
			X0:    80,
			Y0:    160,
			Size0: 42,

			SubTitle: "---黄刚☘️",
			X1:       320,
			Y1:       220,
			Size1:    36,
		}, "msyhbd.ttc")

		if err != nil {
			return "", "", err
		}
	}

	return fileName, path, nil
}
