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
			log.Printf("cryptoShuffle: CSPRNG error during shuffle: %v", err)
			return
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
	// Do not log the destination: even local logs can reveal a user's browsing history.
	log.Printf("DialCircuit: building circuit")

	// Reject unencodable targets up front so they are not blamed on (and evicted
	// from the cache as) failing exit nodes.
	if err := validateTarget(targetAddr); err != nil {
		return nil, err
	}

	// Bound the whole circuit build. The SOCKS5 server passes a context with no
	// deadline, so without this a black-holed peer could stall a client forever.
	ctx, cancel := context.WithTimeout(ctx, config.DefaultConnectionTimeout)
	defer cancel()

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

	if exitErr != nil {
		return nil, fmt.Errorf("exit node lookup failed: %w", exitErr)
	}
	if len(exits) == 0 {
		return nil, errors.New("no exit nodes found in DHT")
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
		if ctx.Err() != nil {
			break
		}
		for _, exit := range exits {
			if relay.ID == exit.ID {
				continue // Never use the same node as both relay and exit
			}
			conn, err := tryViaRelay(ctx, ds, relay, exit, targetAddr)
			if err != nil {
				log.Printf("Relay %s -> Exit %s failed: %v", relay.ID, exit.ID, err)
				lastErr = err
				if ctx.Err() != nil {
					// Out of time budget — the peers are not at fault, so keep them cached.
					return nil, fmt.Errorf("circuit build aborted: %w", err)
				}

				var rErr *relayConnectError
				var eErr *exitConnectError
				if errors.As(err, &rErr) {
					// The relay node itself is unresponsive; invalidate it and break the inner loop
					// so we don't waste time trying all other exit nodes with this dead relay.
					ds.InvalidatePeer(config.RendezvousRelay, relay.ID)
					break
				} else if errors.As(err, &eErr) {
					// The exit node failed; invalidate the exit and try the next exit with this relay.
					ds.InvalidatePeer(config.RendezvousExit, exit.ID)
				} else {
					// Fallback invalidation: if unclassified, invalidate both relay and exit so we don't retry a broken path.
					ds.InvalidatePeer(config.RendezvousRelay, relay.ID)
					ds.InvalidatePeer(config.RendezvousExit, exit.ID)
				}
				continue
			}
			log.Printf("3-hop circuit established: Entry -> %s -> %s -> Target", relay.ID, exit.ID)
			return conn, nil
		}
	}
	if lastErr == nil {
		lastErr = ctx.Err()
	}
	if lastErr == nil {
		return nil, errors.New("no usable relay/exit combination (relay and exit are the same node)")
	}
	return nil, fmt.Errorf("all relay/exit combinations failed: %w", lastErr)
}

// tryViaRelay builds a true 3-hop onion circuit: Entry → Relay → Exit.
//
// Protocol:
//  1. Connect to the relay and send "EXTEND\n<ExitPeerID>\n" in plaintext.
//  2. ECDH with the relay (Entry = INITIATOR) → relayKey using HKDFInfoRelay.
//  3. The relay connects to the exit and bridges the raw byte stream transparently.
//  4. openExitSession through relayConn: "CONNECT\n", ECDH with the exit → exitKey,
//     then the target record encrypted with exitKey. The relay only ever sees the
//     bare CONNECT line and exit-encrypted bytes — never the destination.
//  5. Return a nested libP2PConn: inner is relayConn (relayKey), outer adds exitKey.
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
			_ = stream.Reset()
		}
	}()

	// Step 1: Tell the relay which exit peer to extend to.
	if _, err := fmt.Fprintf(stream, "%s\n%s\n", config.ExtendHeader, exit.ID.String()); err != nil {
		return nil, &relayConnectError{err: fmt.Errorf("write EXTEND header: %w", err)}
	}

	// Step 2: ECDH with the relay using HKDFInfoRelay.
	relayKey, err := crypto.PerformECDHWithInfo(stream, config.HKDFInfoRelay)
	if err != nil {
		return nil, &relayConnectError{err: fmt.Errorf("ECDH with relay: %w", err)}
	}

	// relayConn: all traffic is encrypted with relayKey before hitting the wire.
	relayConn := newLibP2PConn(streamAdapter{stream}, [][]byte{relayKey})

	// Steps 3-4: exit handshake proxied through the relay tunnel. The returned conn
	// nests the exitKey layer on top of the relayKey layer.
	exitConn, err := openExitSession(relayConn, targetAddr)
	if err != nil {
		return nil, &exitConnectError{err: err}
	}

	// Clear handshake deadline for normal streaming transfer.
	_ = stream.SetDeadline(time.Time{})

	success = true
	return exitConn, nil
}

