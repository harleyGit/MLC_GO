package UserServicePackage

import (
	"bytes"
	"encoding/base64"
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
