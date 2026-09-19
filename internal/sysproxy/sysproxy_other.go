//go:build !windows && !darwin && !linux

package sysproxy

import (
	"log"
)

// EnableSOCKS5 provides a no-op fallback for unsupported OS targets (e.g. BSD, Solaris).
func EnableSOCKS5(ip string, port int) error {
	log.Println("Automatic system proxy configuration is not supported on this platform.")
	log.Printf("Please manually configure your OS proxy to SOCKS5 at %s:%d", ip, port)
	return nil
}

// Disable provides a no-op placeholder for macOS/Linux.
// Returns nil always; no registry manipulation needed on these platforms.
// Future implementations can use 'networksetup -setwebproxystate' on macOS or 'gsettings' on Linux.
func Disable() error {
	return nil
}
