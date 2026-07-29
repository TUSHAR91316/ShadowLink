package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"context"

	"github.com/libp2p/go-libp2p/core/network"
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
		conn, err := dialViaRelay(ctx, ds, relays, targetAddr)
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
// Tries each relay in order until one succeeds or all are exhausted.
func dialViaRelay(ctx context.Context, ds *discovery.DiscoveryService, relays []peer.AddrInfo, targetAddr string) (net.Conn, error) {
	var lastErr error
	for _, relay := range relays {
		conn, err := tryViaRelay(ctx, ds, relay, targetAddr)
		if err != nil {
			log.Printf("Relay %s failed: %v, trying next...", relay.ID, err)
			lastErr = err
			continue
		}
		log.Printf("3-hop circuit established via relay %s", relay.ID)
		return conn, nil
	}
	return nil, fmt.Errorf("all %d relay(s) failed: %v", len(relays), lastErr)
}

// tryViaRelay connects to a specific relay and negotiates the circuit.
//
// Protocol:
//  1. Open a libp2p stream to the relay
//  2. Write config.RelayHeader + "\n", then the target address + "\n"
//  3. Perform X25519 ECDH as the initiator (send public key first)
//  4. Relay independently connects to an exit node and bridges the tunnel
//  5. Return a libP2PConn encrypted with the per-session ECDH-HKDF key
func tryViaRelay(ctx context.Context, ds *discovery.DiscoveryService, relay peer.AddrInfo, targetAddr string) (net.Conn, error) {
	if err := ds.Host.Connect(ctx, relay); err != nil {
		return nil, fmt.Errorf("connect to relay: %v", err)
	}
	stream, err := ds.Host.NewStream(ctx, relay.ID, config.ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("open stream to relay: %v", err)
	}

	// Signal relay mode and provide the target address.
	if _, err := fmt.Fprintf(stream, "%s\n%s\n", config.RelayHeader, targetAddr); err != nil {
		stream.Reset()
		return nil, fmt.Errorf("write relay header: %v", err)
	}

	// ECDH key exchange — initiator sends its public key first.
	sessionKey, err := crypto.PerformECDH(stream)
	if err != nil {
		stream.Reset()
		return nil, fmt.Errorf("ECDH with relay: %v", err)
	}

	return &libP2PConn{Stream: stream, Keys: [][]byte{sessionKey}}, nil
}

// dialDirect builds a 1-hop Entry→Exit circuit (fallback when no relays are available).
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
//
// Protocol:
//  1. Open a libp2p stream to the exit node
//  2. Write the target address (no config.RelayHeader prefix — exit detects direct mode)
//  3. Perform X25519 ECDH as the initiator
//  4. Return a libP2PConn encrypted with the per-session ECDH-HKDF key
func tryDirect(ctx context.Context, ds *discovery.DiscoveryService, exitNode peer.AddrInfo, targetAddr string) (net.Conn, error) {
	if err := ds.Host.Connect(ctx, exitNode); err != nil {
		return nil, fmt.Errorf("connect to exit: %v", err)
	}
	stream, err := ds.Host.NewStream(ctx, exitNode.ID, config.ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("open stream to exit: %v", err)
	}

	// Write target address — no RelayHeader signals direct exit mode.
	if _, err := fmt.Fprintf(stream, "%s\n", targetAddr); err != nil {
		stream.Reset()
		return nil, fmt.Errorf("write target addr: %v", err)
	}

	// ECDH key exchange — initiator sends its public key first.
	sessionKey, err := crypto.PerformECDH(stream)
	if err != nil {
		stream.Reset()
		return nil, fmt.Errorf("ECDH with exit: %v", err)
	}

	return &libP2PConn{Stream: stream, Keys: [][]byte{sessionKey}}, nil
}

// libP2PConn wraps a libp2p network.Stream to implement net.Conn.
// All traffic is encrypted with the per-session ECDH-HKDF key and framed
// with a 4-byte big-endian length prefix for reliable stream parsing.
type libP2PConn struct {
	network.Stream
	Keys    [][]byte
	readBuf []byte
}

func (c *libP2PConn) Read(b []byte) (int, error) {
	// Serve buffered plaintext from a previous partial read if available.
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// Step 1: Read the 4-byte big-endian frame length.
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.Stream, lenBuf); err != nil {
		return 0, err
	}
	frameLen := binary.BigEndian.Uint32(lenBuf)

	// Security: enforce max frame size (from config) to prevent OOM DoS.
	if frameLen > config.MaxFrameSize {
		return 0, fmt.Errorf("frame length %d exceeds max frame size %d", frameLen, config.MaxFrameSize)
	}

	// Step 2: Read exactly frameLen bytes (the complete encrypted frame).
	frame := make([]byte, frameLen)
	if _, err := io.ReadFull(c.Stream, frame); err != nil {
		return 0, err
	}

	// Step 3: Decrypt the full frame using the per-session key.
	plaintext, err := onion.UnwrapPayload(frame, c.Keys[0])
	if err != nil {
		return 0, fmt.Errorf("decryption failed: %v", err)
	}

	// Step 4: Copy plaintext into the caller's buffer, saving any remainder.
	n := copy(b, plaintext)
	if n < len(plaintext) {
		c.readBuf = plaintext[n:]
	}

	return n, nil
}

// Write encrypts the payload with the per-session key and prepends a 4-byte length header.
// The matching length prefix ensures the reader always consumes exactly one complete frame.
func (c *libP2PConn) Write(b []byte) (int, error) {
	ciphertext, err := onion.WrapPayload(b, c.Keys)
	if err != nil {
		return 0, err
	}

	// Prepend 4-byte big-endian length header.
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(ciphertext)))

	if _, err := c.Stream.Write(lenBuf); err != nil {
		return 0, err
	}
	if _, err := c.Stream.Write(ciphertext); err != nil {
		return 0, err
	}
	// Return the plaintext length to satisfy the net.Conn contract.
	return len(b), nil
}

// LocalAddr returns a placeholder address. libp2p streams are not TCP connections;
// callers that require real addresses should use the libp2p host API directly.
func (c *libP2PConn) LocalAddr() net.Addr { return &net.TCPAddr{} }

// RemoteAddr returns a placeholder address. See LocalAddr for the same caveat.
func (c *libP2PConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }
