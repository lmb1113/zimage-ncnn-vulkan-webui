package server

import (
	"fmt"
	"strconv"
	"strings"
)

// ModeSpec 描述一种生成模式需要哪些输入、以及它对尺寸的额外约束。
// 不同引擎支持的模式与约束不同（如局部重绘需要遮罩、图像编辑要求 32 倍数），
// 由引擎档案声明，界面与参数构建都据此渲染/生成。
type ModeSpec struct {
	Key      string `json:"key"`      // txt2img / img2img / inpaint / outpaint / controlnet / tile / edit
	Label    string `json:"label"`    // 界面显示名
	NeedsRef     bool `json:"needsRef"`     // 需要输入/参考图（-i）
	MultiRef     bool `json:"multiRef"`     // 参考图可多张（-i 重复）
	NeedsMask    bool `json:"needsMask"`    // 需要遮罩（-k）
	NeedsMargin  bool `json:"needsMargin"`  // 需要扩展边距（-x）
	NeedsControl bool `json:"needsControl"` // 需要控制图（-c）
	Tile         bool `json:"tile"`         // Tile 放大（-t）
	SizeUnit     int  `json:"sizeUnit"`     // 该模式的尺寸对齐粒度（0 = 跟随引擎）
}

// EngineProfile 描述一个 ncnn 推理引擎的能力与参数语义。
// 新增模型只需追加一条档案，无需改动界面与任务流程。
type EngineProfile struct {
	ID        string `json:"id"`        // 唯一标识
	Name      string `json:"name"`      // 界面显示名
	ExePath   string `json:"exePath"`   // 引擎可执行文件
	ModelPath string `json:"modelPath"` // -m 模型路径
	Modes     []ModeSpec `json:"modes"` // 该引擎支持的模式

	SizeUnit     int    `json:"sizeUnit"`     // 默认尺寸对齐粒度（16 / 32）
	DefaultSteps int    `json:"defaultSteps"` // 步数为 0（自动）时使用的默认步数
	MaxRefs      int    `json:"maxRefs"`      // -i 最多可重复次数（1 = 仅单图）
	GuidanceRole string `json:"guidanceRole"` // -w 的语义：controlScale（控制强度）| cfg（引导系数）
	OutputRGBA   bool   `json:"outputRGBA"`   // 输出带透明通道（RGBA）
	Builtin      bool   `json:"builtin"`      // 内置档案（不可删除，可改路径）

	// 运行时探测结果（由 Inspect 填充，便于界面提示路径是否有效）
	ExeFound   bool `json:"exeFound,omitempty"`
	ModelFound bool `json:"modelFound,omitempty"`
}

// Mode 返回指定模式的规范，不支持则返回 nil。
func (e *EngineProfile) Mode(key string) *ModeSpec {
	for i := range e.Modes {
		if e.Modes[i].Key == key {
			return &e.Modes[i]
		}
	}
	return nil
}

// SupportsMode 判断引擎是否支持某模式。
func (e *EngineProfile) SupportsMode(key string) bool {
	return e.Mode(key) != nil
}

// SizeUnitFor 返回某模式实际使用的尺寸对齐粒度。
func (e *EngineProfile) SizeUnitFor(mode string) int {
	if m := e.Mode(mode); m != nil && m.SizeUnit > 0 {
		return m.SizeUnit
	}
	if e.SizeUnit > 0 {
		return e.SizeUnit
	}
	return 16
}

