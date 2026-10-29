package network

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"strings"
	"sync"
	"syscall"
	"time"

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

	// Enforce a handshake deadline on the initial control command
	// to prevent slow-loris connection stalling.
	_ = s.SetDeadline(time.Now().Add(handshakeTimeout))

	// Read one byte at a time — bufio would pre-fetch past the newline into the
	// raw ECDH public-key bytes, permanently corrupting the handshake.
	firstLine, err := readLineRaw(s)
	if err != nil {
		log.Printf("Handler: failed to read first line from %s: %v", s.Conn().RemotePeer(), err)
		_ = s.Reset()
		return
	}

	if !roleAllows(role, firstLine) {
		log.Printf("Handler: %q not permitted for local role %q — dropping stream from %s", firstLine, role, s.Conn().RemotePeer())
		_ = s.Reset()
		return
	}

	switch firstLine {
	case config.ExtendHeader:
		// Relay mode: upstream entry wants us to extend the circuit to an exit peer.
		targetPeerID, err := readLineRaw(s)
		if err != nil {
			log.Printf("Handler: failed to read target peer ID for EXTEND from %s: %v", s.Conn().RemotePeer(), err)
			_ = s.Reset()
			return
		}
		handleRelay(ctx, s, ds, targetPeerID)

	case config.ConnectHeader:
		// Exit mode: upstream (relay or direct entry) wants us to reach an internet host.
		// The destination arrives encrypted after the handshake, inside handleExit.
		handleExit(ctx, s)

	default:
		log.Printf("Handler: unknown protocol header %q from %s — dropping stream", firstLine, s.Conn().RemotePeer())
		_ = s.Reset()
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

	_ = s.SetDeadline(time.Now().Add(handshakeTimeout))

	// The entry gives up after handshakeTimeout, so bound the exit lookup, connect
	// and stream setup by the same budget instead of the process-lifetime ctx.
	setupCtx, cancelSetup := context.WithTimeout(ctx, handshakeTimeout)
	defer cancelSetup()

	// Step 1: ECDH — relay is RESPONDER (reads entry's key first, then sends its own).
	relayKey, err := crypto.RespondECDHWithInfo(rawReadWriter{s, s}, config.HKDFInfoRelay)
	if err != nil {
		log.Printf("Relay: ECDH with entry failed: %v", err)
		_ = s.Reset()
		return
	}

	// Step 2: Parse and resolve the exit peer ID.
	exitID, err := peer.Decode(targetPeerID)
	if err != nil {
		log.Printf("Relay: invalid exit peer ID %q: %v", targetPeerID, err)
		_ = s.Reset()
		return
	}

	var exitAddrInfo peer.AddrInfo
	if pInfo := ds.Host.Peerstore().PeerInfo(exitID); len(pInfo.Addrs) > 0 {
		exitAddrInfo = pInfo
	} else if ds.DHT != nil {
		var err error
		exitAddrInfo, err = ds.DHT.FindPeer(setupCtx, exitID)
		if err != nil {
			log.Printf("Relay: DHT lookup for exit %s failed: %v", exitID, err)
			_ = s.Reset()
			return
		}
	} else {
		log.Printf("Relay: no peerstore addresses and DHT unavailable for exit %s", exitID)
		_ = s.Reset()
		return
	}

	if err := ds.Host.Connect(setupCtx, exitAddrInfo); err != nil {
		log.Printf("Relay: connect to exit %s failed: %v", exitID, err)
		_ = s.Reset()
		return
	}

	exitStream, err := ds.Host.NewStream(setupCtx, exitID, config.ProtocolID)
	if err != nil {
		log.Printf("Relay: open stream to exit %s failed: %v", exitID, err)
		_ = s.Reset()
		return
	}
	defer exitStream.Close()

	// Clear handshake deadlines on both streams for unrestricted payload bridging.
	_ = s.SetDeadline(time.Time{})
	_ = exitStream.SetDeadline(time.Time{})

	// Step 3: Bridge.
	// upstreamConn strips the outer relayKey layer from entry frames.
	// The resulting inner ciphertext is forwarded byte-for-byte to exitStream.
	// The exit node decrypts the inner exitKey layer — relay never sees plaintext.
	upstreamConn := newLibP2PConn(streamAdapter{s}, [][]byte{relayKey})
	log.Printf("Relay: circuit active — bridging entry <-> exit %s", exitID)

	bridge(upstreamConn, streamAdapter{exitStream})
}

// exitControl vets every address the exit dials. Tests swap it to reach loopback servers.
var exitControl = exitDialControl

