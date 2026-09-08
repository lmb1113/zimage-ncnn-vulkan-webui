package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer 基于临时目录构建一台隔离的测试服务：
// 引擎路径指向不存在的文件（不会真的起进程），配置写在临时目录内。
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		ExePath:   filepath.Join(dir, "no-such-engine.exe"),
		WorkDir:   dir,
		OutputDir: filepath.Join(dir, "out"),
		UploadDir: filepath.Join(dir, "uploads"),
		ModelPath: "z-image-turbo",
		GPUID:     -2,
		Port:      0,
	}
	cfgFile = filepath.Join(dir, "config.json")
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	srv, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		for _, j := range srv.mgr.List() {
			srv.mgr.Cancel(j.ID)
		}
	})
	return srv
}

func doJSON(t *testing.T, srv *Server, method, path string, body []byte) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.White)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// ---------- /api/upload 与 /api/uploads/clear ----------

func TestUploadAndClearRoundTrip(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	// multipart 上传
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "hello.png")
	fw.Write([]byte("fake-png-bytes"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("上传应成功，状态码 %d: %s", rec.Code, rec.Body.String())
	}

	cfg := GetConfig()
	entries, _ := os.ReadDir(cfg.UploadDir)
	if len(entries) != 1 {
		t.Fatalf("上传后目录应有 1 个文件，实际 %d", len(entries))
	}

	// 清空
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest("POST", "/api/uploads/clear", nil))
	if rec2.Code != 200 {
		t.Fatalf("清空应成功: %s", rec2.Body.String())
	}
	entries, _ = os.ReadDir(cfg.UploadDir)
	if len(entries) != 0 {
		t.Errorf("清空后目录应为空，实际 %d 个", len(entries))
	}
}

// ---------- /api/gallery 列表 / 详情 / 删除 ----------

func TestGalleryRoundTrip(t *testing.T) {
	srv := newTestServer(t)
	cfg := GetConfig()

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "20260101_000000.000.png"
	writeTestPNG(t, filepath.Join(cfg.OutputDir, name), 3, 2)

	// 列表
	rec, _ := doJSON(t, srv, "GET", "/api/gallery", nil)
	if rec.Code != 200 {
		t.Fatalf("图库列表应成功: %d", rec.Code)
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("图库应有 1 张图，实际 %d", len(list))
	}
	if list[0]["name"] != name {
		t.Errorf("名称不符: %v", list[0]["name"])
	}
	if w, _ := list[0]["width"].(float64); int(w) != 3 {
		t.Errorf("应解析出宽度 3，实际 %v", list[0]["width"])
	}

	// 详情
	rec, _ = doJSON(t, srv, "GET", "/api/gallery/"+name, nil)
	if rec.Code != 200 {
		t.Fatalf("详情应成功: %d", rec.Code)
	}

	// 删除
	rec, _ = doJSON(t, srv, "DELETE", "/api/gallery/"+name, nil)
	if rec.Code != 200 {
		t.Fatalf("删除应成功: %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(cfg.OutputDir, name)); !os.IsNotExist(err) {
		t.Errorf("删除后文件应不存在")
	}
	rec, _ = doJSON(t, srv, "GET", "/api/gallery/"+name, nil)
	if rec.Code != 404 {
		t.Errorf("删除后详情应 404，实际 %d", rec.Code)
	}
}

// ---------- /api/gallery/download ZIP 打包 ----------

func TestGalleryDownloadZip(t *testing.T) {
	srv := newTestServer(t)
	cfg := GetConfig()
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(cfg.OutputDir, "a.png"), 2, 2)
	writeTestPNG(t, filepath.Join(cfg.OutputDir, "b.png"), 2, 2)

	rec, _ := doJSON(t, srv, "GET", "/api/gallery/download", nil)
	if rec.Code != 200 {
		t.Fatalf("下载应成功: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type 应为 application/zip，实际 %q", ct)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("不是有效 ZIP: %v", err)
	}
	if len(zr.File) != 2 {
		t.Errorf("ZIP 应含 2 个文件，实际 %d", len(zr.File))
	}

	// 指定 names 只打包对应文件；不存在的名字自动跳过
	rec2, _ := doJSON(t, srv, "GET", "/api/gallery/download?names=a.png,missing.png", nil)
	zr2, err := zip.NewReader(bytes.NewReader(rec2.Body.Bytes()), int64(rec2.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr2.File) != 1 || zr2.File[0].Name != "a.png" {
		t.Errorf("指定 names 应只含 a.png，实际 %d 个", len(zr2.File))
	}
}

// ---------- /api/generate 参数默认值 ----------

func TestGenerateParamDefaults(t *testing.T) {
	srv := newTestServer(t)

	body := `{"mode":"txt2img","params":{"prompt":"a cat"}}`
	req := httptest.NewRequest("POST", "/api/generate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("提交应成功: %d %s", rec.Code, rec.Body.String())
	}

	var job struct {
		Params Params `json:"params"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Params.Width != 1024 || job.Params.Height != 1024 {
		t.Errorf("默认尺寸应 1024x1024，实际 %dx%d", job.Params.Width, job.Params.Height)
	}
	if job.Params.Batch != 1 {
		t.Errorf("默认批量应 1，实际 %d", job.Params.Batch)
	}
	if job.Params.Seed != -1 {
		t.Errorf("默认种子应 -1(随机)，实际 %d", job.Params.Seed)
	}
	if job.Params.GPUID != nil {
		t.Errorf("未指定设备时 GPUID 应为 nil(自动)，实际 %v", *job.Params.GPUID)
	}
	if job.Params.ModelPath != "z-image-turbo" {
		t.Errorf("默认模型应继承配置，实际 %q", job.Params.ModelPath)
	}
	// 引擎路径不存在：worker 很快会把任务标为失败（不校验时序，仅确认状态合法）
	if job.Status != StatusQueued && job.Status != StatusRunning && job.Status != StatusFailed {
		t.Errorf("状态异常: %q", job.Status)
	}
}

// ---------- SPA 回退 ----------

func TestSPAFallback(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("首页应 200，实际 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("首页应为 HTML，实际 %q", ct)
	}
}