// dialDirect builds a 1-hop Entry→Exit circuit (fallback when no relays are available).
func dialDirect(ctx context.Context, ds *discovery.DiscoveryService, exits []peer.AddrInfo, targetAddr string) (net.Conn, error) {
	var lastErr error
	for _, exitNode := range exits {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("circuit build aborted: %w", err)
		}
		conn, err := tryDirect(ctx, ds, exitNode, targetAddr)
		if err != nil {
			log.Printf("Exit node %s failed: %v, trying next...", exitNode.ID, err)
			lastErr = err
			if ctx.Err() != nil {
				return nil, fmt.Errorf("circuit build aborted: %w", err)
			}
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
			_ = stream.Reset()
		}
	}()

	exitConn, err := openExitSession(streamAdapter{stream}, targetAddr)
	if err != nil {
		return nil, err
	}

	// Clear handshake deadline
	_ = stream.SetDeadline(time.Time{})

	success = true
	return exitConn, nil
}

// openExitSession runs the entry side of the exit handshake over conn — the raw
// stream for a direct circuit, or the relay-encrypted tunnel for a 3-hop circuit:
//
//  1. "CONNECT\n" (no destination).
//  2. X25519 ECDH with the exit → exitKey using HKDFInfoExit.
//  3. The fixed-size target record, encrypted with exitKey.
//
// Because the destination is only ever sent under the exit's key, a relay
// bridging conn learns neither the target nor (thanks to padding) its length.
func openExitSession(conn net.Conn, targetAddr string) (*libP2PConn, error) {
	if _, err := fmt.Fprintf(conn, "%s\n", config.ConnectHeader); err != nil {
		return nil, fmt.Errorf("write CONNECT header: %w", err)
	}
	exitKey, err := crypto.PerformECDHWithInfo(conn, config.HKDFInfoExit)
	if err != nil {
		return nil, fmt.Errorf("ECDH with exit: %w", err)
	}
	exitConn := newLibP2PConn(conn, [][]byte{exitKey})
	if err := writeTargetRecord(exitConn, targetAddr); err != nil {
		return nil, fmt.Errorf("send target to exit: %w", err)
	}
	return exitConn, nil
}

// validateTarget reports whether addr fits in a target record.
func validateTarget(addr string) error {
	if addr == "" || len(addr) > config.TargetRecordSize-2 {
		return fmt.Errorf("invalid target address length %d (must be 1-%d bytes)", len(addr), config.TargetRecordSize-2)
	}
	return nil
}

// writeTargetRecord sends addr as one padded [2-byte BE length][addr][zeros] record.
func writeTargetRecord(w io.Writer, addr string) error {
	if err := validateTarget(addr); err != nil {
		return err
	}
	var rec [config.TargetRecordSize]byte
	binary.BigEndian.PutUint16(rec[:2], uint16(len(addr))) // #nosec G115 -- bounded by validateTarget
	copy(rec[2:], addr)
	_, err := w.Write(rec[:])
	return err
}

