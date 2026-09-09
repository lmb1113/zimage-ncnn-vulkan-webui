package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Config 是工作台的全局设置，保存在服务端可执行文件同级的 config.json。
type Config struct {
	ExePath   string `json:"exePath"`   // zimage-ncnn-vulkan 可执行文件
	WorkDir   string `json:"workDir"`   // 工作目录（模型文件夹 z-image-turbo 所在目录）
	OutputDir string `json:"outputDir"` // 生成图输出目录
	UploadDir string `json:"uploadDir"` // 输入图 / 遮罩 / 控制图上传目录
	ModelPath string `json:"modelPath"` // -m 参数，默认 z-image-turbo
	GPUID     int    `json:"gpuId"`     // -g 参数；-2 表示自动（不传 -g），-1 表示 CPU
	Port      int    `json:"port"`
}

const exeBaseName = "zimage-ncnn-vulkan"

// AppVersion 是工作台的版本号，随每次发版更新。
const AppVersion = "v0.3.6"

var (
	cfgMu   sync.RWMutex
	cfgData Config
	cfgFile string
)

// ExeCandidates 返回当前平台可能的可执行文件名。
func exeCandidates() []string {
	if runtimeIsWindows() {
		return []string{exeBaseName + ".exe", exeBaseName}
	}
	return []string{exeBaseName, exeBaseName + ".exe"}
}

func runtimeIsWindows() bool {
	return filepath.Separator == '\\' || os.Getenv("OS") == "Windows_NT"
}

// containsExe 判断目录下是否直接存在 zimage-ncnn-vulkan 可执行文件。
func containsExe(dir string) bool {
	for _, name := range exeCandidates() {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// detectBase 从服务端可执行文件所在目录向上逐级查找；
// 每一级同时检查其直接子目录，兼容「工作台与引擎平级」的目录布局。
func detectBase() string {
	start := "."
	if exe, err := os.Executable(); err == nil {
		start = filepath.Dir(exe)
	}
	dir := start
	for i := 0; i < 4; i++ {
		if containsExe(dir) {
			return dir
		}
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.IsDir() && containsExe(filepath.Join(dir, e.Name())) {
					return filepath.Join(dir, e.Name())
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func defaultConfig() Config {
	base := detectBase()
	c := Config{
		ExePath:   filepath.Join(base, exeCandidates()[0]),
		WorkDir:   base,
		OutputDir: filepath.Join(base, "out"),
		UploadDir: filepath.Join(base, "uploads"),
		ModelPath: "z-image-turbo",
		GPUID:     -2,
		Port:      19777,
	}
	if _, err := os.Stat(c.ExePath); err != nil {
		if p, err2 := exec.LookPath(exeBaseName); err2 == nil {
			c.ExePath = p
		}
	}
	return c
}

// LoadConfig 读取配置；不存在时自动探测并写入默认配置。
func LoadConfig() Config {
	exe, err := os.Executable()
	if err != nil {
		cfgFile = "config.json"
	} else {
		cfgFile = filepath.Join(filepath.Dir(exe), "config.json")
	}

	c := defaultConfig()
	if b, err := os.ReadFile(cfgFile); err == nil {
		_ = json.Unmarshal(b, &c)
		// 已保存的引擎路径失效时（目录被移动等），回到自动探测结果
		if _, err := os.Stat(c.ExePath); err != nil {
			d := defaultConfig()
			c.ExePath = d.ExePath
			c.WorkDir = d.WorkDir
			c.OutputDir = d.OutputDir
			c.UploadDir = d.UploadDir
		}
	}
	normalize(&c)

	cfgMu.Lock()
	cfgData = c
	cfgMu.Unlock()

	_ = os.MkdirAll(c.OutputDir, 0o755)
	_ = os.MkdirAll(c.UploadDir, 0o755)
	_ = SaveConfig(c)
	return c
}

func normalize(c *Config) {
	if c.Port == 0 {
		c.Port = 19777
	}
	if c.WorkDir == "" {
		c.WorkDir = detectBase()
	}
	if c.OutputDir == "" {
		c.OutputDir = filepath.Join(c.WorkDir, "out")
	}
	if c.UploadDir == "" {
		c.UploadDir = filepath.Join(c.WorkDir, "uploads")
	}
	if c.ModelPath == "" {
		c.ModelPath = "z-image-turbo"
	}
	if c.ExePath == "" {
		c.ExePath = filepath.Join(c.WorkDir, exeCandidates()[0])
	}
}

// GetConfig 返回当前配置快照。
func GetConfig() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfgData
}

// SaveConfig 持久化配置并更新内存快照。
func SaveConfig(c Config) error {
	normalize(&c)
	cfgMu.Lock()
	cfgData = c
	cfgMu.Unlock()
	_ = os.MkdirAll(c.OutputDir, 0o755)
	_ = os.MkdirAll(c.UploadDir, 0o755)
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgFile, b, 0o644)
}

// ModelInfo 描述一个模型目录是否就绪。
type ModelInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Found bool   `json:"found"`
}

// SystemInfo 是设置页展示的环境自检结果。
type SystemInfo struct {
	Version  string      `json:"version"`
	Config   Config      `json:"config"`
	ExeFound bool        `json:"exeFound"`
	Models   []ModelInfo `json:"models"`
}

// Inspect 检查可执行文件与模型目录是否就位。
func Inspect() SystemInfo {
	c := GetConfig()
	info := SystemInfo{Version: AppVersion, Config: c}
	if st, err := os.Stat(c.ExePath); err == nil && !st.IsDir() {
		info.ExeFound = true
	}
	for _, name := range []string{"z-image-turbo", "z-image-control", "z-image-control-tile"} {
		p := filepath.Join(c.WorkDir, name)
		st, err := os.Stat(p)
		info.Models = append(info.Models, ModelInfo{
			Name:  name,
			Path:  p,
			Found: err == nil && st.IsDir(),
		})
	}
	return info
}
