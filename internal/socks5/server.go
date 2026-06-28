package socks5

import (
	"context"
	"fmt"
	"log"
	"net"

	socks5lib "github.com/armon/go-socks5"
)

// Server represents our local entry proxy.
type Server struct {
	port int
	srv  *socks5lib.Server
}

// DialerFunc defines how we connect to the destination. For ShadowLink, this
// will construct an onion circuit via libp2p instead of direct TCP dialing.
type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// NewServer initializes the local proxy.
func NewServer(port int, dialer DialerFunc) (*Server, error) {
	// If no dialer is provided, fallback to direct TCP (useful for testing)
	if dialer == nil {
		dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			log.Printf("Bypassing dVPN, dialing directly: %s", addr)
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}
	}

	conf := &socks5lib.Config{
		Dial: dialer,
	}

	server, err := socks5lib.New(conf)
	if err != nil {
		return nil, err
	}

	return &Server{
		port: port,
		srv:  server,
	}, nil
}

// ListenAndServe blocks and listens for incoming browser connections.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	log.Printf("SOCKS5 Proxy listening on %s", addr)

	// Close listener when context is cancelled
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	return s.srv.Serve(listener)
}
