package network

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/shadowlink/core/internal/crypto"
)

// pipeStream adapts a net.Conn (from net.Pipe) into the full libp2p network.Stream
// interface so we can test libP2PConn locally without a real libp2p host.
// The field is named 'nc' to avoid collision with the Conn() method required by the interface.
type pipeStream struct {
	nc net.Conn
}

// Compile-time check that pipeStream satisfies the full libp2p Stream interface.
var _ libp2pnet.Stream = (*pipeStream)(nil)

// io.Reader / io.Writer / io.Closer
func (p *pipeStream) Read(b []byte) (int, error)  { return p.nc.Read(b) }
func (p *pipeStream) Write(b []byte) (int, error) { return p.nc.Write(b) }
func (p *pipeStream) Close() error                { return p.nc.Close() }

// MuxedStream methods
func (p *pipeStream) CloseWrite() error                                { return p.nc.Close() }
func (p *pipeStream) CloseRead() error                                 { return nil }
func (p *pipeStream) Reset() error                                     { return p.nc.Close() }
func (p *pipeStream) ResetWithError(_ libp2pnet.StreamErrorCode) error { return p.nc.Close() }
func (p *pipeStream) SetDeadline(t time.Time) error                    { return p.nc.SetDeadline(t) }
func (p *pipeStream) SetReadDeadline(t time.Time) error                { return p.nc.SetReadDeadline(t) }
func (p *pipeStream) SetWriteDeadline(t time.Time) error               { return p.nc.SetWriteDeadline(t) }

// Stream-specific metadata methods
func (p *pipeStream) ID() string                          { return "pipe" }
func (p *pipeStream) Protocol() protocol.ID               { return "/test/1.0.0" }
func (p *pipeStream) SetProtocol(_ protocol.ID) error     { return nil }
func (p *pipeStream) Stat() libp2pnet.Stats               { return libp2pnet.Stats{} }
func (p *pipeStream) Conn() libp2pnet.Conn                { return nil }
func (p *pipeStream) Scope() libp2pnet.StreamScope        { return nil }

// newStreamPair creates two connected pipeStreams backed by net.Pipe().
func newStreamPair(t *testing.T) (*pipeStream, *pipeStream) {
	t.Helper()
	c1, c2 := net.Pipe()
	t.Cleanup(func() { c1.Close(); c2.Close() })
	return &pipeStream{nc: c1}, &pipeStream{nc: c2}
}

// makeLibP2PConnPair creates two connected libP2PConns with a shared symmetric key.
func makeLibP2PConnPair(t *testing.T) (*libP2PConn, *libP2PConn) {
	t.Helper()
	s1, s2 := newStreamPair(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return &libP2PConn{Stream: s1, Keys: [][]byte{key}},
		&libP2PConn{Stream: s2, Keys: [][]byte{key}}
}

// TestFraming_RoundTrip verifies encrypt→frame→transmit→deframe→decrypt works end-to-end.
func TestFraming_RoundTrip(t *testing.T) {
	writer, reader := makeLibP2PConnPair(t)
	message := []byte("framing round-trip test")

	done := make(chan struct{})
	var received []byte
	var readErr error

	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		n, err := reader.Read(buf)
		received = buf[:n]
		readErr = err
	}()

	n, err := writer.Write(message)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(message) {
		t.Errorf("Write returned %d, want %d", n, len(message))
	}

	<-done
	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}
	if !bytes.Equal(received, message) {
		t.Errorf("received %q, want %q", received, message)
	}
}

// TestFraming_MultipleMessages verifies sequential frames are independently decoded.
func TestFraming_MultipleMessages(t *testing.T) {
	writer, reader := makeLibP2PConnPair(t)
	messages := [][]byte{
		[]byte("first"),
		[]byte("second message is longer"),
		[]byte("third!"),
	}

	go func() {
		for _, msg := range messages {
			if _, err := writer.Write(msg); err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 4096)
	for _, want := range messages {
		n, err := reader.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if !bytes.Equal(buf[:n], want) {
			t.Errorf("got %q, want %q", buf[:n], want)
		}
	}
}

// TestFraming_WrongKeyFails verifies AEAD authentication rejects wrong-key decryption.
func TestFraming_WrongKeyFails(t *testing.T) {
	s1, s2 := newStreamPair(t)
	rightKey, _ := crypto.GenerateKey()
	wrongKey, _ := crypto.GenerateKey()

	writer := &libP2PConn{Stream: s1, Keys: [][]byte{rightKey}}
	reader := &libP2PConn{Stream: s2, Keys: [][]byte{wrongKey}}

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		_, err := reader.Read(buf)
		done <- err
	}()

	writer.Write([]byte("secret payload"))
	if err := <-done; err == nil {
		t.Error("Read with wrong key must return a decryption error")
	}
}

// TestFraming_LargePayload verifies framing handles payloads larger than typical MTUs.
func TestFraming_LargePayload(t *testing.T) {
	writer, reader := makeLibP2PConnPair(t)
	large := make([]byte, 32*1024) // 32 KB
	for i := range large {
		large[i] = byte(i % 251)
	}

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64*1024)
		n, _ := io.ReadFull(reader, buf[:len(large)])
		received = buf[:n]
	}()

	if _, err := writer.Write(large); err != nil {
		t.Fatalf("Write large: %v", err)
	}
	<-done

	if !bytes.Equal(received, large) {
		t.Error("large payload round-trip mismatch")
	}
}
