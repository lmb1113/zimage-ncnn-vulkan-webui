package server

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"sync"
)

var (
	iconOnce  sync.Once
	iconBytes []byte
)

// TrayIcon 生成托盘图标（内嵌 PNG 的 ICO 格式，Windows Vista+ 支持）。
func TrayIcon() []byte {
	iconOnce.Do(func() {
		img := drawLogo(32)
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		iconBytes = wrapICO(buf.Bytes(), 32, 32)
	})
	return iconBytes
}

// drawLogo 绘制圆角深色底 + 白色圆点与山形折线（与前端 Logo 一致）。
func drawLogo(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{0x14, 0x16, 0x1A, 0xFF}
	fg := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}

	radius := float64(size) * 0.22
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if inRoundedRect(float64(x)+0.5, float64(y)+0.5, 0, 0, float64(size), float64(size), radius) {
				img.Set(x, y, bg)
			}
		}
	}

	// 圆点：左上区域
	cx, cy, r := float64(size)*0.34, float64(size)*0.33, float64(size)*0.095
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, fg)
			}
		}
	}

	// 山形折线
	thick := float64(size) * 0.075
	p1 := pt{0.12 * float64(size), 0.79 * float64(size)}
	p2 := pt{0.34 * float64(size), 0.58 * float64(size)}
	p3 := pt{0.50 * float64(size), 0.74 * float64(size)}
	p4 := pt{0.63 * float64(size), 0.63 * float64(size)}
	p5 := pt{0.88 * float64(size), 0.83 * float64(size)}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			d := distToSeg(px, py, p1, p2)
			d = minFloat(d, distToSeg(px, py, p2, p3))
			d = minFloat(d, distToSeg(px, py, p3, p4))
			d = minFloat(d, distToSeg(px, py, p4, p5))
			if d <= thick/2 {
				img.Set(x, y, fg)
			}
		}
	}
	return img
}

type pt struct{ x, y float64 }

func inRoundedRect(px, py, x0, y0, w, h, r float64) bool {
	if px < x0 || px > x0+w || py < y0 || py > y0+h {
		return false
	}
	cx := maxFloat(x0+r, minFloat(px, x0+w-r))
	cy := maxFloat(y0+r, minFloat(py, y0+h-r))
	dx, dy := px-cx, py-cy
	return dx*dx+dy*dy <= r*r || (px >= x0+r && px <= x0+w-r) || (py >= y0+r && py <= y0+h-r)
}

func distToSeg(px, py float64, a, b pt) float64 {
	vx, vy := b.x-a.x, b.y-a.y
	wx, wy := px-a.x, py-a.y
	l2 := vx*vx + vy*vy
	if l2 == 0 {
		dx, dy := px-a.x, py-a.y
		return sqrtF(dx*dx + dy*dy)
	}
	t := (wx*vx + wy*vy) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	dx, dy := px-(a.x+t*vx), py-(a.y+t*vy)
	return sqrtF(dx*dx + dy*dy)
}

func sqrtF(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 24; i++ {
		x = (x + v/x) / 2
	}
	return x
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// wrapICO 把 PNG 打包成单图标的 ICO 容器。
func wrapICO(pngData []byte, w, h int) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00}) // ICONDIR: reserved, type=icon, count=1
	buf.WriteByte(byte(w % 256))                          // 0 表示 256
	buf.WriteByte(byte(h % 256))
	buf.WriteByte(0x00) // 调色板色数
	buf.WriteByte(0x00) // 保留
	buf.Write([]byte{0x01, 0x00})
	buf.Write([]byte{0x20, 0x00}) // 32 bpp
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(pngData)))
	buf.Write(size[:])
	var offset [4]byte
	binary.LittleEndian.PutUint32(offset[:], 22) // 6 + 16
	buf.Write(offset[:])
	buf.Write(pngData)
	return buf.Bytes()
}
