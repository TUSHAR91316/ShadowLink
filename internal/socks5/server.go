package socks5

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

	socks5lib "github.com/armon/go-socks5"
)

// Server is the local SOCKS5 entry proxy that accepts browser/app connections
// and dials out through the ShadowLink onion circuit.
type Server struct {
	port int
	srv  *socks5lib.Server
}

// DialerFunc is the function signature used to create outbound connections.
// For ShadowLink, the dialer builds an onion circuit via libp2p instead of
// dialing TCP directly.
type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// NewServer creates a SOCKS5 proxy server that uses dialer for all outbound
// connections. If dialer is nil, a safe direct-TCP fallback is used (useful for
// local testing without a live dVPN network).
func NewServer(port int, dialer DialerFunc) (*Server, error) {
	if dialer == nil {
		dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			log.Printf("SOCKS5: no dVPN circuit available — bypassing and dialing %s directly", addr)
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}
	}

	server, err := socks5lib.New(&socks5lib.Config{Dial: dialer})
	if err != nil {
		return nil, fmt.Errorf("socks5: failed to create server: %w", err)
	}

	return &Server{port: port, srv: server}, nil
}

// ListenAndServe binds to the configured port and serves SOCKS5 connections
// until the ctx is cancelled. Returns nil on clean shutdown; returns the
// underlying network error on unexpected failure.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("socks5: listen on %s: %w", addr, err)
	}

	log.Printf("SOCKS5 proxy listening on %s", addr)

	// Watch for context cancellation in a dedicated goroutine and close the
	// listener when signalled. stopCh ensures the goroutine exits when the
	// server terminates normally (before ctx is cancelled).
	stopCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-stopCh:
		}
	}()

	serveErr := s.srv.Serve(listener)
	close(stopCh)

	// When the context is cancelled, listener.Close() causes Serve() to return
	// a "use of closed network connection" error. This is expected and clean;
	// mask it with nil so callers don't treat shutdown as a failure.
	if ctx.Err() != nil {
		return nil
	}

	// An unexpected listener error — surface it to the caller with context.
	if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
		return fmt.Errorf("socks5: server error: %w", serveErr)
	}
	return nil
}
