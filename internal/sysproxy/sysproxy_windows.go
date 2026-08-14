//go:build windows

package sysproxy

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
)

const internetSettingsPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// EnableSOCKS5 sets the Windows system proxy to a local SOCKS5 proxy safely.
func EnableSOCKS5(host string, port int) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key (permission denied or not found): %v", err)
	}
	defer k.Close()

	// Fix from analysis report: Windows expects SOCKS=host:port not socks5://host:port
	proxyServer := fmt.Sprintf("SOCKS=%s:%d", host, port)

	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("failed to enable proxy: %v", err)
	}

	if err := k.SetStringValue("ProxyServer", proxyServer); err != nil {
		return fmt.Errorf("failed to set proxy server: %v", err)
	}

	return nil
}

// Disable removes the system proxy setting.
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %v", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return fmt.Errorf("failed to disable proxy: %v", err)
	}

	return nil
}
