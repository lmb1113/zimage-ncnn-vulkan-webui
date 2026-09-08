package server

import (
	"strings"
	"testing"
)

// ---------- BuildArgs：各模式命令行翻译 ----------

func join(args []string) string { return strings.Join(args, " ") }

func TestBuildArgsTxt2Img(t *testing.T) {
	j := &Job{Mode: "txt2img", Params: Params{
		Prompt: "a cat", Width: 512, Height: 512, Steps: 4, Seed: 42, Batch: 1,
	}}
	got := join(BuildArgs(j, "out.png"))
	for _, want := range []string{"-p a cat", "-o out.png", "-s 512,512", "-l 4", "-r 42"} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少参数 %q，实际: %s", want, got)
		}
	}
	// 默认值不应传多余参数
	if strings.Contains(got, " -b ") || strings.HasSuffix(got, " -b") {
		t.Errorf("batch=1 不应传 -b: %s", got)
	}
	if strings.Contains(got, " -g ") || strings.HasSuffix(got, " -g") {
		t.Errorf("自动 GPU 不应传 -g: %s", got)
	}
}

func TestBuildArgsEmptyPromptBecomesRand(t *testing.T) {
	j := &Job{Mode: "txt2img", Params: Params{Prompt: "   "}}
	args := BuildArgs(j, "o.png")
	if len(args) < 2 || args[0] != "-p" || args[1] != "rand" {
		t.Errorf("空提示词应回退为 rand，实际: %v", args)
	}
}

func TestBuildArgsNegativePrompt(t *testing.T) {
	j := &Job{Mode: "txt2img", Params: Params{Prompt: "x", Negative: "blurry"}}
	got := join(BuildArgs(j, "o.png"))
	if !strings.Contains(got, "-n blurry") {
		t.Errorf("缺少 -n: %s", got)
	}
}

func TestBuildArgsInpaint(t *testing.T) {
	j := &Job{Mode: "inpaint", Params: Params{
		Prompt: "x", InputImage: "in.png", MaskImage: "mask.png",
	}}
	got := join(BuildArgs(j, "o.png"))
	if !strings.Contains(got, "-i in.png") || !strings.Contains(got, "-k mask.png") {
		t.Errorf("重绘缺少 -i/-k: %s", got)
	}
}

func TestBuildArgsOutpaint(t *testing.T) {
	j := &Job{Mode: "outpaint", Params: Params{
		Prompt: "x", InputImage: "in.png", Outpaint: "128,128,128,128",
	}}
	got := join(BuildArgs(j, "o.png"))
	if !strings.Contains(got, "-i in.png") || !strings.Contains(got, "-x 128,128,128,128") {
		t.Errorf("扩图缺少 -i/-x: %s", got)
	}
}

func TestBuildArgsControlNet(t *testing.T) {
	j := &Job{Mode: "controlnet", Params: Params{
		Prompt: "x", ControlImage: "pose.png", ControlScale: 0.8,
	}}
	got := join(BuildArgs(j, "o.png"))
	for _, want := range []string{"-c pose.png", "-w 0.80"} {
		if !strings.Contains(got, want) {
			t.Errorf("ControlNet 缺少 %q: %s", want, got)
		}
	}
	if strings.Contains(got, " -t") {
		t.Errorf("ControlNet 不应带 -t: %s", got)
	}
}

func TestBuildArgsTile(t *testing.T) {
	j := &Job{Mode: "tile", Params: Params{
		Prompt: "x", ControlImage: "lo.png", ControlScale: 1.0,
	}}
	got := join(BuildArgs(j, "o.png"))
	if !strings.Contains(got, " -t") {
		t.Errorf("Tile 模式应带 -t: %s", got)
	}
}

func TestBuildArgsGPUID(t *testing.T) {
	g := -1
	j := &Job{Mode: "txt2img", Params: Params{Prompt: "x", GPUID: &g}}
	got := join(BuildArgs(j, "o.png"))
	if !strings.Contains(got, "-g -1") {
		t.Errorf("CPU 模式应传 -g -1: %s", got)
	}
}

// ---------- randSuffix：修复过的负索引 panic 回归测试 ----------

func TestRandSuffixValid(t *testing.T) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		s := randSuffix(6)
		if len(s) != 6 {
			t.Fatalf("长度错误: %q", s)
		}
		for _, c := range s {
			if !strings.ContainsRune(chars, c) {
				t.Fatalf("非法字符 %q in %q", c, s)
			}
		}
		seen[s] = true
	}
	// 200 次几乎不应重复
	if len(seen) < 180 {
		t.Errorf("随机性不足：200 次仅 %d 种取值", len(seen))
	}
}

// ---------- progTracker：进度解析 ----------

func TestProgTrackerSingleImage(t *testing.T) {
	tr := newProgTracker(1)
	cases := []struct {
		line string
		want int
	}{
		{"step 1/4 done", 25},
		{"step 2/4 done", 50},
		{"step 4/4 done", 100},
		{"vae done", 100},
	}
	for _, c := range cases {
		p, ok := tr.parseProgress(c.line)
		if !ok {
			t.Errorf("%q 应被识别为进度行", c.line)
		}
		if p != c.want {
			t.Errorf("%q: 期望 %d%%，实际 %d%%", c.line, c.want, p)
		}
	}
}

func TestProgTrackerBatch(t *testing.T) {
	tr := newProgTracker(2)
	tr.parseProgress("step 4/4 done") // 第一张完成 → 50%
	if p, _ := tr.parseProgress("step 4/4 done"); p != 50 {
		t.Fatalf("第一张完成应累计到 50%%，实际 %d%%", p)
	}
	p, _ := tr.parseProgress("step 1/4 done") // 第二张开始
	if p <= 50 {
		t.Errorf("第二张开始应超过 50%%，实际 %d%%", p)
	}
	p, _ = tr.parseProgress("vae done")
	if p != 100 {
		t.Errorf("全部完成应为 100%%，实际 %d%%", p)
	}
}

func TestProgTrackerIgnoresNoise(t *testing.T) {
	tr := newProgTracker(1)
	for _, line := range []string{
		"[0 AMD Radeon RX 7900 XT]  queueC=1[8]",
		"prompt = a cat",
		"",
	} {
		if _, ok := tr.parseProgress(line); ok {
			t.Errorf("%q 不应被识别为进度行", line)
		}
	}
}
