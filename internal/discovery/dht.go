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

type DiscoveryService struct {
	Host host.Host
	DHT  *dht.IpfsDHT
}

// NewDiscoveryService initializes a new libp2p host and connects to the Kad DHT.
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
		return nil, err
	}

	log.Printf("Host created with ID: %s, listening on %s", h.ID().String(), listenAddr)

	if len(bootstrapPeers) > 0 {
		var wg sync.WaitGroup
		for _, peerAddr := range bootstrapPeers {
			addr, err := multiaddr.NewMultiaddr(peerAddr)
			if err != nil {
				log.Printf("Invalid bootstrap multiaddr %s: %s", peerAddr, err)
				continue
			}
			peerinfo, err := peer.AddrInfoFromP2pAddr(addr)
			if err != nil {
				log.Printf("Invalid peerinfo %s: %s", peerAddr, err)
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := h.Connect(ctx, *peerinfo); err != nil {
					log.Printf("Failed to connect to bootstrap node %s: %s", peerinfo.ID, err)
				} else {
					log.Printf("Connected to bootstrap node: %s", peerinfo.ID)
				}
			}()
		}
		wg.Wait()
	}

	if err := idht.Bootstrap(ctx); err != nil {
		return nil, err
	}

	return &DiscoveryService{
		Host: h,
		DHT:  idht,
	}, nil
}

// Announce presence in the DHT for a specific rendezvous point
func (d *DiscoveryService) Announce(ctx context.Context, rendezvous string) error {
	routingDiscovery := routing.NewRoutingDiscovery(d.DHT)
	// Bug 8 Fix: Advertise returns a TTL and error — capture them both.
	_, err := routingDiscovery.Advertise(ctx, rendezvous)
	if err != nil {
		log.Printf("Warning: Failed to advertise on DHT for '%s': %v", rendezvous, err)
		return err
	}
	log.Printf("Announcing presence on rendezvous: %s", rendezvous)
	return nil
}

// FindPeers discovers other nodes announcing a specific rendezvous string.
func (d *DiscoveryService) FindPeers(ctx context.Context, rendezvous string) ([]peer.AddrInfo, error) {
	routingDiscovery := routing.NewRoutingDiscovery(d.DHT)
	
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvous)
	if err != nil {
		return nil, err
	}

	var peers []peer.AddrInfo
	for p := range peerChan {
		if p.ID == d.Host.ID() || len(p.Addrs) == 0 {
			continue
		}
		peers = append(peers, p)
	}
	return peers, nil
}
