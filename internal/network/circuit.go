package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"context"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/crypto"
	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/onion"
)

// DialCircuit builds a multi-hop encrypted circuit through the dVPN network.
//
// Circuit selection logic:
//   - If relay nodes are available: Entry → Relay → Exit (3-hop, preferred)
//   - If no relays are available:   Entry → Exit (1-hop, fallback)
//
// Relay and exit peers are selected randomly (Fisher-Yates shuffle) to prevent
// a traffic analysis attack where a passive observer could predict the routing path.
//
// Each hop uses an independent, ephemeral X25519 ECDH session key derived via HKDF.
// Forward secrecy is guaranteed — every connection uses a unique key.
func DialCircuit(ctx context.Context, ds *discovery.DiscoveryService, targetNetwork, targetAddr string) (net.Conn, error) {
	log.Printf("DialCircuit: building circuit for %s", targetAddr)

	exits, err := ds.FindPeers(ctx, config.RendezvousExit)
	if err != nil || len(exits) == 0 {
		return nil, fmt.Errorf("no exit nodes found in DHT: %v", err)
	}

	// Shuffle exit and relay lists before iterating to randomise routing.
	rand.Shuffle(len(exits), func(i, j int) { exits[i], exits[j] = exits[j], exits[i] })

	// Prefer 3-hop routing through a relay node.
	relays, _ := ds.FindPeers(ctx, config.RendezvousRelay)
	if len(relays) > 0 {
		rand.Shuffle(len(relays), func(i, j int) { relays[i], relays[j] = relays[j], relays[i] })
		log.Printf("Found %d relay(s) and %d exit(s) — attempting 3-hop circuit", len(relays), len(exits))
		conn, err := dialViaRelay(ctx, ds, relays, exits, targetAddr)
		if err == nil {
			return conn, nil
		}
		log.Printf("3-hop routing failed (%v), falling back to direct Entry→Exit circuit", err)
	} else {
		log.Printf("No relay nodes found — using direct Entry→Exit circuit")
	}

	// Fallback: direct Entry→Exit circuit.
	return dialDirect(ctx, ds, exits, targetAddr)
}

// dialViaRelay attempts a 3-hop circuit through available relay nodes.
func dialViaRelay(ctx context.Context, ds *discovery.DiscoveryService, relays, exits []peer.AddrInfo, targetAddr string) (net.Conn, error) {
	var lastErr error
	for _, relay := range relays {
		for _, exit := range exits {
			if relay.ID == exit.ID {
				continue // Avoid using the same node as both relay and exit
			}
			conn, err := tryViaRelay(ctx, ds, relay, exit, targetAddr)
			if err != nil {
				log.Printf("Relay %s -> Exit %s failed: %v", relay.ID, exit.ID, err)
				lastErr = err
				continue
			}
			log.Printf("3-hop circuit established: Entry -> %s -> %s -> Target", relay.ID, exit.ID)
			return conn, nil
		}
	}
	return nil, fmt.Errorf("all relay/exit combinations failed: %v", lastErr)
}

// tryViaRelay connects to a relay, extends to an exit, and negotiates keys with both.
func tryViaRelay(ctx context.Context, ds *discovery.DiscoveryService, relay, exit peer.AddrInfo, targetAddr string) (net.Conn, error) {
	if err := ds.Host.Connect(ctx, relay); err != nil {
		return nil, fmt.Errorf("connect to relay: %v", err)
	}
	stream, err := ds.Host.NewStream(ctx, relay.ID, config.ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("open stream to relay: %v", err)
	}
	cleanup := true
	defer func() {
		if cleanup && stream != nil {
			stream.Reset()
		}
	}()

	// Signal EXTEND mode and provide the exit peer ID.
	if _, err := fmt.Fprintf(stream, "%s\n%s\n", config.ExtendHeader, exit.ID.String()); err != nil {
		return nil, fmt.Errorf("write extend header: %v", err)
	}

	// ECDH with relay — initiator sends its public key first.
	relayKey, err := crypto.PerformECDH(stream)
	if err != nil {
		return nil, fmt.Errorf("ECDH with relay: %v", err)
	}

	relayConn := &libP2PConn{Conn: streamAdapter{stream}, Keys: [][]byte{relayKey}}

	// Connect to Exit through Relay
	if _, err := fmt.Fprintf(relayConn, "%s\n%s\n", config.ConnectHeader, targetAddr); err != nil {
		return nil, fmt.Errorf("write connect header: %v", err)
	}

	// ECDH with Exit (through the relay proxy)
	exitKey, err := crypto.PerformECDH(relayConn)
	if err != nil {
		return nil, fmt.Errorf("ECDH with exit: %v", err)
	}

	exitConn := &libP2PConn{Conn: relayConn, Keys: [][]byte{exitKey}}
	cleanup = false
	return exitConn, nil
}

