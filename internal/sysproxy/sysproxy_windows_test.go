//go:build windows

package sysproxy

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// TestWindows_EnableDisableRoundTrip tests enabling and disabling the proxy in the registry.
func TestWindows_EnableDisableRoundTrip(t *testing.T) {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.READ|registry.SET_VALUE)
	if err != nil {
		t.Skipf("cannot open registry for test: %v", err)
	}
	defer k.Close()

	// Backup original values
	origEnable, _, err := k.GetIntegerValue("ProxyEnable")
	hasOrigEnable := err == nil
	origServer, _, err := k.GetStringValue("ProxyServer")
	hasOrigServer := err == nil

	defer func() {
		// Restore original state
		if hasOrigEnable {
			_ = k.SetDWordValue("ProxyEnable", uint32(origEnable))
		}
		if hasOrigServer {
			_ = k.SetStringValue("ProxyServer", origServer)
		} else {
			_ = k.SetStringValue("ProxyServer", "")
		}
	}()

	testHost := "127.0.0.1"
	testPort := 1088

	// Enable
	if err := EnableSOCKS5(testHost, testPort); err != nil {
		t.Fatalf("EnableSOCKS5 failed: %v", err)
	}

	val, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || val != 1 {
		t.Errorf("expected ProxyEnable=1, got %d (err: %v)", val, err)
	}

	server, _, err := k.GetStringValue("ProxyServer")
	expectedServer := fmt.Sprintf("SOCKS=%s:%d", testHost, testPort)
	if err != nil || server != expectedServer {
		t.Errorf("expected ProxyServer=%q, got %q (err: %v)", expectedServer, server, err)
	}

	// Disable
	if err := Disable(); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	val, _, err = k.GetIntegerValue("ProxyEnable")
	if err != nil || val != 0 {
		t.Errorf("expected ProxyEnable=0, got %d (err: %v)", val, err)
	}

	server, _, err = k.GetStringValue("ProxyServer")
	if err != nil || server != "" {
		t.Errorf("expected ProxyServer=\"\", got %q (err: %v)", server, err)
	}
}
