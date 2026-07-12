//go:build !windows

package sysproxy

import (
	"log"
)

// EnableSOCKS5 provides a no-op placeholder for macOS/Linux system proxy.
// Future implementations can use 'networksetup' on macOS or 'gsettings' on Linux.
func EnableSOCKS5(ip string, port int) error {
	log.Println("Automatic system proxy configuration is currently only supported on Windows.")
	log.Printf("Please manually configure your OS proxy to SOCKS5 at %s:%d", ip, port)
	return nil
}

// Disable provides a no-op placeholder for macOS/Linux.
// Returns nil always; no registry manipulation needed on these platforms.
// Future implementations can use 'networksetup -setwebproxystate' on macOS or 'gsettings' on Linux.
func Disable() error {
	return nil
}