// dialDirect builds a 1-hop Entry→Exit circuit.
func dialDirect(ctx context.Context, ds *discovery.DiscoveryService, exits []peer.AddrInfo, targetAddr string) (net.Conn, error) {
	var lastErr error
	for _, exitNode := range exits {
		conn, err := tryDirect(ctx, ds, exitNode, targetAddr)
		if err != nil {
			log.Printf("Exit node %s failed: %v, trying next...", exitNode.ID, err)
			lastErr = err
			continue
		}
		log.Printf("Direct circuit established to exit %s", exitNode.ID)
		return conn, nil
	}
	return nil, fmt.Errorf("all %d exit node(s) failed: %v", len(exits), lastErr)
}

// tryDirect attempts a direct Entry→Exit stream.
func tryDirect(ctx context.Context, ds *discovery.DiscoveryService, exitNode peer.AddrInfo, targetAddr string) (net.Conn, error) {
	if err := ds.Host.Connect(ctx, exitNode); err != nil {
		return nil, fmt.Errorf("connect to exit: %v", err)
	}
	stream, err := ds.Host.NewStream(ctx, exitNode.ID, config.ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("open stream to exit: %v", err)
	}
	cleanup := true
	defer func() {
		if cleanup && stream != nil {
			stream.Reset()
		}
	}()

	if _, err := fmt.Fprintf(stream, "%s\n%s\n", config.ConnectHeader, targetAddr); err != nil {
		return nil, fmt.Errorf("write target addr: %v", err)
	}

	sessionKey, err := crypto.PerformECDH(stream)
	if err != nil {
		return nil, fmt.Errorf("ECDH with exit: %v", err)
	}

	cleanup = false
	return &libP2PConn{Conn: streamAdapter{stream}, Keys: [][]byte{sessionKey}}, nil
}

// streamAdapter adapts a libp2p network.Stream to the net.Conn interface.
type streamAdapter struct {
	libp2pnet.Stream
}

func (s streamAdapter) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (s streamAdapter) RemoteAddr() net.Addr { return &net.TCPAddr{} }

// libP2PConn wraps a net.Conn to provide multi-layered encryption/decryption (Onion Routing)
// and frame sizing.
type libP2PConn struct {
	net.Conn
	Keys     [][]byte
	readBuf  []byte
	frameBuf []byte // reusable buffer to avoid allocations
	lenBuf   [4]byte
}

func (c *libP2PConn) Read(b []byte) (int, error) {
	// Serve buffered plaintext from a previous partial read if available.
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// Step 1: Read the 4-byte big-endian frame length.
	if _, err := io.ReadFull(c.Conn, c.lenBuf[:]); err != nil {
		return 0, err
	}
	frameLen := binary.BigEndian.Uint32(c.lenBuf[:])

	// Security: enforce max frame size (from config) to prevent OOM DoS.
	if frameLen > config.MaxFrameSize {
		return 0, fmt.Errorf("frame length %d exceeds max frame size %d", frameLen, config.MaxFrameSize)
	}

	// Step 2: Read exactly frameLen bytes. Reuse frameBuf to avoid allocations.
	if uint32(cap(c.frameBuf)) < frameLen {
		c.frameBuf = make([]byte, frameLen)
	}
	frame := c.frameBuf[:frameLen]
	if _, err := io.ReadFull(c.Conn, frame); err != nil {
		return 0, err
	}

	// Step 3: Decrypt the full frame using the per-session keys (Onion Unwrap).
	plaintext := frame
	var err error
	for _, key := range c.Keys {
		plaintext, err = onion.UnwrapPayload(plaintext, key)
		if err != nil {
			return 0, fmt.Errorf("decryption failed: %v", err)
		}
	}

	// Step 4: Copy plaintext into the caller's buffer, saving any remainder.
	n := copy(b, plaintext)
	if n < len(plaintext) {
		// Allocate a new buffer for the remainder, since plaintext shares memory with frameBuf
		c.readBuf = make([]byte, len(plaintext)-n)
		copy(c.readBuf, plaintext[n:])
	}

	return n, nil
}

// Write encrypts the payload with the per-session keys and prepends a 4-byte length header.
func (c *libP2PConn) Write(b []byte) (int, error) {
	ciphertext, err := onion.WrapPayload(b, c.Keys)
	if err != nil {
		return 0, err
	}

	// Prepend 4-byte big-endian length header.
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(ciphertext)))

	if _, err := c.Conn.Write(lenBuf[:]); err != nil {
		return 0, err
	}
	if _, err := c.Conn.Write(ciphertext); err != nil {
		return 0, err
	}
	// Return the plaintext length to satisfy the net.Conn contract.
	return len(b), nil
}