// BuildArgs 按引擎档案把任务参数翻译成该引擎的命令行。
func (e *EngineProfile) BuildArgs(j *Job, outPath string) []string {
	p := j.Params
	a := []string{}

	prompt := strings.TrimSpace(p.Prompt)
	if prompt == "" {
		prompt = "rand"
	}
	a = append(a, "-p", prompt)

	if strings.TrimSpace(p.Negative) != "" {
		a = append(a, "-n", strings.TrimSpace(p.Negative))
	}
	a = append(a, "-o", outPath)
	// 扩图：引擎按「原图 + -x 边距」自行推导画布，
	// 传入与扩展后尺寸不符的 -s 会导致引擎崩溃，因此该模式跳过 -s。
	if p.Width > 0 && p.Height > 0 && j.Mode != "outpaint" {
		a = append(a, "-s", fmt.Sprintf("%d,%d", p.Width, p.Height))
	}
	if p.Steps > 0 {
		a = append(a, "-l", strconv.Itoa(p.Steps))
	}
	if p.Seed >= 0 {
		a = append(a, "-r", strconv.FormatInt(p.Seed, 10))
	}
	if strings.TrimSpace(p.ModelPath) != "" {
		a = append(a, "-m", strings.TrimSpace(p.ModelPath))
	}
	if p.GPUID != nil && *p.GPUID != -2 {
		a = append(a, "-g", strconv.Itoa(*p.GPUID))
	}
	if p.Batch > 1 {
		a = append(a, "-b", strconv.Itoa(p.Batch))
	}
	// -w 语义随引擎不同：控制强度 或 引导系数
	if w := e.guidanceValue(p); w > 0 {
		a = append(a, "-w", strconv.FormatFloat(w, 'f', 2, 64))
	}

	spec := e.Mode(j.Mode)
	if spec == nil {
		return a
	}

	// 参考/输入图：支持多图的引擎按列表重复 -i
	if spec.NeedsRef {
		refs := p.RefImages
		if len(refs) == 0 && strings.TrimSpace(p.InputImage) != "" {
			refs = []string{p.InputImage}
		}
		limit := e.MaxRefs
		if limit < 1 {
			limit = 1
		}
		for i, r := range refs {
			if i >= limit {
				break
			}
			if strings.TrimSpace(r) != "" {
				a = append(a, "-i", strings.TrimSpace(r))
			}
		}
	}
	if spec.NeedsMask && strings.TrimSpace(p.MaskImage) != "" {
		a = append(a, "-k", strings.TrimSpace(p.MaskImage))
	}
	if spec.NeedsMargin && strings.TrimSpace(p.Outpaint) != "" {
		a = append(a, "-x", strings.TrimSpace(p.Outpaint))
	}
	if spec.NeedsControl && strings.TrimSpace(p.ControlImage) != "" {
		a = append(a, "-c", strings.TrimSpace(p.ControlImage))
	}
	if spec.Tile {
		a = append(a, "-t")
	}
	return a
}

// guidanceValue 按引擎语义取出 -w 的值（0 = 不传）。
func (e *EngineProfile) guidanceValue(p Params) float64 {
	switch e.GuidanceRole {
	case "cfg":
		if p.Guidance > 0 {
			return p.Guidance
		}
		return 0
	default: // controlScale
		if p.ControlScale > 0 {
			return p.ControlScale
		}
		return 0
	}
}

// builtinProfiles 返回内置的引擎档案。路径留空时由配置层按常见位置探测。
func builtinProfiles() []EngineProfile {
	return []EngineProfile{
		{
			ID:        "z-image-turbo",
			Name:      "Z-Image Turbo",
			ModelPath: "z-image-turbo",
			Modes: []ModeSpec{
				{Key: "txt2img", Label: "文生图"},
				{Key: "img2img", Label: "图生图", NeedsRef: true},
				{Key: "inpaint", Label: "局部重绘", NeedsRef: true, NeedsMask: true},
				{Key: "outpaint", Label: "画布扩图", NeedsRef: true, NeedsMargin: true},
				{Key: "controlnet", Label: "控制生成", NeedsControl: true},
				{Key: "tile", Label: "图片放大", NeedsControl: true, Tile: true},
			},
			SizeUnit:     16,
			DefaultSteps: 9,
			MaxRefs:      1,
			GuidanceRole: "controlScale",
			Builtin:      true,
		},
		{
			ID:        "qwen-image-21",
			Name:      "Qwen-Image 2.1",
			ModelPath: "", // 由配置层探测填充（如 F:\aiimage\qwen-image-ncnn\qwenimage21）
			Modes: []ModeSpec{
				{Key: "txt2img", Label: "文生图"},
				{Key: "edit", Label: "图像编辑", NeedsRef: true, MultiRef: true, SizeUnit: 32},
			},
			SizeUnit:     16,
			DefaultSteps: 40,
			MaxRefs:      10,
			GuidanceRole: "cfg",
			OutputRGBA:   true,
			Builtin:      true,
		},
	}
}
