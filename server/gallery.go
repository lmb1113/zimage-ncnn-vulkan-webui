package server

import (
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GalleryItem 是图库中一张图片的摘要信息。
type GalleryItem struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Path      string `json:"path,omitempty"` // 服务端绝对路径，供一键联动复用
	Size      int64  `json:"size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	ModTime   string `json:"modTime"`
	Mode      string `json:"mode,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Negative  string `json:"negative,omitempty"`
	Seed      int64  `json:"seed,omitempty"`
	Steps     int    `json:"steps,omitempty"`
	ElapsedMs int64  `json:"elapsedMs,omitempty"`
	Command   string `json:"command,omitempty"`
}

// metaRecord 是写在 .meta 目录里的生成参数快照。
type metaRecord struct {
	Mode      string    `json:"mode"`
	Params    Params    `json:"params"`
	ElapsedMs int64     `json:"elapsedMs"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
}

func metaDir(outputDir string) string {
	return filepath.Join(outputDir, ".meta")
}

func saveMeta(outputDir string, j *Job) {
	dir := metaDir(outputDir)
	_ = os.MkdirAll(dir, 0o755)
	rec := metaRecord{
		Mode:      j.Mode,
		Params:    j.Params,
		ElapsedMs: j.ElapsedMs,
		Command:   j.Command,
		CreatedAt: time.Now(),
	}
	if j.EndedAt != nil {
		rec.CreatedAt = *j.EndedAt
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	for _, name := range j.Outputs {
		_ = os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644)
	}
}

func readMeta(outputDir, name string) (metaRecord, bool) {
	var rec metaRecord
	b, err := os.ReadFile(filepath.Join(metaDir(outputDir), name+".json"))
	if err != nil {
		return rec, false
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return rec, false
	}
	return rec, true
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
		return true
	}
	return false
}

// ListGallery 扫描输出目录，按修改时间倒序返回图片列表。
func ListGallery() []GalleryItem {
	cfg := GetConfig()
	entries, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		return []GalleryItem{}
	}

	items := make([]GalleryItem, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isImage(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		it := GalleryItem{
			Name:    e.Name(),
			URL:     "/media/" + e.Name(),
			Path:    filepath.Join(cfg.OutputDir, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		}
		if f, err := os.Open(filepath.Join(cfg.OutputDir, e.Name())); err == nil {
			if c, _, err := image.DecodeConfig(f); err == nil {
				it.Width = c.Width
				it.Height = c.Height
			}
			_ = f.Close()
		}
		items = append(items, it)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name > items[j].Name
	})

	// 元数据按需补齐（图库首屏只补最近 60 张，避免大量小文件读取）
	limit := len(items)
	if limit > 60 {
		limit = 60
	}
	for i := 0; i < limit; i++ {
		if rec, ok := readMeta(cfg.OutputDir, items[i].Name); ok {
			items[i].Mode = rec.Mode
			items[i].Prompt = rec.Params.Prompt
			items[i].Negative = rec.Params.Negative
			items[i].Seed = rec.Params.Seed
			items[i].Steps = rec.Params.Steps
			items[i].ElapsedMs = rec.ElapsedMs
			items[i].Command = rec.Command
		}
	}
	return items
}

// GalleryDetail 返回单张图片的完整信息（含生成参数）。
func GalleryDetail(name string) (GalleryItem, bool) {
	cfg := GetConfig()
	clean := filepath.Base(name)
	st, err := os.Stat(filepath.Join(cfg.OutputDir, clean))
	if err != nil || st.IsDir() {
		return GalleryItem{}, false
	}
	it := GalleryItem{
		Name:    clean,
		URL:     "/media/" + clean,
		Path:    filepath.Join(cfg.OutputDir, clean),
		Size:    st.Size(),
		ModTime: st.ModTime().Format("2006-01-02 15:04:05"),
	}
	if f, err := os.Open(filepath.Join(cfg.OutputDir, clean)); err == nil {
		if c, _, err := image.DecodeConfig(f); err == nil {
			it.Width = c.Width
			it.Height = c.Height
		}
		_ = f.Close()
	}
	if rec, ok := readMeta(cfg.OutputDir, clean); ok {
		it.Mode = rec.Mode
		it.Prompt = rec.Params.Prompt
		it.Negative = rec.Params.Negative
		it.Seed = rec.Params.Seed
		it.Steps = rec.Params.Steps
		it.ElapsedMs = rec.ElapsedMs
		it.Command = rec.Command
	}
	return it, true
}

// DeleteImage 删除图片及其元数据。
func DeleteImage(name string) error {
	cfg := GetConfig()
	clean := filepath.Base(name)
	if err := os.Remove(filepath.Join(cfg.OutputDir, clean)); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(metaDir(cfg.OutputDir), clean+".json"))
	return nil
}
