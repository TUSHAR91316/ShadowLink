package network

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/crypto"
)

// relaySpy records every byte the entry hands to the relay tunnel — exactly
// what a relay sees after stripping its own encryption layer.
type relaySpy struct {
	net.Conn
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *relaySpy) Write(b []byte) (int, error) {
	r.mu.Lock()
	r.buf.Write(b)
	r.mu.Unlock()
	return r.Conn.Write(b)
}

func (r *relaySpy) seen() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.buf.Bytes()...)
}

// fakeExit plays the exit's side of the handshake and returns the decoded target.
func fakeExit(t *testing.T, s *pipeStream) <-chan string {
	t.Helper()
	got := make(chan string, 1)
	go func() {
		defer close(got)
		if line, err := readLineRaw(s); err != nil || line != config.ConnectHeader {
			return
		}
		key, err := crypto.RespondECDHWithInfo(s, config.HKDFInfoExit)
		if err != nil {
			return
		}
		target, err := readTargetRecord(newLibP2PConn(streamAdapter{s}, [][]byte{key}))
		if err != nil {
			return
		}
		got <- target
	}()
	return got
}

// TestOpenExitSession_RelayNeverSeesTarget verifies that the destination never
// crosses the relay in plaintext and that its length is hidden by padding.
func TestOpenExitSession_RelayNeverSeesTarget(t *testing.T) {
	targets := []string{
		"a.io:80",
		"a-much-longer-destination-hostname.example.com:443",
	}
	var observedLen []int
	for _, target := range targets {
		entrySide, exitSide := newStreamPair(t)
		spy := &relaySpy{Conn: streamAdapter{entrySide}}
		exitGot := fakeExit(t, exitSide)

		if _, err := openExitSession(spy, target); err != nil {
			t.Fatalf("openExitSession(%q): %v", target, err)
		}
		if got := <-exitGot; got != target {
			t.Fatalf("exit decoded target %q, want %q", got, target)
		}

		seen := spy.seen()
		host := target[:strings.LastIndex(target, ":")]
		if bytes.Contains(seen, []byte(host)) {
			t.Errorf("relay observed plaintext destination %q", host)
		}
		observedLen = append(observedLen, len(seen))
	}
	if observedLen[0] != observedLen[1] {
		t.Errorf("relay-visible handshake size depends on target length: %v", observedLen)
	}
}

// TestHandleExit_EndToEnd drives the real exit handler through the v2 handshake
// to a local echo server and checks data flows both ways.
func TestHandleExit_EndToEnd(t *testing.T) {
	// Loopback targets are refused in production; allow them for this test only.
	exitControl = nil
	t.Cleanup(func() { exitControl = exitDialControl })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(c, c)
	}()

	entrySide, exitSide := newStreamPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if line, err := readLineRaw(exitSide); err == nil && line == config.ConnectHeader {
			handleExit(ctx, exitSide)
		}
	}()

	conn, err := openExitSession(streamAdapter{entrySide}, ln.Addr().String())
	if err != nil {
		t.Fatalf("openExitSession: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	msg := []byte("hello through the exit")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	echo := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, echo); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(echo, msg) {
		t.Errorf("echo = %q, want %q", echo, msg)
	}
}

// TestTargetRecord verifies record encoding round-trips and rejects bad lengths.
func TestTargetRecord(t *testing.T) {
	var buf bytes.Buffer
	if err := writeTargetRecord(&buf, "example.com:443"); err != nil {
		t.Fatalf("writeTargetRecord: %v", err)
	}
	if buf.Len() != config.TargetRecordSize {
		t.Errorf("record size = %d, want %d", buf.Len(), config.TargetRecordSize)
	}
	got, err := readTargetRecord(&buf)
	if err != nil || got != "example.com:443" {
		t.Errorf("readTargetRecord = %q, %v", got, err)
	}

	for _, bad := range []string{"", strings.Repeat("x", config.TargetRecordSize-1)} {
		if err := writeTargetRecord(io.Discard, bad); err == nil {
			t.Errorf("writeTargetRecord accepted invalid length %d", len(bad))
		}
	}

	var zeroLen [config.TargetRecordSize]byte
	if _, err := readTargetRecord(bytes.NewReader(zeroLen[:])); err == nil {
		t.Error("readTargetRecord accepted a zero-length record")
	}
}

// TestRedactTarget verifies dial errors are logged without the destination.
func TestRedactTarget(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "tcp", Err: io.EOF,
		Addr: &net.TCPAddr{IP: net.IPv4(93, 184, 216, 34), Port: 443}}
	got := redactTarget(err, "93.184.216.34:443")
	if strings.Contains(got, "93.184.216.34") {
		t.Errorf("redactTarget leaked destination: %q", got)
	}
}
