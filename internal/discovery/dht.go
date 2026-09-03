package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routing2 "github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
)

// peerCacheTTL is the duration for which discovered DHT peers are cached.
// Caching eliminates expensive DHT round-trips for consecutive SOCKS5 connections.
const peerCacheTTL = 45 * time.Second

type peerCacheEntry struct {
	peers     []peer.AddrInfo
	timestamp time.Time
}

// DiscoveryService wraps a libp2p host and a Kademlia DHT for peer discovery.
type DiscoveryService struct {
	Host       host.Host
	DHT        *dht.IpfsDHT
	peerCache  map[string]peerCacheEntry
	cacheMutex sync.RWMutex
}

// NewDiscoveryService initialises a new libp2p host, bootstraps the Kad DHT,
// and optionally connects to a list of well-known bootstrap peers.
//
// Pass listenPort=0 to let the OS assign a free port (recommended for mobile).
// Pass bootstrapPeers=nil to skip bootstrap (useful for unit tests).
func NewDiscoveryService(ctx context.Context, listenPort int, bootstrapPeers []string) (*DiscoveryService, error) {
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)

	var idht *dht.IpfsDHT

	h, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.Routing(func(h host.Host) (routing2.PeerRouting, error) {
			var err error
			// ModeAuto dynamically adjusts between DHT client and full routing server,
			// saving significant CPU and battery on client devices behind NAT.
			idht, err = dht.New(ctx, h, dht.Mode(dht.ModeAuto))
			return idht, err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	log.Printf("Host created: ID=%s, listening on %s", h.ID(), listenAddr)

	// Connect to bootstrap peers concurrently with bounded timeout for fast startup.
	if len(bootstrapPeers) > 0 {
		connectToBootstrapPeers(ctx, h, bootstrapPeers)
	}

	if err := idht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	return &DiscoveryService{
		Host:      h,
		DHT:       idht,
		peerCache: make(map[string]peerCacheEntry),
	}, nil
}

// connectToBootstrapPeers dials all bootstrap peers in parallel with a 10s timeout
// so unreachable or slow peers do not delay overall node startup.
func connectToBootstrapPeers(ctx context.Context, h host.Host, bootstrapPeers []string) {
	bootCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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
		go func(pi peer.AddrInfo) {
			defer wg.Done()
			if err := h.Connect(bootCtx, pi); err != nil {
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
// Results are cached in-memory with a 45s TTL to prevent DHT query saturation
// on bursty connection requests (e.g. web browser page loads).
func (d *DiscoveryService) FindPeers(ctx context.Context, rendezvous string) ([]peer.AddrInfo, error) {
	// Check cache first
	d.cacheMutex.RLock()
	entry, found := d.peerCache[rendezvous]
	if found && time.Since(entry.timestamp) < peerCacheTTL && len(entry.peers) > 0 {
		cached := make([]peer.AddrInfo, len(entry.peers))
		copy(cached, entry.peers)
		d.cacheMutex.RUnlock()
		return cached, nil
	}
	d.cacheMutex.RUnlock()

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

	// Update cache
	d.cacheMutex.Lock()
	d.peerCache[rendezvous] = peerCacheEntry{
		peers:     peers,
		timestamp: time.Now(),
	}
	d.cacheMutex.Unlock()

	res := make([]peer.AddrInfo, len(peers))
	copy(res, peers)
	return res, nil
}

// InvalidatePeer removes a failed peer from the in-memory cache so subsequent dials
// do not repeatedly attempt to route through a dead or unreachable node.
func (d *DiscoveryService) InvalidatePeer(rendezvous string, id peer.ID) {
	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	entry, found := d.peerCache[rendezvous]
	if !found {
		return
	}

	filtered := make([]peer.AddrInfo, 0, len(entry.peers))
	for _, p := range entry.peers {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	entry.peers = filtered
	d.peerCache[rendezvous] = entry
}
