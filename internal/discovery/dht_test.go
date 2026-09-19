package discovery

import (
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// makeTestPeer creates a mock peer.AddrInfo for testing.
func makeTestPeer(t *testing.T, idStr, addrStr string) peer.AddrInfo {
	t.Helper()
	id, err := peer.Decode(idStr)
	if err != nil {
		t.Fatalf("failed to decode peer id: %v", err)
	}
	ma, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		t.Fatalf("failed to parse multiaddr: %v", err)
	}
	return peer.AddrInfo{
		ID:    id,
		Addrs: []multiaddr.Multiaddr{ma},
	}
}

// TestPeerCache_InvalidatePeer verifies that InvalidatePeer removes the targeted peer ID
// from the in-memory cache and leaves other peers unaffected.
func TestPeerCache_InvalidatePeer(t *testing.T) {
	// Standard valid libp2p test IDs
	p1 := makeTestPeer(t, "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", "/ip4/127.0.0.1/tcp/9001")
	p2 := makeTestPeer(t, "QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa", "/ip4/127.0.0.1/tcp/9002")

	ds := &DiscoveryService{
		peerCache: make(map[string]peerCacheEntry),
	}

	rendezvous := "shadowlink-relay"
	ds.peerCache[rendezvous] = peerCacheEntry{
		peers:     []peer.AddrInfo{p1, p2},
		timestamp: time.Now(),
	}

	// Invalidate p1
	ds.InvalidatePeer(rendezvous, p1.ID)

	ds.cacheMutex.RLock()
	entry := ds.peerCache[rendezvous]
	ds.cacheMutex.RUnlock()

	if len(entry.peers) != 1 {
		t.Fatalf("expected 1 cached peer after eviction, got %d", len(entry.peers))
	}
	if entry.peers[0].ID != p2.ID {
		t.Errorf("expected peer %s to remain in cache, got %s", p2.ID, entry.peers[0].ID)
	}

	// Invalidating a nonexistent rendezvous key should not panic or error
	ds.InvalidatePeer("nonexistent-key", p1.ID)
}

// TestPeerCache_TTL verifies cache expiration logic.
func TestPeerCache_TTL(t *testing.T) {
	p1 := makeTestPeer(t, "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", "/ip4/127.0.0.1/tcp/9001")

	ds := &DiscoveryService{
		peerCache: make(map[string]peerCacheEntry),
	}

	rendezvous := "shadowlink-exit"

	// Add an expired cache entry (older than peerCacheTTL)
	ds.peerCache[rendezvous] = peerCacheEntry{
		peers:     []peer.AddrInfo{p1},
		timestamp: time.Now().Add(-50 * time.Second),
	}

	ds.cacheMutex.RLock()
	entry, found := ds.peerCache[rendezvous]
	isExpired := !found || time.Since(entry.timestamp) >= peerCacheTTL || len(entry.peers) == 0
	ds.cacheMutex.RUnlock()

	if !isExpired {
		t.Error("cache entry older than peerCacheTTL should be expired")
	}

	// Add a fresh cache entry
	ds.peerCache[rendezvous] = peerCacheEntry{
		peers:     []peer.AddrInfo{p1},
		timestamp: time.Now(),
	}

	ds.cacheMutex.RLock()
	entryFresh, foundFresh := ds.peerCache[rendezvous]
	isFresh := foundFresh && time.Since(entryFresh.timestamp) < peerCacheTTL && len(entryFresh.peers) > 0
	ds.cacheMutex.RUnlock()

	if !isFresh {
		t.Error("cache entry within peerCacheTTL should be fresh")
	}
}

// TestPeerCache_ConcurrentAccess verifies thread-safety under heavy concurrent reads and invalidations.
func TestPeerCache_ConcurrentAccess(t *testing.T) {
	p1 := makeTestPeer(t, "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", "/ip4/127.0.0.1/tcp/9001")
	p2 := makeTestPeer(t, "QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa", "/ip4/127.0.0.1/tcp/9002")

	ds := &DiscoveryService{
		peerCache: make(map[string]peerCacheEntry),
	}

	rendezvous := "shadowlink-relay"
	ds.peerCache[rendezvous] = peerCacheEntry{
		peers:     []peer.AddrInfo{p1, p2},
		timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ds.cacheMutex.RLock()
			_ = ds.peerCache[rendezvous]
			ds.cacheMutex.RUnlock()
		}()
		go func(iteration int) {
			defer wg.Done()
			if iteration%2 == 0 {
				ds.InvalidatePeer(rendezvous, p1.ID)
			} else {
				ds.InvalidatePeer(rendezvous, p2.ID)
			}
		}(i)
	}
	wg.Wait()
}