// readTargetRecord reads and decodes a record produced by writeTargetRecord.
func readTargetRecord(r io.Reader) (string, error) {
	var rec [config.TargetRecordSize]byte
	if _, err := io.ReadFull(r, rec[:]); err != nil {
		return "", fmt.Errorf("read target record: %w", err)
	}
	n := int(binary.BigEndian.Uint16(rec[:2]))
	if n == 0 || n > len(rec)-2 {
		return "", fmt.Errorf("invalid target record length %d", n)
	}
	return string(rec[2 : 2+n]), nil
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

// frameOverhead is the per-layer AEAD cost: 24-byte XChaCha20 nonce + 16-byte Poly1305 tag.
const frameOverhead = chacha20poly1305.NonceSizeX + chacha20poly1305.Overhead

// maxWriteChunk is the largest plaintext carried by a single frame. Keeping it at half
// of config.MaxFrameSize leaves room for every onion layer, so a nested conn's frame
// (chunk + its own header and overhead) still fits inside one frame of the conn below.
const maxWriteChunk = config.MaxFrameSize / 2

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
			if len(k) != chacha20poly1305.KeySize {
				panic(fmt.Sprintf("initCiphers: key %d has invalid length %d, want %d", i, len(k), chacha20poly1305.KeySize))
			}
			aead, err := chacha20poly1305.NewX(k)
			if err != nil {
				panic(fmt.Sprintf("initCiphers: failed to instantiate cipher for layer %d: %v", i, err))
			}
			ciphers[i] = aead
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

	if len(c.ciphers) == 0 {
		// Never hand raw wire bytes to the caller as if they were decrypted plaintext.
		return 0, errors.New("decryption failed: no session keys available")
	}

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

	// Enforce frame size bounds (minimum one layer: 24-byte nonce + 16-byte Poly1305 AEAD tag).
	if frameLen < frameOverhead || frameLen > config.MaxFrameSize {
		return 0, fmt.Errorf("invalid frame length %d (must be between %d and %d): possible protocol violation", frameLen, frameOverhead, config.MaxFrameSize)
	}

	// Grow the reusable frameBuf only when the incoming frame is larger than any seen so far.
	if cap(c.frameBuf) < int(frameLen) {
		c.frameBuf = make([]byte, frameLen)
	}
	frame := c.frameBuf[:frameLen]
	if _, err := io.ReadFull(c.Conn, frame); err != nil {
		return 0, err
	}

	// Peel each encryption layer in order (outermost first) using zero-allocation in-place decryption.
	plaintext := frame
	for i, aead := range c.ciphers {
		var err error
		plaintext, err = onion.UnwrapPayloadInPlace(plaintext, aead)
		if err != nil {
			return 0, fmt.Errorf("decryption layer %d failed: %w", i, err)
		}
	}

	n := copy(b, plaintext)
	if n < len(plaintext) {
		// plaintext aliases frameBuf under readMu lock; copy the tail into reusable readBuf
		// to guarantee memory safety for subsequent Read calls.
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

// Write encrypts b with all session keys and sends it as one or more framed
// messages: [4-byte BE length][ciphertext].
//
// Payloads larger than maxWriteChunk are split across several frames, because the
// receiver rejects any frame above config.MaxFrameSize as a protocol violation.
// Optimized with reusable writeBuf and scratchBuf to eliminate heap allocations per write.
func (c *libP2PConn) Write(b []byte) (int, error) {
	c.initCiphers()

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if len(c.ciphers) == 0 {
		return 0, errors.New("encryption failed: no session keys available")
	}

	written := 0
	for written < len(b) {
		chunk := b[written:]
		if len(chunk) > maxWriteChunk {
			chunk = chunk[:maxWriteChunk]
		}
		if err := c.writeFrame(chunk); err != nil {
			return written, err
		}
		written += len(chunk)
	}
	return written, nil
}

// writeFrame wraps p in every onion layer and writes it as a single frame.
// The caller must hold writeMu.
func (c *libP2PConn) writeFrame(p []byte) error {
	numLayers := len(c.ciphers)
	needed := 4 + len(p) + numLayers*frameOverhead
	if cap(c.writeBuf) < needed {
		c.writeBuf = make([]byte, needed)
	}

	var scratch []byte
	if numLayers > 1 {
		scratchNeeded := len(p) + (numLayers-1)*frameOverhead
		if cap(c.scratchBuf) < scratchNeeded {
			c.scratchBuf = make([]byte, scratchNeeded)
		}
		scratch = c.scratchBuf[:0]
	}

	ciphertext, err := onion.WrapPayloadWithBuffers(p, c.ciphers, c.writeBuf[4:4], scratch)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	frame := c.writeBuf[:4+len(ciphertext)]
	// Write 4-byte big-endian length prefix directly into the frame header.
	// #nosec G115 -- len(ciphertext) <= maxWriteChunk + layer overhead, far below MaxUint32.
	binary.BigEndian.PutUint32(frame[:4], uint32(len(ciphertext)))
	if &ciphertext[0] != &frame[4] {
		copy(frame[4:], ciphertext)
	}

	_, err = c.Conn.Write(frame)
	return err
}
