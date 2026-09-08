//go:build windows

package server

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const autostartKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
const autostartValue = "ZImageWebUI"

// AutostartSupported 当前平台是否支持开机自启。
func AutostartSupported() bool { return true }

// IsAutostartEnabled 读取注册表 Run 项判断是否已配置自启。
func IsAutostartEnabled() bool {
	out, err := exec.Command("reg", "query", autostartKey, "/v", autostartValue).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(autostartValue))
}

// SetAutostart 写入或移除开机自启（以 -no-browser -tray 静默常驻）。
func SetAutostart(enable bool) error {
	if !enable {
		err := exec.Command("reg", "delete", autostartKey, "/v", autostartValue, "/f").Run()
		if err != nil {
			// 值本来就不存在视为成功
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				return nil
			}
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	val := fmt.Sprintf(`"%s" -no-browser`, exe)
	return exec.Command("reg", "add", autostartKey, "/v", autostartValue, "/t", "REG_SZ", "/d", val, "/f").Run()
}
