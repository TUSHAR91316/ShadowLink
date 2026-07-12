package socks5

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestNewServer_NilDialer verifies that a nil dialer is replaced with the
// safe TCP fallback so the server is still usable for testing.
func TestNewServer_NilDialer(t *testing.T) {
	srv, err := NewServer(0, nil)
	if err != nil {
		t.Fatalf("NewServer with nil dialer: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer returned nil server")
	}
}

// TestNewServer_CustomDialer verifies a custom dialer is accepted without error.
func TestNewServer_CustomDialer(t *testing.T) {
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return net.Dial(network, addr)
	}
	srv, err := NewServer(0, dialer)
	if err != nil {
		t.Fatalf("NewServer with custom dialer: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}

// TestListenAndServe_ContextCancellation verifies that cancelling the context
// causes ListenAndServe to return nil (clean shutdown, not a panic/Fatalf).
func TestListenAndServe_ContextCancellation(t *testing.T) {
	// Use port 0 so the OS assigns a free port (avoids test port conflicts).
	srv, err := NewServer(0, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()

	// Give the server a moment to start listening
	time.Sleep(50 * time.Millisecond)

	// Cancel the context → must trigger clean shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServe must return nil on clean shutdown, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("ListenAndServe did not shut down within 3 seconds after context cancellation")
	}
}

// TestListenAndServe_PortInUse verifies that starting a server on an already-
// occupied port returns an error immediately.
func TestListenAndServe_PortInUse(t *testing.T) {
	// Bind a port to occupy it
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind test port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	srv, _ := NewServer(port, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = srv.ListenAndServe(ctx)
	if err == nil {
		t.Error("ListenAndServe must fail when the port is already in use")
	}
}
