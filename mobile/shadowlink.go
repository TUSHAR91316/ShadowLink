package mobile

import (
	"context"
	"fmt"
	"net"

	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/network"
	"github.com/shadowlink/core/internal/socks5"
)

// MobileNode represents a running instance of ShadowLink on an iOS or Android device.
// gomobile will automatically export this struct to Swift and Kotlin.
type MobileNode struct {
	ds     *discovery.DiscoveryService
	cancel context.CancelFunc
}

// StartEntryNode is the entry point for the Mobile UI. When the user taps the "Connect"
// button in your future iOS/Android app, it will call this function.
func StartEntryNode(socksPort int) (*MobileNode, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Use port 0 to let the mobile OS dynamically assign a free networking port
	ds, err := discovery.NewDiscoveryService(ctx, 0)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize discovery: %v", err)
	}

	dialer := func(ctx context.Context, netwk, addr string) (net.Conn, error) {
		return network.DialCircuit(ctx, ds, netwk, addr)
	}

	// Start the SOCKS5 proxy on the mobile device
	proxy, err := socks5.NewServer(socksPort, dialer)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init proxy: %v", err)
	}

	go proxy.ListenAndServe(ctx)

	return &MobileNode{
		ds:     ds,
		cancel: cancel,
	}, nil
}

// Stop allows the mobile UI to cleanly disconnect from the dVPN network.
func (m *MobileNode) Stop() {
	m.cancel()
	m.ds.Host.Close()
}
