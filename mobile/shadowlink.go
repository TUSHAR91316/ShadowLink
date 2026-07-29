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

// MobileNode represents a running instance of ShadowLink on an iOS or Android device.
// gomobile automatically exports this struct to Swift and Kotlin.
type MobileNode struct {
	ds     *discovery.DiscoveryService
	cancel context.CancelFunc
}

// StartEntryNode starts a ShadowLink entry node on the mobile device.
//
// This is the entry point called by the Flutter MethodChannel when the user
// taps "Connect". It initialises the DHT, builds circuits on demand, and
// listens for SOCKS5 connections on the given port.
//
// NOTE: socksPort is int64 because gomobile maps Go int64 → Java long and Swift Int.
// Use config.DefaultSOCKSPort (1080) as the standard value.
func StartEntryNode(socksPort int64) (*MobileNode, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Port 0 lets the mobile OS dynamically assign a free P2P port.
	// On mobile we do not connect to bootstrap peers — the device may be behind
	// NAT and battery-sensitive; peers are discovered opportunistically.
	ds, err := discovery.NewDiscoveryService(ctx, 0, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize discovery: %v", err)
	}

	dialer := func(ctx context.Context, netwk, addr string) (net.Conn, error) {
		return network.DialCircuit(ctx, ds, netwk, addr)
	}

	// Start the SOCKS5 proxy on the requested port.
	proxy, err := socks5.NewServer(int(socksPort), dialer)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init proxy: %v", err)
	}

	go func() {
		if err := proxy.ListenAndServe(ctx); err != nil {
			// Expected error on context cancellation; not fatal.
			_ = err
		}
	}()

	return &MobileNode{
		ds:     ds,
		cancel: cancel,
	}, nil
}

// DefaultSOCKSPort returns the default SOCKS5 proxy port used by this build.
// Exposed via gomobile so host apps can read it without hardcoding.
func DefaultSOCKSPort() int64 {
	return int64(config.DefaultSOCKSPort)
}

// Stop cleanly disconnects from the dVPN network.
func (m *MobileNode) Stop() {
	m.cancel()
	m.ds.Host.Close()
}
