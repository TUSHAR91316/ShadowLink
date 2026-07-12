package crypto

import (
	"bytes"
	"io"
	"testing"
)

// pipeReadWriter is a synchronous, in-process io.ReadWriter backed by two
// byte pipes — one for each direction of an ECDH exchange.
type pipeReadWriter struct {
	r io.Reader
	w io.Writer
}

func (p *pipeReadWriter) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipeReadWriter) Write(b []byte) (int, error) { return p.w.Write(b) }

// makePair creates a symmetric pair of connected pipeReadWriters.
func makePair() (*pipeReadWriter, *pipeReadWriter) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &pipeReadWriter{r: r1, w: w2}, &pipeReadWriter{r: r2, w: w1}
}

// TestECDH_SharedSecretMatches verifies that initiator and responder derive
// the same shared secret (the fundamental correctness property of ECDH).
func TestECDH_SharedSecretMatches(t *testing.T) {
	initiatorSide, responderSide := makePair()

	var initiatorKey, responderKey []byte
	var initiatorErr, responderErr error

	done := make(chan struct{})
	go func() {
		responderKey, responderErr = RespondECDH(responderSide)
		close(done)
	}()

	initiatorKey, initiatorErr = PerformECDH(initiatorSide)
	<-done

	if initiatorErr != nil {
		t.Fatalf("PerformECDH error: %v", initiatorErr)
	}
	if responderErr != nil {
		t.Fatalf("RespondECDH error: %v", responderErr)
	}
	if !bytes.Equal(initiatorKey, responderKey) {
		t.Errorf("shared secrets do not match:\n  initiator: %x\n  responder: %x", initiatorKey, responderKey)
	}
}

// TestECDH_KeyLength verifies the derived shared secret is exactly 32 bytes
// (the required size for ChaCha20-Poly1305).
func TestECDH_KeyLength(t *testing.T) {
	a, b := makePair()
	var key []byte
	done := make(chan struct{})
	go func() { RespondECDH(b); close(done) }()
	key, _ = PerformECDH(a)
	<-done

	if len(key) != 32 {
		t.Errorf("expected 32-byte shared secret, got %d bytes", len(key))
	}
}

// TestECDH_ForwardSecrecy verifies that two separate handshakes produce
// different shared secrets (ephemeral keys, no session reuse).
func TestECDH_ForwardSecrecy(t *testing.T) {
	a1, b1 := makePair()
	a2, b2 := makePair()

	var key1, key2 []byte
	done1 := make(chan struct{})
	done2 := make(chan struct{})

	go func() { RespondECDH(b1); close(done1) }()
	go func() { RespondECDH(b2); close(done2) }()

	key1, _ = PerformECDH(a1)
	key2, _ = PerformECDH(a2)
	<-done1
	<-done2

	if bytes.Equal(key1, key2) {
		t.Error("two separate ECDH handshakes must produce different shared secrets (forward secrecy broken)")
	}
}

// TestECDH_EncryptDecryptWithDerivedKey verifies that the ECDH-derived key
// works correctly as input to our ChaCha20-Poly1305 Encrypt/Decrypt functions.
func TestECDH_EncryptDecryptWithDerivedKey(t *testing.T) {
	a, b := makePair()

	var initiatorKey, responderKey []byte
	done := make(chan struct{})
	go func() {
		responderKey, _ = RespondECDH(b)
		close(done)
	}()
	initiatorKey, _ = PerformECDH(a)
	<-done

	message := []byte("encrypted with ECDH session key")
	ct, err := Encrypt(initiatorKey, message)
	if err != nil {
		t.Fatalf("Encrypt with ECDH key: %v", err)
	}
	pt, err := Decrypt(responderKey, ct)
	if err != nil {
		t.Fatalf("Decrypt with ECDH key: %v", err)
	}
	if !bytes.Equal(pt, message) {
		t.Errorf("round-trip mismatch: got %q", pt)
	}
}
