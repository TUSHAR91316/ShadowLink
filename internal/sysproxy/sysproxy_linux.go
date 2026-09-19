//go:build linux

package sysproxy

import (
	"fmt"
	"log"
	"os/exec"
)

// EnableSOCKS5 sets the Linux system proxy using gsettings (GNOME/desktop environments).
func EnableSOCKS5(host string, port int) error {
	if _, err := exec.LookPath("gsettings"); err != nil {
		log.Printf("Linux: gsettings not found (headless or non-GNOME environment).")
		log.Printf("Please manually configure your proxy or set: export all_proxy=socks5://%s:%d", host, port)
		return nil
	}

	// 1. Set mode to manual
	if err := exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "manual").Run(); err != nil {
		return fmt.Errorf("failed to set GNOME proxy mode to manual: %w", err)
	}

	// 2. Set SOCKS host
	if err := exec.Command("gsettings", "set", "org.gnome.system.proxy.socks", "host", host).Run(); err != nil {
		return fmt.Errorf("failed to set GNOME SOCKS host: %w", err)
	}

	// 3. Set SOCKS port
	portStr := fmt.Sprintf("%d", port)
	if err := exec.Command("gsettings", "set", "org.gnome.system.proxy.socks", "port", portStr).Run(); err != nil {
		return fmt.Errorf("failed to set GNOME SOCKS port: %w", err)
	}

	log.Printf("Linux GNOME system SOCKS5 proxy configured to %s:%d", host, port)
	return nil
}

// Disable restores the Linux system proxy setting to none.
func Disable() error {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return nil
	}

	if err := exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run(); err != nil {
		return fmt.Errorf("failed to reset GNOME proxy mode to none: %w", err)
	}
	return nil
}
