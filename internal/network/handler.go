package network

import (
	"context"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/crypto"
	"github.com/shadowlink/core/internal/discovery"
)

// copyBufferPool recycles 32 KiB I/O buffers across all active bridge tunnels,
// significantly reducing garbage collection overhead during high-concurrency routing.
var copyBufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32*1024)
		return &b
	},
}

// rawReadWriter combines a plain io.Reader with a plain io.Writer.
// Used during ECDH so we write directly to the raw libp2p stream without
// any buffering — a buffered writer could hold the key bytes in memory and
// never flush, causing both sides to deadlock waiting for each other.
type rawReadWriter struct {
	io.Reader
	io.Writer
}

// HandleStream dispatches an incoming libp2p stream to the correct handler
// based on the first protocol command sent by the upstream node.
//
// ctx is propagated from main so all downstream DHT queries and stream dials
// honour the application-level shutdown signal.
func HandleStream(ctx context.Context, s libp2pnet.Stream, role string, ds *discovery.DiscoveryService) {
	log.Printf("Incoming stream from %s (local role: %s)", s.Conn().RemotePeer(), role)
	defer s.Close()

	// Read one byte at a time — bufio would pre-fetch past the newline into the
	// raw ECDH public-key bytes, permanently corrupting the handshake.
	firstLine, err := readLineRaw(s)
	if err != nil {
		log.Printf("Handler: failed to read first line from %s: %v", s.Conn().RemotePeer(), err)
		s.Reset() //nolint:errcheck
		return
	}

	switch firstLine {
	case config.ExtendHeader:
		// Relay mode: upstream entry wants us to extend the circuit to an exit peer.
		targetPeerID, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target peer ID for EXTEND from %s: %v", s.Conn().RemotePeer(), err)
			s.Reset() //nolint:errcheck
			return
		}
		handleRelay(ctx, s, ds, targetPeerID)

	case config.ConnectHeader:
		// Exit mode: upstream (relay or direct entry) wants us to reach an internet host.
		targetAddr, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target addr for CONNECT from %s: %v", s.Conn().RemotePeer(), err)
			s.Reset() //nolint:errcheck
			return
		}
		handleExit(s, targetAddr)

	default:
		log.Printf("Handler: unknown protocol header %q from %s — dropping stream", firstLine, s.Conn().RemotePeer())
		s.Reset() //nolint:errcheck
	}
}

// handleRelay implements the relay node's transparent forwarding logic.
//
// Onion-routing flow:
//  1. ECDH handshake with the upstream entry → relayKey.
//  2. Locate the requested exit peer via DHT.
//  3. Open a raw stream to the exit node.
//  4. Bridge: upstream frames are decrypted (outer relayKey layer removed) and
//     forwarded verbatim to the exit. The relay never sees plaintext.
func handleRelay(ctx context.Context, s libp2pnet.Stream, ds *discovery.DiscoveryService, targetPeerID string) {
	log.Printf("Relay: extending circuit to exit peer %s", targetPeerID)

	// Step 1: ECDH — relay is RESPONDER (reads entry's key first, then sends its own).
	relayKey, err := crypto.RespondECDH(rawReadWriter{s, s})
	if err != nil {
		log.Printf("Relay: ECDH with entry failed: %v", err)
		s.Reset() //nolint:errcheck
		return
	}

	// Step 2: Parse and resolve the exit peer ID.
	exitID, err := peer.Decode(targetPeerID)
	if err != nil {
		log.Printf("Relay: invalid exit peer ID %q: %v", targetPeerID, err)
		s.Reset() //nolint:errcheck
		return
	}

	exitAddrInfo, err := ds.DHT.FindPeer(ctx, exitID)
	if err != nil {
		log.Printf("Relay: DHT lookup for exit %s failed: %v", exitID, err)
		s.Reset() //nolint:errcheck
		return
	}

	if err := ds.Host.Connect(ctx, exitAddrInfo); err != nil {
		log.Printf("Relay: connect to exit %s failed: %v", exitID, err)
		s.Reset() //nolint:errcheck
		return
	}

	exitStream, err := ds.Host.NewStream(ctx, exitID, config.ProtocolID)
	if err != nil {
		log.Printf("Relay: open stream to exit %s failed: %v", exitID, err)
		s.Reset() //nolint:errcheck
		return
	}
	defer exitStream.Close()

	// Step 3: Bridge.
	// upstreamConn strips the outer relayKey layer from entry frames.
	// The resulting inner ciphertext is forwarded byte-for-byte to exitStream.
	// The exit node decrypts the inner exitKey layer — relay never sees plaintext.
	upstreamConn := newLibP2PConn(streamAdapter{s}, [][]byte{relayKey})
	log.Printf("Relay: circuit active — bridging entry <-> exit %s", exitID)

	bridge(upstreamConn, streamAdapter{exitStream})
}

