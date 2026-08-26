package mobile

import (
	"context"
	"fmt"
	"net"

	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/network"
	"github.com/shadowlink/core/internal/socks5"
)

// MobileNode represents a running ShadowLink entry node on iOS or Android.
// gomobile exports this struct and its methods to Swift and Kotlin automatically.
type MobileNode struct {
	ds     *discovery.DiscoveryService
	cancel context.CancelFunc
}

// StartEntryNode starts a ShadowLink entry node and a local SOCKS5 proxy on
// the given port. It is the entry point called by the Flutter MethodChannel when
// the user taps "Connect".
//
// socksPort is int64 because gomobile maps Go int64 → Java long / Swift Int.
// Use DefaultSOCKSPort() to get the standard value without hardcoding.
//
// Passing socksPort=0 lets the OS assign a free port (useful for testing).
func StartEntryNode(socksPort int64) (*MobileNode, error) {
	if socksPort < 0 || socksPort > 65535 {
		return nil, fmt.Errorf("mobile: socksPort %d out of range [0, 65535]", socksPort)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Port 0 lets the mobile OS assign a free P2P port.
	// Uses DefaultBootstrapPeers with bounded timeout to find DHT relay/exit nodes.
	ds, err := discovery.NewDiscoveryService(ctx, 0, config.DefaultBootstrapPeers)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("mobile: discovery service init failed: %w", err)
	}

	dialer := func(dialCtx context.Context, netwk, addr string) (net.Conn, error) {
		return network.DialCircuit(dialCtx, ds, netwk, addr)
	}

	proxy, err := socks5.NewServer(int(socksPort), dialer)
	if err != nil {
		cancel()
		_ = ds.Host.Close()
		return nil, fmt.Errorf("mobile: SOCKS5 proxy init failed: %w", err)
	}

	go func() {
		// ListenAndServe returns nil on clean shutdown (ctx cancelled).
		if err := proxy.ListenAndServe(ctx); err != nil {
			// Unexpected error — log it. On mobile we cannot os.Exit.
			fmt.Printf("mobile: SOCKS5 proxy exited with error: %v\n", err)
		}
	}()

	return &MobileNode{ds: ds, cancel: cancel}, nil
}

// DefaultSOCKSPort returns the standard SOCKS5 proxy port for this build.
// Exposed via gomobile so host apps can read it without hardcoding the value.
func DefaultSOCKSPort() int64 {
	return int64(config.DefaultSOCKSPort)
}

// Stop cleanly shuts down the ShadowLink entry node:
// it cancels all in-flight circuit dials and closes the libp2p host,
// freeing the P2P port binding immediately.
func (m *MobileNode) Stop() {
	m.cancel()
	_ = m.ds.Host.Close()
}
