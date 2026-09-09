package server

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTo16(t *testing.T) {
	cases := []struct{ in, want int }{
		{574, 576}, {733, 736}, {256, 256}, {255, 256}, {512, 512}, {100, 256},
	}
	for _, c := range cases {
		if got := roundTo16(c.in); got != c.want {
			t.Errorf("roundTo16(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPrepareInpaintAutoResize(t *testing.T) {
	dir := t.TempDir()
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		t.Fatal(err)
	}

	// 574x733 输入图 + 同尺寸遮罩（非 16 倍数）
	in := image.NewRGBA(image.Rect(0, 0, 574, 733))
	in.Set(0, 0, color.White)
	inPath := filepath.Join(uploads, "in.png")
	writePNG(t, inPath, in)

	mask := image.NewRGBA(image.Rect(0, 0, 574, 733))
	mask.Set(0, 0, color.White)
	maskPath := filepath.Join(uploads, "mask.png")
	writePNG(t, maskPath, mask)

	j := &Job{ID: "t1", Mode: "inpaint"}
	j.Params.InputImage = inPath
	j.Params.MaskImage = maskPath

	ow, oh, err := prepareInpaint(j, uploads)
	if err != nil {
		t.Fatalf("prepareInpaint: %v", err)
	}
	if ow != 574 || oh != 733 {
		t.Errorf("应返回原始尺寸 574x733，实际 %dx%d", ow, oh)
	}
	if j.Params.Width != 576 || j.Params.Height != 736 {
		t.Errorf("目标尺寸应 576x736，实际 %dx%d", j.Params.Width, j.Params.Height)
	}
	// 574x733 非法 → 输入与遮罩都应替换为 576x736 的预处理产物
	if j.Params.InputImage == inPath {
		t.Errorf("非法尺寸应替换输入路径为预处理产物")
	}
	if _, err := os.Stat(j.Params.InputImage); err != nil {
		t.Errorf("预处理输入不存在: %v", err)
	}
	if w, h, err := decodeImageDims(j.Params.InputImage); err != nil || w != 576 || h != 736 {
		t.Errorf("预处理输入尺寸应 576x736，实际 %dx%d (%v)", w, h, err)
	}
	if w, h, err := decodeImageDims(j.Params.MaskImage); err != nil || w != 576 || h != 736 {
		t.Errorf("预处理遮罩尺寸应 576x736，实际 %dx%d (%v)", w, h, err)
	}

	// 合法尺寸（512x512）不替换
	in2 := filepath.Join(uploads, "in2.png")
	img2 := image.NewRGBA(image.Rect(0, 0, 512, 512))
	f2, _ := os.Create(in2)
	png.Encode(f2, img2)
	f2.Close()
	j.Params.InputImage = in2
	j.Params.MaskImage = ""
	if _, _, err := prepareInpaint(j, uploads); err != nil {
		t.Fatalf("合法尺寸 prepareInpaint: %v", err)
	}
	if j.Params.InputImage != in2 {
		t.Errorf("合法尺寸不应替换输入路径")
	}
	if j.Params.Width != 512 || j.Params.Height != 512 {
		t.Errorf("合法尺寸应保持 512x512，实际 %dx%d", j.Params.Width, j.Params.Height)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
