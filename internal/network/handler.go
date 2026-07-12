package network

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/shadowlink/core/internal/crypto"
	"github.com/shadowlink/core/internal/discovery"
)

// rawReadWriter combines a buffered reader (for safe line reads) with the raw stream
// writer (no buffering). This prevents ECDH key exchange deadlocks: if both sides
// used buffered writers and waited for each other's key, neither write would flush.
type rawReadWriter struct {
	io.Reader
	io.Writer
}

// HandleStream dispatches an incoming libp2p stream to the appropriate handler
// based on the node's configured role (relay or exit) and the protocol header.
//
// Bug 4 Fix: Relay nodes now fully forward traffic instead of just closing the stream.
// Bug 1 Fix: All connections use per-session X25519 ECDH keys instead of a hardcoded key.
func HandleStream(s libp2pnet.Stream, role string, ds *discovery.DiscoveryService) {
	log.Printf("Incoming stream from %s (local role: %s)", s.Conn().RemotePeer(), role)
	defer s.Close()

	// Read the first line using a byte-by-byte approach to avoid over-buffering.
	// If we used bufio.NewReader, it would pre-fetch data past the text lines
	// and into the raw ECDH key bytes, causing decryption failures.
	firstLine, err := readLineRaw(s)
	if err != nil {
		log.Printf("Handler: failed to read first line: %v", err)
		s.Reset()
		return
	}

	if firstLine == "RELAY" {
		// I-3 / Bug 4 Fix: "RELAY" keyword means the entry node wants us to forward
		// this circuit to an exit node on its behalf. Read the target and bridge.
		targetAddr, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target address for relay: %v", err)
			s.Reset()
			return
		}
		handleRelay(s, ds, targetAddr)
	} else {
		// No "RELAY" prefix — firstLine is the target address directly.
		// This is either a direct Entry→Exit connection, or traffic from a relay to an exit.
		handleExit(s, firstLine)
	}
}

// handleRelay implements the relay node's forwarding logic.
//
// Flow:
//  1. ECDH handshake with the upstream (entry) connection — get entryKey
//  2. Find an available exit node from the DHT
//  3. Open a stream to the exit node and send the target address
//  4. ECDH handshake with the downstream (exit) connection — get exitKey
//  5. Bridge the two tunnels bidirectionally:
//     upstream (encrypted with entryKey) ↔ downstream (encrypted with exitKey)
//
// The relay sees only encrypted traffic at each layer. It decrypts Entry→Relay
// traffic with entryKey and re-encrypts with exitKey before sending to Exit.
// This achieves independent per-hop encryption (layered security).
func handleRelay(s libp2pnet.Stream, ds *discovery.DiscoveryService, targetAddr string) {
	log.Printf("Relay: building circuit for %s", targetAddr)

	// Step 1: ECDH with the entry node.
	// Relay is the RESPONDER — reads entry's public key first, then sends its own.
	entryKey, err := crypto.RespondECDH(rawReadWriter{s, s})
	if err != nil {
		log.Printf("Relay: ECDH with entry failed: %v", err)
		s.Reset()
		return
	}

	// Step 2: Find an exit node from the DHT
	exits, err := ds.FindPeers(context.Background(), "shadowlink-exit")
	if err != nil || len(exits) == 0 {
		log.Printf("Relay: no exit nodes available: %v", err)
		s.Reset()
		return
	}

	// Step 3: Connect to an exit node and establish the downstream stream
	var exitStream libp2pnet.Stream
	for _, exit := range exits {
		if err := ds.Host.Connect(context.Background(), exit); err != nil {
			log.Printf("Relay: failed to reach exit %s: %v", exit.ID, err)
			continue
		}
		exitStream, err = ds.Host.NewStream(context.Background(), exit.ID, ProtocolID)
		if err != nil {
			log.Printf("Relay: failed to open stream to exit %s: %v", exit.ID, err)
			continue
		}
		log.Printf("Relay: connected to exit %s", exit.ID)
		break
	}

	if exitStream == nil {
		log.Printf("Relay: all exit nodes unreachable")
		s.Reset()
		return
	}
	defer exitStream.Close()

	// Step 4: Send target address to the exit node (no "RELAY" prefix — exit handles directly)
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

	// Step 6: Create encrypted connection wrappers for each hop
	upstreamConn := &libP2PConn{Stream: s, Keys: [][]byte{entryKey}}
	downstreamConn := &libP2PConn{Stream: exitStream, Keys: [][]byte{exitKey}}

	log.Printf("Relay: circuit active for %s (entryKey and exitKey are independent)", targetAddr)

	// Step 7: Bridge the two tunnels bidirectionally.
	// Data from entry is decrypted with entryKey, then re-encrypted with exitKey toward exit.
	// Data from exit is decrypted with exitKey, then re-encrypted with entryKey toward entry.
	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(downstreamConn, upstreamConn)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(upstreamConn, downstreamConn)
		errc <- err
	}()

	// Wait for either side to close (normal on disconnect)
	if err := <-errc; err != nil {
		log.Printf("Relay: circuit closed: %v", err)
	}
}

// handleExit implements the exit node's forwarding logic.
//
// Flow:
//  1. ECDH handshake with upstream (relay or entry) — get sessionKey
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

	// Dial the real target on the internet
	outConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Exit Node: failed to dial %s: %v", targetAddr, err)
		s.Reset()
		return
	}
	defer outConn.Close()

	// Create encrypted wrapper using the per-session ECDH key
	wrapper := &libP2PConn{Stream: s, Keys: [][]byte{sessionKey}}

	log.Printf("Exit Node: circuit active for %s", targetAddr)

	// Proxy: cleartext internet ↔ encrypted dVPN tunnel
	errc := make(chan error, 2)
	go func() {
		// Receive encrypted data from dVPN, decrypt, send to internet
		_, err := io.Copy(outConn, wrapper)
		errc <- err
	}()
	go func() {
		// Receive data from internet, encrypt, send back through dVPN
		_, err := io.Copy(wrapper, outConn)
		errc <- err
	}()

	if err := <-errc; err != nil {
		log.Printf("Exit Node: circuit closed: %v", err)
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
