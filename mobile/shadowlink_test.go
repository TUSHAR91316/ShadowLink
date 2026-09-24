package mobile

import (
	"testing"

	"github.com/shadowlink/core/internal/config"
)

func TestDefaultSOCKSPort(t *testing.T) {
	port := DefaultSOCKSPort()
	if port != int64(config.DefaultSOCKSPort) {
		t.Errorf("expected %d, got %d", config.DefaultSOCKSPort, port)
	}
}

func TestStartEntryNode_InvalidPort(t *testing.T) {
	_, err := StartEntryNode(-1)
	if err == nil {
		t.Error("expected error for negative port, got nil")
	}

	_, err = StartEntryNode(70000)
	if err == nil {
		t.Error("expected error for port > 65535, got nil")
	}
}
