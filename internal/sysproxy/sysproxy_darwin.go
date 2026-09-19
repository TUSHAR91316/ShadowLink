//go:build darwin

package sysproxy

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

var (
	configuredServices []string
	serviceMutex       sync.Mutex
)

// getActiveServices returns the list of valid network services on macOS.
func getActiveServices() []string {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		// Ignore header or disabled services that start with '*'
		if trimmed == "" || strings.HasPrefix(trimmed, "*") {
			continue
		}
		services = append(services, trimmed)
	}
	return services
}

// EnableSOCKS5 configures the macOS system SOCKS proxy using networksetup.
func EnableSOCKS5(host string, port int) error {
	serviceMutex.Lock()
	defer serviceMutex.Unlock()

	services := getActiveServices()
	if len(services) == 0 {
		return fmt.Errorf("sysproxy: no active macOS network services found")
	}

	var configured []string
	portStr := fmt.Sprintf("%d", port)
	for _, s := range services {
		cmd := exec.Command("networksetup", "-setsocksfirewallproxy", s, host, portStr)
		if err := cmd.Run(); err != nil {
			continue
		}
		cmdOn := exec.Command("networksetup", "-setsocksfirewallproxystate", s, "on")
		if err := cmdOn.Run(); err != nil {
			continue
		}
		configured = append(configured, s)
	}

	if len(configured) == 0 {
		return fmt.Errorf("sysproxy: failed to configure macOS proxy on any network service")
	}

	configuredServices = configured
	log.Printf("macOS system SOCKS5 proxy enabled on services: %v", configured)
	return nil
}

// Disable restores the macOS system proxy state to disabled.
func Disable() error {
	serviceMutex.Lock()
	defer serviceMutex.Unlock()

	services := configuredServices
	if len(services) == 0 {
		services = getActiveServices()
	}

	var lastErr error
	for _, s := range services {
		cmdOff := exec.Command("networksetup", "-setsocksfirewallproxystate", s, "off")
		if err := cmdOff.Run(); err != nil {
			lastErr = err
		}
	}
	configuredServices = nil
	return lastErr
}
