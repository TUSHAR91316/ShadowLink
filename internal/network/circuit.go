package network

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/onion"
)

const ProtocolID = "/shadowlink/1.0.0"

// DialCircuit acts as the net.Conn dialer for the SOCKS5 proxy.
// It queries the DHT, establishes a libp2p stream, and wraps it in onion routing.
func DialCircuit(ctx context.Context, ds *discovery.DiscoveryService, targetNetwork, targetAddr string) (net.Conn, error) {
	log.Printf("DialCircuit: looking for relay/exit nodes for %s", targetAddr)

	peers, err := ds.FindPeers(ctx, "shadowlink-exit")
	if err != nil || len(peers) == 0 {
		return nil, fmt.Errorf("no exit nodes found in DHT")
	}

	exitNode := peers[0]
	log.Printf("Dialing exit node: %s", exitNode.ID)

	if err := ds.Host.Connect(ctx, exitNode); err != nil {
		return nil, fmt.Errorf("failed to connect to exit node: %v", err)
	}

	stream, err := ds.Host.NewStream(ctx, exitNode.ID, ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %v", err)
	}

	// Write the target address to the stream so the exit node knows where to go.
	// In a full implementation, this handshaking is encrypted and layered.
	_, err = stream.Write([]byte(targetAddr + "\n"))
	if err != nil {
		stream.Reset()
		return nil, err
	}

	return &libP2PConn{
		Stream: stream,
		// Using a dummy 32-byte key for the scaffold. Real implementation uses ECDH.
		Keys: [][]byte{[]byte("12345678901234567890123456789012")}, 
	}, nil
}

// libP2PConn wraps a libp2p network.Stream to implement net.Conn
// and applies onion encryption transparently.
type libP2PConn struct {
	network.Stream
	Keys [][]byte
}

func (c *libP2PConn) Read(b []byte) (n int, err error) {
	// Account for encryption overhead (24 byte nonce + 16 byte tag)
	buf := make([]byte, len(b)+40)
	n, err = c.Stream.Read(buf)
	if err != nil || n == 0 {
		return n, err
	}

	plaintext, err := onion.UnwrapPayload(buf[:n], c.Keys[0])
	if err != nil {
		return 0, fmt.Errorf("decryption failed: %v", err)
	}

	copied := copy(b, plaintext)
	return copied, nil
}

func (c *libP2PConn) Write(b []byte) (n int, err error) {
	ciphertext, err := onion.WrapPayload(b, c.Keys)
	if err != nil {
		return 0, err
	}

	_, err = c.Stream.Write(ciphertext)
	if err != nil {
		return 0, err
	}
	// Return the plaintext length to satisfy net.Conn contract
	return len(b), nil
}

func (c *libP2PConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (c *libP2PConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }
