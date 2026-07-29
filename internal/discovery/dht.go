package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routing2 "github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
)

// DiscoveryService wraps a libp2p host and a Kademlia DHT for peer discovery.
type DiscoveryService struct {
	Host host.Host
	DHT  *dht.IpfsDHT
}

// NewDiscoveryService initialises a new libp2p host, bootstraps the Kad DHT,
// and optionally connects to a list of well-known bootstrap peers.
//
// Pass listenPort=0 to let the OS assign a free port (recommended for mobile).
// Pass bootstrapPeers=nil to skip bootstrap (useful for unit tests and mobile).
func NewDiscoveryService(ctx context.Context, listenPort int, bootstrapPeers []string) (*DiscoveryService, error) {
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)

	var idht *dht.IpfsDHT

	h, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.Routing(func(h host.Host) (routing2.PeerRouting, error) {
			var err error
			idht, err = dht.New(ctx, h)
			return idht, err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	log.Printf("Host created: ID=%s, listening on %s", h.ID(), listenAddr)

	// Connect to bootstrap peers concurrently for faster startup.
	if len(bootstrapPeers) > 0 {
		connectToBootstrapPeers(ctx, h, bootstrapPeers)
	}

	if err := idht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	return &DiscoveryService{
		Host: h,
		DHT:  idht,
	}, nil
}

// connectToBootstrapPeers dials all bootstrap peers in parallel and waits for
// all goroutines to finish before returning.
//
// Previously this had a classic Go goroutine variable capture bug:
//   go func() { h.Connect(ctx, *peerinfo) }()
// where peerinfo was reused across loop iterations before the goroutine ran.
// This is now fixed by passing peerinfo as a function argument.
func connectToBootstrapPeers(ctx context.Context, h host.Host, bootstrapPeers []string) {
	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapPeers {
		addr, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			log.Printf("Invalid bootstrap multiaddr %q: %v", peerAddr, err)
			continue
		}
		peerinfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Printf("Invalid peerinfo %q: %v", peerAddr, err)
			continue
		}

		wg.Add(1)
		// Pass peerinfo as an argument to the goroutine closure to avoid the
		// loop-variable capture bug present in Go versions before 1.22.
		go func(pi peer.AddrInfo) {
			defer wg.Done()
			if err := h.Connect(ctx, pi); err != nil {
				log.Printf("Failed to connect to bootstrap node %s: %v", pi.ID, err)
			} else {
				log.Printf("Connected to bootstrap node: %s", pi.ID)
			}
		}(*peerinfo)
	}
	wg.Wait()
}

// Announce registers this node under a rendezvous string in the DHT,
// making it discoverable by peers calling FindPeers with the same string.
func (d *DiscoveryService) Announce(ctx context.Context, rendezvous string) error {
	routingDiscovery := routing.NewRoutingDiscovery(d.DHT)
	_, err := routingDiscovery.Advertise(ctx, rendezvous)
	if err != nil {
		log.Printf("Warning: Failed to advertise on DHT for %q: %v", rendezvous, err)
		return err
	}
	log.Printf("Announcing presence on DHT rendezvous: %q", rendezvous)
	return nil
}

// FindPeers discovers other nodes announcing a specific rendezvous string.
// Self-connections and peers with no addresses are filtered out.
func (d *DiscoveryService) FindPeers(ctx context.Context, rendezvous string) ([]peer.AddrInfo, error) {
	routingDiscovery := routing.NewRoutingDiscovery(d.DHT)

	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvous)
	if err != nil {
		return nil, err
	}

	var peers []peer.AddrInfo
	for p := range peerChan {
		// Skip self and peers with no known addresses (they cannot be dialled).
		if p.ID == d.Host.ID() || len(p.Addrs) == 0 {
			continue
		}
		peers = append(peers, p)
	}
	return peers, nil
}
