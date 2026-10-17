package adapters

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

// imageHash 计算图片的平均感知哈希（8x8 灰度均值，返回 16 位十六进制串）。
// 移植自 autosignin tjupt 的 imagehash.average_hash 语义。
func imageHash(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return "", fmt.Errorf("empty image")
	}
	// 采样为 8x8 灰度
	const size = 8
	gray := make([]uint8, size*size)
	total := 0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := x * w / size
			sy := y * h / size
			r, g, b, _ := img.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
			gray[y*size+x] = uint8((r*299 + g*587 + b*114) / 1000 >> 8)
			total += int(gray[y*size+x])
		}
	}
	mean := total / (size * size)
	var hash uint64
	for i, v := range gray {
		if int(v) >= mean {
			hash |= 1 << (63 - uint(i))
		}
	}
	return fmt.Sprintf("%016x", hash), nil
}

// compareHashes 计算两个 64 位哈希的相似度（1 - 汉明距离/64）。
func compareHashes(a, b string) float64 {
	var ia, ib uint64
	if _, err := fmt.Sscanf(a, "%016x", &ia); err != nil {
		return 0
	}
	if _, err := fmt.Sscanf(b, "%016x", &ib); err != nil {
		return 0
	}
	x := ia ^ ib
	dist := 0
	for x != 0 {
		x &= x - 1
		dist++
	}
	return 1 - float64(dist)/64
}