// handleExit implements the exit node's internet egress.
//
// Flow:
//  1. ECDH handshake with upstream (relay or direct entry) → sessionKey.
//  2. Dial the target on the public internet using a context-aware dialer.
//  3. Proxy bidirectionally: encrypted dVPN ↔ plaintext internet.
func handleExit(s libp2pnet.Stream, targetAddr string) {
	log.Printf("Exit: connecting to %s", targetAddr)

	// Exit is always RESPONDER — reads the upstream peer's public key first.
	sessionKey, err := crypto.RespondECDH(rawReadWriter{s, s})
	if err != nil {
		log.Printf("Exit: ECDH handshake failed: %v", err)
		s.Reset() //nolint:errcheck
		return
	}

	// Use a context-aware dialer so the exit dial respects shutdown signals.
	var d net.Dialer
	outConn, err := d.DialContext(context.Background(), "tcp", targetAddr)
	if err != nil {
		log.Printf("Exit: failed to dial %s: %v", targetAddr, err)
		s.Reset() //nolint:errcheck
		return
	}
	defer outConn.Close()

	wrapper := newLibP2PConn(streamAdapter{s}, [][]byte{sessionKey})
	log.Printf("Exit: circuit active — proxying to %s", targetAddr)

	bridge(wrapper, outConn)
}

// bridge proxies data bidirectionally between two net.Conn connections.
// Two goroutines run io.CopyBuffer in parallel using pooled 32 KiB buffers.
// When either direction closes or errors, both connections are closed to guarantee
// the other goroutine terminates, preventing goroutine leaks.
func bridge(a, b net.Conn) {
	errc := make(chan error, 2)
	copyDir := func(dst, src net.Conn) {
		bufPtr := copyBufferPool.Get().(*[]byte)
		defer copyBufferPool.Put(bufPtr)

		_, err := io.CopyBuffer(dst, src, *bufPtr)
		errc <- err
	}

	go copyDir(b, a) // a → b
	go copyDir(a, b) // b → a

	// Wait for the first direction to finish (normal on disconnect or error).
	if err := <-errc; err != nil {
		log.Printf("Bridge: circuit closed with error: %v", err)
	}

	// Forcibly close both to unblock the companion goroutine.
	_ = a.Close()
	_ = b.Close()

	// Drain the second result to prevent a channel/goroutine leak.
	<-errc
}

// readLineRaw reads one newline-terminated line from r one byte at a time.
//
// Using bufio is intentionally avoided: it pre-reads past the newline into
// the raw ECDH key bytes that immediately follow, permanently losing those bytes.
//
// Lines are trimmed of surrounding whitespace (handles both \n and \r\n).
// A hard limit of maxLineLen protects against slow-loris attacks where an
// adversary streams a very long line without a newline to exhaust memory.
func readLineRaw(r io.Reader) (string, error) {
	const maxLineLen = 4096
	var line []byte
	var buf [1]byte

	for len(line) <= maxLineLen {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return strings.TrimSpace(string(line)), err
		}
		if buf[0] == '\n' {
			break
		}
		line = append(line, buf[0])
	}
	return strings.TrimSpace(string(line)), nil
}
