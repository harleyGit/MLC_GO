/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-31
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-31
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_captcha_image.go
 * @Description: 点选验证码图片生成工具
 */
package UserServicePackage

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/fonts/NotoSansSC-Regular.ttf
var captchaFontBytes []byte

var (
	captchaFontOnce sync.Once
	captchaFont     *opentype.Font
	captchaFontErr  error
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

// generateCaptchaID 生成唯一的验证码 ID。
func generateCaptchaID() string {
	return fmt.Sprintf("%d%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}

// generateRandomChars 生成指定数量的随机字符（数字、字母、汉字混合）。
func generateRandomChars(count int) []string {
	// 字符池：数字、大写字母、小写字母、常用汉字。
	digits := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	letters := []string{
		"A", "B", "C", "D", "E", "F", "G", "H", "J", "K", "L", "M",
		"N", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
		"a", "b", "c", "d", "e", "f", "g", "h", "j", "k", "m", "n",
		"p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	}
	chinese := []string{
		"你", "好", "世", "界", "春", "夏", "秋", "冬", "风", "雨",
		"雪", "花", "月", "星", "日", "山", "水", "火", "木", "金",
	}

	allChars := append(append(digits, letters...), chinese...)
	result := make([]string, count)

	for i := 0; i < count; i++ {
		result[i] = allChars[rand.Intn(len(allChars))]
	}

	return result
}

// generateRandomPoints 生成指定数量的随机坐标点。
func generateRandomPoints(count, width, height int) []ClickCaptchaPoint {
	points := make([]ClickCaptchaPoint, count)
	padding := 30    // 边距，避免字符太靠近边缘
	minSpacing := 50 // 字符之间的最小间距

	for i := 0; i < count; i++ {
		valid := false
		for !valid {
			points[i] = ClickCaptchaPoint{
				X: padding + rand.Intn(width-2*padding),
				Y: padding + rand.Intn(height-2*padding),
			}
			valid = true
			// 检查与已有点位的距离
			for j := 0; j < i; j++ {
				dx := points[i].X - points[j].X
				dy := points[i].Y - points[j].Y
				if math.Sqrt(float64(dx*dx+dy*dy)) < float64(minSpacing) {
					valid = false
					break
				}
			}
		}
	}

	return points
}

// generateCaptchaImage 生成验证码图片并返回 Base64 编码。
func generateCaptchaImage(chars []string, points []ClickCaptchaPoint) (string, error) {
	width := 280
	height := 100
	face, err := newCaptchaFontFace()
	if err != nil {
		return "", err
	}
	if closer, ok := face.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	// 创建 RGBA 图片
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 填充背景色（浅灰色）
	bgColor := color.RGBA{R: 245, G: 245, B: 245, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// 添加干扰线
	drawInterferenceLines(img, width, height)

	// 添加干扰点
	drawInterferenceDots(img, width, height)

	// 绘制字符
	for i, char := range chars {
		if i < len(points) {
			drawChar(img, face, char, points[i].X, points[i].Y)
		}
	}

	// 编码为 PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	// 转换为 Base64
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func newCaptchaFontFace() (font.Face, error) {
	captchaFontOnce.Do(func() {
		captchaFont, captchaFontErr = opentype.Parse(captchaFontBytes)
	})
	if captchaFontErr != nil {
		return nil, fmt.Errorf("parse captcha font: %w", captchaFontErr)
	}

	face, err := opentype.NewFace(captchaFont, &opentype.FaceOptions{
		Size:    26,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create captcha font face: %w", err)
	}

	return face, nil
}

// drawInterferenceLines 绘制干扰线。
func drawInterferenceLines(img *image.RGBA, width, height int) {
	lineColors := []color.RGBA{
		{R: 200, G: 200, B: 200, A: 255},
		{R: 180, G: 180, B: 180, A: 255},
		{R: 160, G: 160, B: 160, A: 255},
	}

	for i := 0; i < 5; i++ {
		c := lineColors[rand.Intn(len(lineColors))]
		x1, y1 := rand.Intn(width), rand.Intn(height)
		x2, y2 := rand.Intn(width), rand.Intn(height)

		// 简单的线段绘制
		steps := int(math.Max(math.Abs(float64(x2-x1)), math.Abs(float64(y2-y1))))
		if steps == 0 {
			continue
		}

		for step := 0; step <= steps; step++ {
			t := float64(step) / float64(steps)
			x := int(float64(x1) + t*float64(x2-x1))
			y := int(float64(y1) + t*float64(y2-y1))

			if x >= 0 && x < width && y >= 0 && y < height {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

// drawInterferenceDots 绘制干扰点。
func drawInterferenceDots(img *image.RGBA, width, height int) {
	for i := 0; i < 50; i++ {
		x := rand.Intn(width)
		y := rand.Intn(height)
		c := color.RGBA{
			R: uint8(180 + rand.Intn(75)),
			G: uint8(180 + rand.Intn(75)),
			B: uint8(180 + rand.Intn(75)),
			A: 255,
		}
		img.SetRGBA(x, y, c)
	}
}

// drawChar 在指定位置绘制字符。
func drawChar(img *image.RGBA, face font.Face, char string, x, y int) {
	// 字符颜色（随机深色）
	charColors := []color.RGBA{
		{R: 50, G: 50, B: 150, A: 255},
		{R: 150, G: 50, B: 50, A: 255},
		{R: 50, G: 120, B: 50, A: 255},
		{R: 100, G: 50, B: 120, A: 255},
		{R: 50, G: 100, B: 150, A: 255},
	}
	c := charColors[rand.Intn(len(charColors))]

	metrics := face.Metrics()
	textWidth := font.MeasureString(face, char).Ceil()
	textHeight := (metrics.Ascent + metrics.Descent).Ceil()
	if textWidth <= 0 || textHeight <= 0 {
		return
	}

	mask := image.NewAlpha(image.Rect(0, 0, textWidth, textHeight))
	d := &font.Drawer{
		Dst:  mask,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(0, metrics.Ascent.Ceil()),
	}
	d.DrawString(char)

	startX := x - textWidth/2
	startY := y - textHeight/2
	bounds := img.Bounds()

	for py := 0; py < textHeight; py++ {
		for px := 0; px < textWidth; px++ {
			if mask.AlphaAt(px, py).A == 0 {
				continue
			}
			dx := startX + px
			dy := startY + py
			if dx >= bounds.Min.X && dx < bounds.Max.X && dy >= bounds.Min.Y && dy < bounds.Max.Y {
				img.SetRGBA(dx, dy, c)
			}
		}
	}
}
