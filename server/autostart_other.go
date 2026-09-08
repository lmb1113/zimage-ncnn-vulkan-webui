//go:build !windows

package server

import "fmt"

// AutostartSupported 非 Windows 平台暂未实现自启配置。
func AutostartSupported() bool { return false }

// IsAutostartEnabled 非 Windows 平台恒为 false。
func IsAutostartEnabled() bool { return false }

// SetAutostart 非 Windows 平台不支持。
func SetAutostart(bool) error {
	return fmt.Errorf("当前平台暂不支持开机自启")
}