// handleExit implements the exit node's internet egress.
//
// Flow:
//  1. ECDH handshake with upstream (relay or direct entry) → sessionKey.
//  2. Read the target record encrypted under sessionKey.
//  3. Dial the target on the public internet using a context-aware dialer.
//  4. Proxy bidirectionally: encrypted dVPN ↔ plaintext internet.
//
// Destinations are deliberately not logged, so exit operators keep no record
// of where peers' traffic goes.
func handleExit(ctx context.Context, s libp2pnet.Stream) {
	_ = s.SetDeadline(time.Now().Add(handshakeTimeout))

	// Exit is always RESPONDER — reads the upstream peer's public key first.
	sessionKey, err := crypto.RespondECDHWithInfo(rawReadWriter{s, s}, config.HKDFInfoExit)
	if err != nil {
		log.Printf("Exit: ECDH handshake failed: %v", err)
		_ = s.Reset()
		return
	}

	wrapper := newLibP2PConn(streamAdapter{s}, [][]byte{sessionKey})
	targetAddr, err := readTargetRecord(wrapper)
	if err != nil {
		log.Printf("Exit: %v", err)
		_ = s.Reset()
		return
	}

	// Clear stream deadline for data streaming.
	_ = s.SetDeadline(time.Time{})

	// Use a context-aware dialer with connection timeout derived from parent ctx.
	// Control refuses loopback/private/link-local targets so peers cannot use this
	// exit to reach the operator's own machine or LAN (SSRF).
	d := net.Dialer{Control: exitControl}
	dialCtx, dialCancel := context.WithTimeout(ctx, handshakeTimeout)
	defer dialCancel()

	outConn, err := d.DialContext(dialCtx, "tcp", targetAddr)
	if err != nil {
		log.Printf("Exit: failed to dial target: %v", redactTarget(err, targetAddr))
		_ = s.Reset()
		return
	}
	defer outConn.Close()

	log.Printf("Exit: circuit active")
	bridge(wrapper, outConn)
}

// redactTarget strips the destination from dial errors before they are logged.
func redactTarget(err error, targetAddr string) string {
	msg := err.Error()
	if host, _, splitErr := net.SplitHostPort(targetAddr); splitErr == nil && host != "" {
		msg = strings.ReplaceAll(msg, host, "<target>")
	}
	return strings.ReplaceAll(msg, targetAddr, "<target>")
}

// roleAllows reports whether a node running under role may serve the given
// protocol command. Without this check a relay-only node would accept CONNECT
// and silently egress third-party traffic from its own IP.
func roleAllows(role, command string) bool {
	switch command {
	case config.ExtendHeader:
		return role == config.RoleRelay || role == config.RoleRelayExit
	case config.ConnectHeader:
		return role == config.RoleExit || role == config.RoleRelayExit
	default:
		return false
	}
}

// blockedExitPrefixes lists special-purpose ranges that netip's IsGlobalUnicast
// helper may still classify as globally unicast. Exit nodes must not route to
// documentation, benchmarking, transition, or reserved address space.
var blockedExitPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
	netip.MustParsePrefix("192.0.0.0/24"),    // protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // documentation (TEST-NET-1)
	netip.MustParsePrefix("192.88.99.0/24"),  // deprecated 6to4 relay anycast
	netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // documentation (TEST-NET-2)
	netip.MustParsePrefix("203.0.113.0/24"),  // documentation (TEST-NET-3)
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved + limited broadcast
	netip.MustParsePrefix("64:ff9b::/96"),    // well-known NAT64 embeds an IPv4 destination
	netip.MustParsePrefix("64:ff9b:1::/48"),  // local-use IPv4/IPv6 translation
	netip.MustParsePrefix("2001::/32"),       // Teredo embeds an IPv4 destination
	netip.MustParsePrefix("2001:2::/48"),     // benchmarking
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
	netip.MustParsePrefix("2002::/16"),       // 6to4 embeds an IPv4 destination
	netip.MustParsePrefix("3fff::/20"),       // documentation
}

// isPublicAddr reports whether ip is a globally routable unicast address.
func isPublicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, p := range blockedExitPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// exitDialControl rejects connections to non-public addresses. It runs after DNS
// resolution, so hostnames that resolve to internal addresses are refused too.
func exitDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if !isPublicAddr(ip) {
		return fmt.Errorf("exit: refusing to dial non-public address %s", ip)
	}
	return nil
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
// A stack-allocated buffer (stackBuf) handles lines <= 128 bytes with 0 allocations.
// Lines between 129 and maxLineLen bytes dynamically escape to the heap via append.
// A hard limit of maxLineLen protects against slow-loris attacks where an
// adversary streams a very long line without a newline to exhaust memory.
func readLineRaw(r io.Reader) (string, error) {
	const maxLineLen = 4096
	var stackBuf [128]byte
	line := stackBuf[:0]
	var buf [1]byte

	for {
		if len(line) >= maxLineLen {
			return "", fmt.Errorf("line length exceeds maximum allowed (%d bytes)", maxLineLen)
		}
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
