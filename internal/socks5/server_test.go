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

// TestServer_AddrAndPortInspection verifies that Addr() and Port() correctly reflect
// the dynamically allocated ephemeral port when port 0 is configured.
func TestServer_AddrAndPortInspection(t *testing.T) {
	srv, err := NewServer(0, nil)
	if err != nil {
		t.Fatalf("NewServer(0): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()

	// Wait until server has bound to port
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Port() > 0 && srv.Addr() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if srv.Port() <= 0 {
		t.Errorf("expected dynamically bound port > 0, got %d", srv.Port())
	}
	if srv.Addr() == nil {
		t.Error("expected non-nil Addr()")
	}

	cancel()
	<-errCh
}


// TestServer_NoLocalDNSResolution verifies that hostname requests reach the dialer
// unresolved, so DNS lookups happen at the exit node instead of leaking locally.
func TestServer_NoLocalDNSResolution(t *testing.T) {
	gotAddr := make(chan string, 1)
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		gotAddr <- addr
		c1, c2 := net.Pipe()
		go c2.Close()
		return &tcpAddrConn{c1}, nil
	}
	srv, err := NewServer(0, dialer)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.ListenAndServe(ctx) //nolint:errcheck
	<-srv.Ready()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()

	host := "unresolvable.shadowlink.invalid"
	req := []byte{5, 1, 0} // greeting: version 5, one method, no-auth
	req = append(req, 5, 1, 0, 3, byte(len(host)))
	req = append(req, host...)
	req = append(req, 0x01, 0xBB) // port 443
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case addr := <-gotAddr:
		if addr != host+":443" {
			t.Errorf("dialer got %q, want %q (hostname must not be resolved locally)", addr, host+":443")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("dialer was never called — hostname was likely resolved locally and failed")
	}
}

// tcpAddrConn reports TCP addresses, which go-socks5 requires for its reply.
type tcpAddrConn struct{ net.Conn }

func (c *tcpAddrConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (c *tcpAddrConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }
