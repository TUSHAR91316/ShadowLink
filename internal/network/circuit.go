package network

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"sync"
	"time"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/crypto"
	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/onion"
)

type relayConnectError struct {
	err error
}

func (e *relayConnectError) Error() string { return e.err.Error() }
func (e *relayConnectError) Unwrap() error { return e.err }

type exitConnectError struct {
	err error
}

func (e *exitConnectError) Error() string { return e.err.Error() }
func (e *exitConnectError) Unwrap() error { return e.err }

// handshakeTimeout bounds circuit negotiation so unresponsive peers fail fast.
const handshakeTimeout = 15 * time.Second

// cryptoShuffle performs a cryptographically secure Fisher-Yates shuffle using crypto/rand
// to eliminate selection bias and prevent passive traffic analysis attacks against routing paths.
func cryptoShuffle[T any](slice []T) {
	for i := len(slice) - 1; i > 0; i-- {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			continue
		}
		j := int(nBig.Int64())
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// DialCircuit builds a multi-hop encrypted circuit through the dVPN network.
//
// Circuit selection logic:
//   - If relay nodes are available: Entry → Relay → Exit (3-hop, preferred)
//   - If no relays are available:   Entry → Exit (1-hop, fallback)
//
// Relay and exit peers are selected using cryptographically secure Fisher-Yates
// shuffling to prevent a traffic analysis attack where a passive observer could
// predict the routing path.
//
// The 3-hop path uses true Onion Routing: the Entry node independently negotiates
// ECDH session keys with both the Relay and the Exit node. The Relay never sees
// plaintext — it only strips one layer of encryption before forwarding.
func DialCircuit(ctx context.Context, ds *discovery.DiscoveryService, targetNetwork, targetAddr string) (net.Conn, error) {
	log.Printf("DialCircuit: building circuit for %s", targetAddr)

	// Concurrently query exit and relay nodes to halve initial circuit discovery latency.
	var exits, relays []peer.AddrInfo
	var exitErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		exits, exitErr = ds.FindPeers(ctx, config.RendezvousExit)
	}()
	go func() {
		defer wg.Done()
		relays, _ = ds.FindPeers(ctx, config.RendezvousRelay)
	}()
	wg.Wait()

	if exitErr != nil || len(exits) == 0 {
		return nil, fmt.Errorf("no exit nodes found in DHT: %w", exitErr)
	}
	// Cryptographically shuffle exit list to randomise routing.
	cryptoShuffle(exits)

	// Prefer 3-hop routing through a relay node.
	if len(relays) > 0 {
		cryptoShuffle(relays)
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

// dialViaRelay attempts a 3-hop circuit through available relay/exit node combinations.
// It iterates over all (relay, exit) pairs (skipping same-node combos) until one succeeds.
func dialViaRelay(ctx context.Context, ds *discovery.DiscoveryService, relays, exits []peer.AddrInfo, targetAddr string) (net.Conn, error) {
	var lastErr error
	for _, relay := range relays {
		relayDead := false
		for _, exit := range exits {
			if relay.ID == exit.ID {
				continue // Never use the same node as both relay and exit
			}
			conn, err := tryViaRelay(ctx, ds, relay, exit, targetAddr)
			if err != nil {
				log.Printf("Relay %s -> Exit %s failed: %v", relay.ID, exit.ID, err)
				lastErr = err

				var rErr *relayConnectError
				var eErr *exitConnectError
				if errors.As(err, &rErr) {
					// The relay node itself is unresponsive; invalidate it and break the inner loop
					// so we don't waste time trying all other exit nodes with this dead relay.
					ds.InvalidatePeer(config.RendezvousRelay, relay.ID)
					relayDead = true
					break
				} else if errors.As(err, &eErr) {
					// The exit node failed; invalidate the exit and try the next exit with this relay.
					ds.InvalidatePeer(config.RendezvousExit, exit.ID)
				} else {
					// Fallback invalidation
					ds.InvalidatePeer(config.RendezvousRelay, relay.ID)
				}
				continue
			}
			log.Printf("3-hop circuit established: Entry -> %s -> %s -> Target", relay.ID, exit.ID)
			return conn, nil
		}
		if relayDead {
			continue
		}
	}
	return nil, fmt.Errorf("all relay/exit combinations failed: %w", lastErr)
}

// tryViaRelay builds a true 3-hop onion circuit: Entry → Relay → Exit.
//
// Protocol:
//  1. Connect to the relay and send "EXTEND\n<ExitPeerID>\n" in plaintext.
//  2. ECDH with the relay (Entry = INITIATOR) → relayKey.
//  3. The relay connects to the exit and bridges the raw byte stream transparently.
//  4. Entry sends "CONNECT\n<TargetAddr>\n" encrypted with relayKey through relayConn.
//  5. ECDH with the exit through the relay → exitKey.
//  6. Return a nested libP2PConn: inner is relayConn (relayKey), outer adds exitKey.
func tryViaRelay(ctx context.Context, ds *discovery.DiscoveryService, relay, exit peer.AddrInfo, targetAddr string) (net.Conn, error) {
	if err := ds.Host.Connect(ctx, relay); err != nil {
		return nil, &relayConnectError{err: fmt.Errorf("connect to relay: %w", err)}
	}
	stream, err := ds.Host.NewStream(ctx, relay.ID, config.ProtocolID)
	if err != nil {
		return nil, &relayConnectError{err: fmt.Errorf("open stream to relay: %w", err)}
	}

	// Set deadline during circuit handshake so unresponsive peers fail fast.
	_ = stream.SetDeadline(time.Now().Add(handshakeTimeout))

	// resetOnError ensures the underlying stream is always torn down on failure.
	success := false
	defer func() {
		if !success {
			stream.Reset()
		}
	}()

	// Step 1: Tell the relay which exit peer to extend to.
	if _, err := fmt.Fprintf(stream, "%s\n%s\n", config.ExtendHeader, exit.ID.String()); err != nil {
		return nil, &relayConnectError{err: fmt.Errorf("write EXTEND header: %w", err)}
	}

	// Step 2: ECDH with the relay.
	relayKey, err := crypto.PerformECDH(stream)
	if err != nil {
		return nil, &relayConnectError{err: fmt.Errorf("ECDH with relay: %w", err)}
	}

	// relayConn: all traffic is encrypted with relayKey before hitting the wire.
	relayConn := newLibP2PConn(streamAdapter{stream}, [][]byte{relayKey})

	// Step 3: Send the CONNECT command encrypted through the relay tunnel.
	if _, err := fmt.Fprintf(relayConn, "%s\n%s\n", config.ConnectHeader, targetAddr); err != nil {
		return nil, &exitConnectError{err: fmt.Errorf("write CONNECT header: %w", err)}
	}

	// Step 4: ECDH with the exit, proxied transparently through the relay.
	exitKey, err := crypto.PerformECDH(relayConn)
	if err != nil {
		return nil, &exitConnectError{err: fmt.Errorf("ECDH with exit: %w", err)}
	}

	// Clear handshake deadline for normal streaming transfer.
	_ = stream.SetDeadline(time.Time{})

	// Step 5: Nested conn — outer exitKey wrap sits on top of inner relayKey wrap.
	exitConn := newLibP2PConn(relayConn, [][]byte{exitKey})
	success = true
	return exitConn, nil
}

// dialDirect builds a 1-hop Entry→Exit circuit (fallback when no relays are available).
func dialDirect(ctx context.Context, ds *discovery.DiscoveryService, exits []peer.AddrInfo, targetAddr string) (net.Conn, error) {
	var lastErr error
	for _, exitNode := range exits {
		conn, err := tryDirect(ctx, ds, exitNode, targetAddr)
		if err != nil {
			log.Printf("Exit node %s failed: %v, trying next...", exitNode.ID, err)
			lastErr = err
			ds.InvalidatePeer(config.RendezvousExit, exitNode.ID)
			continue
		}
		log.Printf("Direct circuit established to exit %s", exitNode.ID)
		return conn, nil
	}
	return nil, fmt.Errorf("all %d exit node(s) failed: %w", len(exits), lastErr)
}

// tryDirect opens a 1-hop encrypted stream directly to an exit node.
func tryDirect(ctx context.Context, ds *discovery.DiscoveryService, exitNode peer.AddrInfo, targetAddr string) (net.Conn, error) {
	if err := ds.Host.Connect(ctx, exitNode); err != nil {
		return nil, fmt.Errorf("connect to exit: %w", err)
	}
	stream, err := ds.Host.NewStream(ctx, exitNode.ID, config.ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("open stream to exit: %w", err)
	}

	_ = stream.SetDeadline(time.Now().Add(handshakeTimeout))

	success := false
	defer func() {
		if !success {
			stream.Reset()
		}
	}()

	if _, err := fmt.Fprintf(stream, "%s\n%s\n", config.ConnectHeader, targetAddr); err != nil {
		return nil, fmt.Errorf("write CONNECT header: %w", err)
	}

	sessionKey, err := crypto.PerformECDH(stream)
	if err != nil {
		return nil, fmt.Errorf("ECDH with exit: %w", err)
	}

	// Clear handshake deadline
	_ = stream.SetDeadline(time.Time{})

	success = true
	return newLibP2PConn(streamAdapter{stream}, [][]byte{sessionKey}), nil
}

// ─── Stream / Conn Adapters ──────────────────────────────────────────────────

// streamAdapter makes a libp2p network.Stream satisfy net.Conn by adding
// stub LocalAddr/RemoteAddr methods. libp2p streams expose full I/O but
// intentionally omit these TCP-centric fields.
type streamAdapter struct {
	libp2pnet.Stream
}

func (s streamAdapter) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (s streamAdapter) RemoteAddr() net.Addr { return &net.TCPAddr{} }

// ─── libP2PConn ──────────────────────────────────────────────────────────────

// libP2PConn wraps any net.Conn to provide layered onion encryption and
// 4-byte big-endian length-prefix framing.
//
// Key optimizations & thread-safety:
//   - Pre-instantiated cipher.AEAD instances eliminate allocations per packet.
//   - Separate readMu and writeMu enable full-duplex I/O without lock contention.
//   - Reused frameBuf, writeBuf, and scratchBuf enable zero-allocation read and write pipelines.
//   - Stack-allocated lenBuf eliminates shared struct state and concurrent clobbering.
type libP2PConn struct {
	net.Conn
	Keys       [][]byte
	ciphers    []cipher.AEAD
	onceInit   sync.Once
	readMu     sync.Mutex
	readBuf    []byte
	frameBuf   []byte // reused across reads to eliminate per-frame heap allocations
	writeMu    sync.Mutex
	writeBuf   []byte // reused across writes to eliminate per-frame heap allocations
	scratchBuf []byte // reused across multi-layer writes for zero-allocation ping-pong
}

// newLibP2PConn constructs a libP2PConn with pre-instantiated ciphers.
func newLibP2PConn(conn net.Conn, keys [][]byte) *libP2PConn {
	c := &libP2PConn{
		Conn: conn,
		Keys: keys,
	}
	c.initCiphers()
	return c
}

func (c *libP2PConn) initCiphers() {
	c.onceInit.Do(func() {
		ciphers := make([]cipher.AEAD, len(c.Keys))
		for i, k := range c.Keys {
			if len(k) == chacha20poly1305.KeySize {
				aead, err := chacha20poly1305.NewX(k)
				if err == nil {
					ciphers[i] = aead
				}
			}
		}
		c.ciphers = ciphers
	})
}

// Read decrypts the next onion frame and copies the plaintext into b.
// Partial reads are buffered in readBuf and served on subsequent calls.
func (c *libP2PConn) Read(b []byte) (int, error) {
	c.initCiphers()

	c.readMu.Lock()
	defer c.readMu.Unlock()

	// Drain any leftover plaintext from a prior partial read.
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// Read the 4-byte big-endian frame length using a stack-allocated buffer.
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.Conn, lenBuf[:]); err != nil {
		return 0, err
	}
	frameLen := binary.BigEndian.Uint32(lenBuf[:])

	// Enforce frame size bounds (minimum 40 bytes: 24-byte nonce + 16-byte Poly1305 AEAD tag).
	if frameLen < 40 || frameLen > config.MaxFrameSize {
		return 0, fmt.Errorf("invalid frame length %d (must be between 40 and %d): possible protocol violation", frameLen, config.MaxFrameSize)
	}

	// Grow the reusable frameBuf only when the incoming frame is larger than any seen so far.
	if uint32(cap(c.frameBuf)) < frameLen {
		c.frameBuf = make([]byte, frameLen)
	}
	frame := c.frameBuf[:frameLen]
	if _, err := io.ReadFull(c.Conn, frame); err != nil {
		return 0, err
	}

	// Peel each encryption layer in order (outermost first) using zero-allocation in-place decryption.
	plaintext := frame
	var err error
	if len(c.ciphers) > 0 {
		for i, aead := range c.ciphers {
			if aead == nil {
				plaintext, err = onion.UnwrapPayload(plaintext, c.Keys[i])
			} else {
				plaintext, err = onion.UnwrapPayloadInPlace(plaintext, aead)
			}
			if err != nil {
				return 0, fmt.Errorf("decryption layer %d failed: %w", i, err)
			}
		}
	} else {
		for _, key := range c.Keys {
			plaintext, err = onion.UnwrapPayload(plaintext, key)
			if err != nil {
				return 0, fmt.Errorf("decryption failed: %w", err)
			}
		}
	}

	n := copy(b, plaintext)
	if n < len(plaintext) {
		// plaintext aliases frameBuf; copy the tail into reusable readBuf without fresh heap allocations.
		remaining := len(plaintext) - n
		if cap(c.readBuf) < remaining {
			c.readBuf = make([]byte, remaining)
		} else {
			c.readBuf = c.readBuf[:remaining]
		}
		copy(c.readBuf, plaintext[n:])
	}
	return n, nil
}

// Write encrypts b with all session keys and sends it as a single framed message:
// [4-byte BE length][ciphertext].
//
// Optimized with reusable writeBuf and scratchBuf to eliminate heap allocations per write.
func (c *libP2PConn) Write(b []byte) (int, error) {
	c.initCiphers()

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Calculate needed buffer size for all cipher layers: 4 + len(b) + layers*(24+16)
	overheadPerLayer := 40 // 24 nonce + 16 poly1305 tag
	numLayers := len(c.Keys)
	if numLayers == 0 {
		return 0, fmt.Errorf("encryption failed: no session keys available")
	}

	needed := 4 + len(b) + numLayers*overheadPerLayer
	if cap(c.writeBuf) < needed {
		c.writeBuf = make([]byte, needed)
	}

	var ciphertext []byte
	var err error

	if len(c.ciphers) > 0 {
		if numLayers > 1 {
			scratchNeeded := len(b) + (numLayers-1)*overheadPerLayer
			if cap(c.scratchBuf) < scratchNeeded {
				c.scratchBuf = make([]byte, scratchNeeded)
			}
			ciphertext, err = onion.WrapPayloadWithBuffers(b, c.ciphers, c.writeBuf[4:4], c.scratchBuf[:0])
		} else {
			ciphertext, err = onion.WrapPayloadWithBuffers(b, c.ciphers, c.writeBuf[4:4], nil)
		}
	} else {
		ciphertext, err = onion.WrapPayload(b, c.Keys)
	}
	if err != nil {
		return 0, fmt.Errorf("encryption failed: %w", err)
	}

	totalLen := 4 + len(ciphertext)
	if cap(c.writeBuf) < totalLen {
		c.writeBuf = make([]byte, totalLen)
	}
	c.writeBuf = c.writeBuf[:totalLen]

	// Write 4-byte big-endian length prefix directly into writeBuf header.
	binary.BigEndian.PutUint32(c.writeBuf[:4], uint32(len(ciphertext)))

	if len(ciphertext) > 0 && &ciphertext[0] != &c.writeBuf[4] {
		copy(c.writeBuf[4:], ciphertext)
	}

	if _, err := c.Conn.Write(c.writeBuf); err != nil {
		return 0, err
	}

	return len(b), nil
}
