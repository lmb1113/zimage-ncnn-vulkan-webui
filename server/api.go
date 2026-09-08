package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"math/rand/v2"
)

// Server 聚合了任务管理器、事件总线与前端资源（磁盘优先，其次内嵌）。
type Server struct {
	bus    *Bus
	mgr    *Manager
	webDir string // 磁盘上的构建产物目录（可选，便于开发期热更）
	webFS  fs.FS  // 编译期嵌入的前端产物（单文件分发用）
}

// New 创建服务端实例；web 为嵌入的前端文件系统，可为 nil。
func New(cfg Config, web fs.FS) (*Server, error) {
	bus := newBus()
	mgr := NewManager(bus)

	webDir := ""
	if exe, err := os.Executable(); err == nil {
		webDir = filepath.Join(filepath.Dir(exe), "web", "dist")
	} else {
		webDir = filepath.Join("web", "dist")
	}
	return &Server{bus: bus, mgr: mgr, webDir: webDir, webFS: web}, nil
}

// Handler 返回全部路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("POST /api/config", s.postConfig)
	mux.HandleFunc("GET /api/system", s.system)

	mux.HandleFunc("POST /api/generate", s.generate)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("DELETE /api/jobs/{id}", s.deleteJob)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.cancelJob)
	mux.HandleFunc("POST /api/jobs/cancel-queued", s.cancelQueued)

	mux.HandleFunc("GET /api/events", s.events)

	mux.HandleFunc("POST /api/upload", s.upload)
	mux.HandleFunc("POST /api/uploads/clear", s.clearUploads)
	mux.HandleFunc("GET /api/gallery", s.gallery)
	mux.HandleFunc("GET /api/gallery/{name}", s.galleryDetail)
	mux.HandleFunc("GET /api/gallery/download", s.galleryDownload)
	mux.HandleFunc("DELETE /api/gallery/{name}", s.galleryDelete)

	mux.HandleFunc("POST /api/open", s.openPath)
	mux.HandleFunc("GET /api/autostart", s.getAutostart)
	mux.HandleFunc("POST /api/autostart", s.postAutostart)

	mux.HandleFunc("/media/", s.mediaHandler)
	mux.HandleFunc("GET /media/thumb/", s.mediaThumb)
	mux.HandleFunc("/uploads/", s.uploadHandler)
	mux.HandleFunc("/", s.spa)

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, Inspect())
}

func (s *Server) postConfig(w http.ResponseWriter, r *http.Request) {
	var c Config
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := SaveConfig(c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.bus.Publish("config", Inspect())
	writeJSON(w, Inspect())
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, Inspect())
}

type generateRequest struct {
	Mode   string `json:"mode"`
	Params Params `json:"params"`
}

func (s *Server) generate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	p := req.Params
	if p.Width <= 0 {
		p.Width = 1024
	}
	if p.Height <= 0 {
		p.Height = 1024
	}
	if p.Batch <= 0 {
		p.Batch = 1
	}
	if p.Seed == 0 {
		p.Seed = -1
	}
	if p.ModelPath == "" {
		p.ModelPath = GetConfig().ModelPath
	}
	if req.Mode == "" {
		req.Mode = "txt2img"
	}
	j := s.mgr.Submit(p, req.Mode)
	writeJSON(w, j)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.mgr.List())
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	j := s.mgr.Get(r.PathValue("id"))
	if j == nil {
		writeErr(w, http.StatusNotFound, "任务不存在")
		return
	}
	writeJSON(w, j)
}

func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request) {
	s.mgr.Delete(r.PathValue("id"))
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	ok := s.mgr.Cancel(r.PathValue("id"))
	writeJSON(w, map[string]bool{"ok": ok})
}

func (s *Server) cancelQueued(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]int{"canceled": s.mgr.CancelQueued()})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	ch := s.bus.Subscribe()
	defer s.bus.Unsubscribe(ch)

	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			flusher.Flush()
		case ev := <-ch:
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()

	cfg := GetConfig()
	_ = os.MkdirAll(cfg.UploadDir, 0o755)

	ext := strings.ToLower(filepath.Ext(header.Filename))
	name := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), randSuffix(6), ext)
	dst := filepath.Join(cfg.UploadDir, name)

	out, err := os.Create(dst)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]string{
		"path": dst,
		"name": name,
		"url":  "/uploads/" + name,
	})
}

