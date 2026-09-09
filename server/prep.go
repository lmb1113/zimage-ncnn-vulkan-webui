package server

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// 局部重绘的引擎约束（实测）：
//   1. -s 必须与输入图尺寸完全一致
//   2. 宽高必须为 16 的倍数
// 行业通行做法（A1111/ComfyUI 等）：服务端把输入与遮罩自动缩放到最近的合法尺寸。
// 这里在任务执行前统一处理，UI 侧无需关心限制。

// roundTo16 四舍五入到最近的 16 倍数，最小 256（过小尺寸重绘无意义）。
func roundTo16(v int) int {
	if v < 256 {
		return 256
	}
	return int(math.Round(float64(v)/16)) * 16
}

// decodeImageDims 读取图片尺寸（仅解析文件头）。
func decodeImageDims(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("无效的图片尺寸 %dx%d", cfg.Width, cfg.Height)
	}
	return cfg.Width, cfg.Height, nil
}

// resizeImageFile 把图片等比缩放到指定尺寸并写 PNG。
// mask=true 时用最近邻（保留遮罩硬边），否则用 CatmullRom（输入图保质量）。
func resizeImageFile(srcPath, dstPath string, w, h int, mask bool) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return err
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if mask {
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	}

	out, err := os.CreateTemp(filepath.Dir(dstPath), "prep-*")
	if err != nil {
		return err
	}
	tmpPath := out.Name()
	if err := png.Encode(out, dst); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	out.Close()
	return os.Rename(tmpPath, dstPath)
}

// prepareInpaint 局部重绘前置处理：
// 自动把输入图与遮罩缩放到「最近的 16 倍数」尺寸，并让 -s 与之对齐。
// 输入/遮罩尺寸不一致时，以输入图为准同步缩放遮罩。
func prepareInpaint(j *Job, uploadsDir string) (int, int, error) {
	p := &j.Params
	if p.InputImage == "" {
		return 0, 0, nil
	}

	w, h, err := decodeImageDims(p.InputImage)
	if err != nil {
		return 0, 0, fmt.Errorf("无法读取输入图: %w", err)
	}
	tw, th := roundTo16(w), roundTo16(h)

	// 输入遮罩：存在则以输入图目标尺寸为准（遮罩尺寸不符也会被引擎拒绝）
	maskPath := ""
	maskOK := false
	if p.MaskImage != "" {
		if _, err := os.Stat(p.MaskImage); err == nil {
			maskPath = p.MaskImage
			maskOK = true
		}
	}

	needResize := tw != w || th != h
	needMaskResize := maskOK && maskPath != "" && func() bool {
		mw, mh, err := decodeImageDims(maskPath)
		return err != nil || mw != tw || mh != th
	}()
	if !needResize && !(maskOK && needMaskResize) && !(maskOK && (tw != w || th != h)) {
		// 尺寸已合法：仅确保 -s 与输入一致
		p.Width, p.Height = w, h
		return w, h, nil
	}

	// 输入图缩放（尺寸不合法时）
	if needResize {
		dst := filepath.Join(uploadsDir, fmt.Sprintf("pre_%s_input.png", j.ID))
		if err := resizeImageFile(p.InputImage, dst, tw, th, false); err != nil {
			return w, h, fmt.Errorf("输入图缩放失败: %w", err)
		}
		p.InputImage = dst
	}

	// 遮罩缩放：与输入图最终尺寸对齐（可能跟随 needResize 一起变）
	if maskOK && maskPath != "" {
		dst := filepath.Join(uploadsDir, fmt.Sprintf("pre_%s_mask.png", j.ID))
		if err := resizeImageFile(maskPath, dst, tw, th, true); err != nil {
			return w, h, fmt.Errorf("遮罩缩放失败: %w", err)
		}
		p.MaskImage = dst
	}

	p.Width, p.Height = tw, th
	return w, h, nil
}
