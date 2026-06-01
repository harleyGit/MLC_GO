package UserServicePackage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/color"
	"image/png"
	"math/rand"
	"testing"
)

func TestGenerateCaptchaImageDrawsVisibleCharacters(t *testing.T) {
	chars := []string{"春", "7", "好"}
	points := []ClickCaptchaPoint{
		{X: 50, Y: 50},
		{X: 140, Y: 50},
		{X: 230, Y: 50},
	}

	imageBase64, err := generateCaptchaImage(chars, points)
	if err != nil {
		t.Fatalf("generate captcha image: %v", err)
	}

	imageBytes, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		t.Fatalf("decode image base64: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	darkPixels := 0
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			pixel := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
			if pixel.R < 170 && pixel.G < 170 && pixel.B < 170 {
				darkPixels++
			}
		}
	}

	if darkPixels < 250 {
		t.Fatalf("expected visible character pixels, got %d dark pixels", darkPixels)
	}
}

func TestGenerateCaptchaImageUsesCharacterValue(t *testing.T) {
	points := []ClickCaptchaPoint{{X: 140, Y: 50}}

	rand.Seed(1)
	imageA, err := generateCaptchaImage([]string{"春"}, points)
	if err != nil {
		t.Fatalf("generate captcha image 春: %v", err)
	}

	rand.Seed(1)
	imageB, err := generateCaptchaImage([]string{"夏"}, points)
	if err != nil {
		t.Fatalf("generate captcha image 夏: %v", err)
	}

	if imageA == imageB {
		t.Fatal("expected different image output for different captcha characters")
	}
}

func TestParseCaptchaDataSupportsObjectAndLegacyEncodedString(t *testing.T) {
	// expected 模拟 Redis 中应该保存的验证码业务数据：
	// Chars 是前端需要按顺序点击的字符；Points 是后端私有坐标；Created 用于记录生成时间。
	expected := CaptchaData{
		Chars: []string{"春", "7"},
		Points: []ClickCaptchaPoint{
			{X: 50, Y: 40},
			{X: 120, Y: 60},
		},
		Created: 123456,
	}

	// 新格式：SetToRedisV2 直接接收 CaptchaData 结构体，因此 Redis value 是普通 JSON object。
	objectBytes, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal captcha data: %v", err)
	}

	parsedObject, err := parseCaptchaData(string(objectBytes))
	if err != nil {
		t.Fatalf("parse object captcha data: %v", err)
	}
	if parsedObject.Created != expected.Created || len(parsedObject.Chars) != len(expected.Chars) || len(parsedObject.Points) != len(expected.Points) {
		t.Fatalf("unexpected parsed object captcha data: %+v", parsedObject)
	}

	// 旧格式：历史实现先 Marshal 成 []byte，再转 string 传给 SetToRedisV2。
	// SetToRedisV2 内部再次 Marshal 后，Redis value 变成 JSON encoded string。
	// 这个用例确保 parseCaptchaData 能兼容 TTL 内尚未过期的旧验证码。
	legacyBytes, err := json.Marshal(string(objectBytes))
	if err != nil {
		t.Fatalf("marshal legacy captcha data: %v", err)
	}

	parsedLegacy, err := parseCaptchaData(string(legacyBytes))
	if err != nil {
		t.Fatalf("parse legacy captcha data: %v", err)
	}
	if parsedLegacy.Created != expected.Created || len(parsedLegacy.Chars) != len(expected.Chars) || len(parsedLegacy.Points) != len(expected.Points) {
		t.Fatalf("unexpected parsed legacy captcha data: %+v", parsedLegacy)
	}
}