// randSuffix 生成随机文件名后缀。
// 使用标准库随机数；此前手写 LCG 因 int64 溢出产生负数导致负索引 panic（上传接口崩溃）。
// galleryDownload 把指定图片（names 逗号分隔；缺省为全部）打包成 ZIP 下载。
func (s *Server) galleryDownload(w http.ResponseWriter, r *http.Request) {
	cfg := GetConfig()
	q := strings.Trim(r.URL.Query().Get("names"), ",")
	var names []string
	if q == "" {
		entries, err := os.ReadDir(cfg.OutputDir)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, e := range entries {
			if !e.IsDir() && isImage(e.Name()) {
				names = append(names, e.Name())
			}
		}
	} else {
		for _, n := range strings.Split(q, ",") {
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
	}
	if len(names) == 0 {
		writeErr(w, http.StatusBadRequest, "没有可下载的图片")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		`attachment; filename="z-image-`+time.Now().Format("20060102-150405")+`.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, name := range names {
		name = filepath.Base(name) // 防路径穿越
		src, err := os.Open(filepath.Join(cfg.OutputDir, name))
		if err != nil {
			continue // 单个缺失不影响整体
		}
		fw, err := zw.Create(name)
		if err == nil {
			_, _ = io.Copy(fw, src)
		}
		src.Close()
	}
}

// clearUploads 清空上传目录（仅删文件，不动目录结构）。
func (s *Server) clearUploads(w http.ResponseWriter, r *http.Request) {
	cfg := GetConfig()
	entries, err := os.ReadDir(cfg.UploadDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if os.Remove(filepath.Join(cfg.UploadDir, e.Name())) == nil {
			removed++
		}
	}
	writeJSON(w, map[string]interface{}{"ok": true, "removed": removed})
}

func randSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

func (s *Server) gallery(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, ListGallery())
}

func (s *Server) galleryDetail(w http.ResponseWriter, r *http.Request) {
	it, ok := GalleryDetail(r.PathValue("name"))
	if !ok {
		writeErr(w, http.StatusNotFound, "图片不存在")
		return
	}
	writeJSON(w, it)
}

func (s *Server) galleryDelete(w http.ResponseWriter, r *http.Request) {
	if err := DeleteImage(r.PathValue("name")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

type openRequest struct {
	Path string `json:"path"`
}

func (s *Server) openPath(w http.ResponseWriter, r *http.Request) {
	var req openRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	target := req.Path
	if target == "" {
		target = GetConfig().OutputDir
	}
	if _, err := os.Stat(target); err != nil {
		writeErr(w, http.StatusBadRequest, "路径不存在: "+target)
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// autostartStatus 返回当前平台是否支持自启及当前状态。
func (s *Server) getAutostart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{
		"supported": AutostartSupported(),
		"enabled":   AutostartSupported() && IsAutostartEnabled(),
	})
}

type autostartRequest struct {
	Enable bool `json:"enable"`
}

func (s *Server) postAutostart(w http.ResponseWriter, r *http.Request) {
	if !AutostartSupported() {
		writeErr(w, http.StatusBadRequest, "当前平台暂不支持开机自启")
		return
	}
	var req autostartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := SetAutostart(req.Enable); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true, "enabled": IsAutostartEnabled()})
}

// mediaThumb 输出图片的缩略图（480px JPEG，首次访问生成并缓存，解码失败回退原图）。
func (s *Server) mediaThumb(w http.ResponseWriter, r *http.Request) {
	cfg := GetConfig()
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/media/thumb/"))
	dst, err := ensureThumb(cfg.OutputDir, name)
	if err != nil {
		http.ServeFile(w, r, filepath.Join(cfg.OutputDir, name))
		return
	}
	http.ServeFile(w, r, dst)
}

func (s *Server) mediaHandler(w http.ResponseWriter, r *http.Request) {
	cfg := GetConfig()
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/media/"))
	http.ServeFile(w, r, filepath.Join(cfg.OutputDir, name))
}

func (s *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	cfg := GetConfig()
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/uploads/"))
	http.ServeFile(w, r, filepath.Join(cfg.UploadDir, name))
}

// spa 服务前端：磁盘构建产物优先（便于开发），否则回退到内嵌资源。
func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if s.webDir != "" {
		if st, err := os.Stat(s.webDir); err == nil && st.IsDir() {
			rel := strings.TrimPrefix(r.URL.Path, "/")
			if rel == "" {
				rel = "index.html"
			}
			full := filepath.Join(s.webDir, filepath.FromSlash(rel))
			if st, err := os.Stat(full); err == nil && !st.IsDir() {
				http.ServeFile(w, r, full)
				return
			}
			http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
			return
		}
	}

	if s.webFS != nil {
		name := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		name = strings.TrimPrefix(name, "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if b, err := fs.ReadFile(s.webFS, "web/dist/"+name); err == nil {
			if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
				w.Header().Set("Content-Type", ct)
			}
			_, _ = w.Write(b)
			return
		}
		if b, err := fs.ReadFile(s.webFS, "web/dist/index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, fallbackHTML)
}

const fallbackHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Z-Image 工作台</title>
<style>body{font-family:system-ui,"Microsoft YaHei",sans-serif;background:#F6F7F9;color:#14161A;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.box{max-width:520px;padding:32px;background:#fff;border:1px solid #EBECEF;border-radius:12px;line-height:1.7}
code{background:#F3F4F6;padding:2px 6px;border-radius:4px}</style></head>
<body><div class="box"><h2>前端尚未构建</h2>
<p>后端已启动。请在 <code>web</code> 目录下执行：</p>
<p><code>npm install &amp;&amp; npm run build</code></p>
<p>构建完成后刷新本页即可使用。</p></div></body></html>`
