package network

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/crypto"
	"github.com/shadowlink/core/internal/discovery"
)

// rawReadWriter combines a reader with the raw stream writer (no buffering).
// This prevents ECDH key exchange deadlocks: if both sides used buffered writers
// and waited for each other's key, neither write would ever flush.
type rawReadWriter struct {
	io.Reader
	io.Writer
}

// HandleStream dispatches an incoming libp2p stream to the appropriate handler
// based on the protocol header sent by the entry node.
//
// ctx is passed from main so that all downstream operations (DHT queries, stream opens)
// respect the application shutdown signal and do not leak goroutines.
func HandleStream(ctx context.Context, s libp2pnet.Stream, role string, ds *discovery.DiscoveryService) {
	log.Printf("Incoming stream from %s (local role: %s)", s.Conn().RemotePeer(), role)
	defer s.Close()

	// Read the first line one byte at a time to avoid over-buffering.
	// Using bufio.NewReader would pre-fetch bytes past the text lines and into
	// the raw ECDH key bytes, causing decryption failures.
	firstLine, err := readLineRaw(s)
	if err != nil {
		log.Printf("Handler: failed to read first line: %v", err)
		s.Reset()
		return
	}

	if firstLine == config.RelayHeader {
		// config.RelayHeader means the entry node wants us to forward this circuit to an exit.
		targetAddr, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target address for relay: %v", err)
			s.Reset()
			return
		}
		handleRelay(ctx, s, ds, targetAddr)
	} else {
		// No RelayHeader prefix — firstLine is the target address directly.
		// This is either a direct Entry→Exit connection, or traffic arriving from a relay.
		handleExit(s, firstLine)
	}
}

// handleRelay implements the relay node's forwarding logic.
//
// Flow:
//  1. ECDH handshake with the upstream (entry) — derive entryKey
//  2. Find a random exit node from the DHT (uses ctx, not context.Background)
//  3. Open a stream to the exit node and send the target address
//  4. ECDH handshake with the downstream (exit) — derive exitKey
//  5. Bridge the two encrypted tunnels bidirectionally
//
// The relay sees only encrypted traffic at each layer. It decrypts Entry→Relay
// traffic with entryKey and re-encrypts with exitKey before forwarding to Exit.
func handleRelay(ctx context.Context, s libp2pnet.Stream, ds *discovery.DiscoveryService, targetAddr string) {
	log.Printf("Relay: building circuit for %s", targetAddr)

	// Step 1: ECDH with the entry node.
	// Relay is the RESPONDER — reads entry's public key first, then sends its own.
	entryKey, err := crypto.RespondECDH(rawReadWriter{s, s})
	if err != nil {
		log.Printf("Relay: ECDH with entry failed: %v", err)
		s.Reset()
		return
	}

	// Step 2: Find an exit node from the DHT.
	// Uses the passed ctx so shutdown cancels this cleanly.
	exits, err := ds.FindPeers(ctx, config.RendezvousExit)
	if err != nil || len(exits) == 0 {
		log.Printf("Relay: no exit nodes available: %v", err)
		s.Reset()
		return
	}

	// Step 3: Connect to an exit node and open a downstream stream.
	// connErr and streamErr use distinct names to avoid shadowing the outer err variable.
	var exitStream libp2pnet.Stream
	for _, exit := range exits {
		if connErr := ds.Host.Connect(ctx, exit); connErr != nil {
			log.Printf("Relay: failed to reach exit %s: %v", exit.ID, connErr)
			continue
		}
		stream, streamErr := ds.Host.NewStream(ctx, exit.ID, config.ProtocolID)
		if streamErr != nil {
			log.Printf("Relay: failed to open stream to exit %s: %v", exit.ID, streamErr)
			continue
		}
		exitStream = stream
		log.Printf("Relay: connected to exit %s", exit.ID)
		break
	}

	if exitStream == nil {
		log.Printf("Relay: all exit nodes unreachable")
		s.Reset()
		return
	}
	defer exitStream.Close()

	// Step 4: Send target address to the exit node (no RelayHeader — exit handles directly).
	if _, err := fmt.Fprintf(exitStream, "%s\n", targetAddr); err != nil {
		log.Printf("Relay: failed to send target addr to exit: %v", err)
		s.Reset()
		return
	}

	// Step 5: ECDH with the exit node.
	// Relay is the INITIATOR toward exit — sends its public key first.
	exitKey, err := crypto.PerformECDH(exitStream)
	if err != nil {
		log.Printf("Relay: ECDH with exit failed: %v", err)
		s.Reset()
		return
	}

	// Step 6: Create encrypted connection wrappers for each hop.
	upstreamConn := &libP2PConn{Stream: s, Keys: [][]byte{entryKey}}
	downstreamConn := &libP2PConn{Stream: exitStream, Keys: [][]byte{exitKey}}
	log.Printf("Relay: circuit active for %s (independent per-hop ECDH-HKDF keys)", targetAddr)

	// Step 7: Bridge the two tunnels bidirectionally.
	bridge(upstreamConn, downstreamConn)
}

// handleExit implements the exit node's forwarding logic.
//
// Flow:
//  1. ECDH handshake with upstream (relay or entry) — derive sessionKey
//  2. Dial the target address on the public internet
//  3. Proxy cleartext internet traffic ↔ encrypted dVPN traffic
func handleExit(s libp2pnet.Stream, targetAddr string) {
	log.Printf("Exit Node: connecting to %s", targetAddr)

	// ECDH with upstream (relay or entry).
	// Exit is always the RESPONDER — reads upstream's public key first.
	sessionKey, err := crypto.RespondECDH(rawReadWriter{s, s})
	if err != nil {
		log.Printf("Exit: ECDH failed: %v", err)
		s.Reset()
		return
	}

	// Dial the real target on the public internet.
	outConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Exit Node: failed to dial %s: %v", targetAddr, err)
		s.Reset()
		return
	}
	defer outConn.Close()

	// Create the encrypted dVPN tunnel wrapper using the per-session ECDH-HKDF key.
	wrapper := &libP2PConn{Stream: s, Keys: [][]byte{sessionKey}}
	log.Printf("Exit Node: circuit active for %s", targetAddr)

	bridge(wrapper, outConn)
}

// bridge proxies data bidirectionally between two net.Conn connections.
// It launches two goroutines and waits for either direction to close.
func bridge(a, b net.Conn) {
	errc := make(chan error, 2)
	copyDir := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go copyDir(b, a)
	go copyDir(a, b)

	// Wait for either direction to close (normal on disconnect).
	if err := <-errc; err != nil {
		log.Printf("Bridge: circuit closed: %v", err)
	}
}

// readLineRaw reads a newline-terminated string one byte at a time from the raw stream.
// This is critical for correctness: unlike bufio.ReadString, it never over-reads,
// so the bytes immediately following the newline (e.g., ECDH key bytes) are not lost.
func readLineRaw(r io.Reader) (string, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return strings.TrimSpace(string(line)), err
		}
		if buf[0] == '\n' {
			break
		}
		line = append(line, buf[0])
	}
	return strings.TrimSpace(string(line)), nil
}
