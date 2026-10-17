package adapters

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngOf(px [][]uint8) []byte {
	w, h := len(px[0]), len(px)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := px[y][x]
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestImageHashSimilar(t *testing.T) {
	black := make([][]uint8, 8)
	for i := range black {
		black[i] = make([]uint8, 8)
	}
	// 同一图像必须 1.0
	h1, err := imageHash(pngOf(black))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := imageHash(pngOf(black))
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || compareHashes(h1, h2) != 1.0 {
		t.Fatalf("identical images must match exactly, %s vs %s", h1, h2)
	}
	// 上下半区反差：平均哈希约一半位翻转
	half := make([][]uint8, 8)
	for i := range half {
		half[i] = make([]uint8, 8)
		for j := range half[i] {
			if i < 4 {
				half[i][j] = 255
			}
		}
	}
	h3, err := imageHash(pngOf(half))
	if err != nil {
		t.Fatal(err)
	}
	if sim := compareHashes(h1, h3); sim <= 0.2 || sim >= 0.8 {
		t.Fatalf("half-difference similarity = %f", sim)
	}
	// 条纹与纯色区分明显
	stripes := make([][]uint8, 8)
	for i := range stripes {
		stripes[i] = make([]uint8, 8)
		for j := range stripes[i] {
			if (i+j)%2 == 0 {
				stripes[i][j] = 255
			}
		}
	}
	h4, err := imageHash(pngOf(stripes))
	if err != nil {
		t.Fatal(err)
	}
	if sim := compareHashes(h1, h4); sim > 0.5 {
		t.Fatalf("different images similarity = %f", sim)
	}
}
