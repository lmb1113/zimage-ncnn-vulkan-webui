package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"zimage-webui/server"
)

//go:embed all:web/dist
var webFS embed.FS

// 保证 embed 的文件系统在构建期被引用
var _ fs.FS = webFS

func main() {
	port := flag.Int("port", 0, "服务端口；0 表示使用 config.json 中的端口")
	noBrowser := flag.Bool("no-browser", false, "启动后不自动打开浏览器")
	flag.Parse()

	cfg := server.LoadConfig()
	if *port > 0 {
		cfg.Port = *port
	}

	srv, err := server.New(cfg, webFS)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	url := "http://" + addr
	log.Printf("Z-Image 工作台已启动: %s", url)

	if !*noBrowser {
		go func() {
			time.Sleep(600 * time.Millisecond)
			openBrowser(url)
		}()
	}

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
