package network

import (
	"context"
	"io"
	"log"
	"net"
	"strings"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
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

	if firstLine == config.ExtendHeader {
		targetPeerID, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target peer ID for EXTEND: %v", err)
			s.Reset()
			return
		}
		handleRelay(ctx, s, ds, targetPeerID)
	} else if firstLine == config.ConnectHeader {
		targetAddr, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target addr for CONNECT: %v", err)
			s.Reset()
			return
		}
		handleExit(s, targetAddr)
	} else {
		log.Printf("Handler: unknown protocol header: %q", firstLine)
		s.Reset()
	}
}

// handleRelay implements the relay node's forwarding logic.
//
// Flow:
//  1. ECDH handshake with the upstream (entry) — derive relayKey
//  2. Connect to the requested Exit node
//  3. Bridge the encrypted tunnel transparently
func handleRelay(ctx context.Context, s libp2pnet.Stream, ds *discovery.DiscoveryService, targetPeerID string) {
	log.Printf("Relay: extending circuit to %s", targetPeerID)

	// Step 1: ECDH with the entry node.
	relayKey, err := crypto.RespondECDH(rawReadWriter{s, s})
	if err != nil {
		log.Printf("Relay: ECDH with entry failed: %v", err)
		s.Reset()
		return
	}

	// Step 2: Connect to the requested Exit node.
	exitID, err := peer.Decode(targetPeerID)
	if err != nil {
		log.Printf("Relay: invalid exit peer ID %q: %v", targetPeerID, err)
		s.Reset()
		return
	}

	exitAddrInfo, err := ds.DHT.FindPeer(ctx, exitID)
	if err != nil {
		log.Printf("Relay: failed to find exit node %s in DHT: %v", exitID, err)
		s.Reset()
		return
	}

	if err := ds.Host.Connect(ctx, exitAddrInfo); err != nil {
		log.Printf("Relay: failed to connect to exit %s: %v", exitID, err)
		s.Reset()
		return
	}

	exitStream, err := ds.Host.NewStream(ctx, exitID, config.ProtocolID)
	if err != nil {
		log.Printf("Relay: failed to open stream to exit %s: %v", exitID, err)
		s.Reset()
		return
	}
	defer exitStream.Close()

	// Step 3: Bridge the two tunnels bidirectionally.
	// We wrap the upstream stream with libP2PConn to decrypt outer frames,
	// and bridge the decrypted inner frames transparently to the raw exit stream.
	upstreamConn := &libP2PConn{Conn: streamAdapter{s}, Keys: [][]byte{relayKey}}
	log.Printf("Relay: circuit active for %s", targetPeerID)

	bridge(upstreamConn, streamAdapter{exitStream})
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
	wrapper := &libP2PConn{Conn: streamAdapter{s}, Keys: [][]byte{sessionKey}}
	log.Printf("Exit Node: circuit active for %s", targetAddr)

	bridge(wrapper, outConn)
}

// bridge proxies data bidirectionally between two net.Conn connections.
// It launches two goroutines and waits for either direction to close,
// then closes both connections to prevent goroutine leaks.
func bridge(a, b net.Conn) {
	errc := make(chan error, 2)
	copyDir := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go copyDir(b, a)
	go copyDir(a, b)

	// Wait for the first direction to close
	if err := <-errc; err != nil {
		log.Printf("Bridge: circuit closed: %v", err)
	}

	// Forcibly close both to unblock the other copyDir goroutine.
	a.Close()
	b.Close()

	// Wait for the second goroutine to finish and send its error.
	<-errc
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
