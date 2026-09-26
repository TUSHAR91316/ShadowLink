package onion

import (
	"bytes"
	"crypto/cipher"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/shadowlink/core/internal/crypto"
)

func mustGenerateKey(t *testing.T) []byte {
	t.Helper()
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return k
}

// TestWrapUnwrap_SingleLayer verifies a single-key onion round-trip.
func TestWrapUnwrap_SingleLayer(t *testing.T) {
	key := mustGenerateKey(t)
	payload := []byte("single layer onion")

	wrapped, err := WrapPayload(payload, [][]byte{key})
	if err != nil {
		t.Fatalf("WrapPayload: %v", err)
	}

	unwrapped, err := UnwrapPayload(wrapped, key)
	if err != nil {
		t.Fatalf("UnwrapPayload: %v", err)
	}
	if !bytes.Equal(unwrapped, payload) {
		t.Errorf("single-layer round-trip mismatch: got %q", unwrapped)
	}
}

// TestWrapUnwrap_ThreeLayers verifies a full 3-hop onion (Entry, Relay, Exit keys).
// WrapPayload encrypts Entry→Relay→Exit; each UnwrapPayload peels one layer.
func TestWrapUnwrap_ThreeLayers(t *testing.T) {
	entryKey := mustGenerateKey(t)
	relayKey := mustGenerateKey(t)
	exitKey := mustGenerateKey(t)

	payload := []byte("secret data through 3 hops")
	keys := [][]byte{entryKey, relayKey, exitKey}

	// Wrap: innermost (exit) encrypted first, outermost (entry) last
	wrapped, err := WrapPayload(payload, keys)
	if err != nil {
		t.Fatalf("WrapPayload 3-layer: %v", err)
	}

	// Unwrap layer by layer: entry → relay → exit
	afterEntry, err := UnwrapPayload(wrapped, entryKey)
	if err != nil {
		t.Fatalf("UnwrapPayload entry layer: %v", err)
	}
	afterRelay, err := UnwrapPayload(afterEntry, relayKey)
	if err != nil {
		t.Fatalf("UnwrapPayload relay layer: %v", err)
	}
	afterExit, err := UnwrapPayload(afterRelay, exitKey)
	if err != nil {
		t.Fatalf("UnwrapPayload exit layer: %v", err)
	}

	if !bytes.Equal(afterExit, payload) {
		t.Errorf("3-layer round-trip mismatch: got %q, want %q", afterExit, payload)
	}
}

// TestWrapPayload_WrongKeyFails verifies that the AEAD tag prevents decryption with the wrong key.
func TestWrapPayload_WrongKeyFails(t *testing.T) {
	key := mustGenerateKey(t)
	wrongKey := mustGenerateKey(t)

	wrapped, _ := WrapPayload([]byte("private"), [][]byte{key})
	_, err := UnwrapPayload(wrapped, wrongKey)
	if err == nil {
		t.Error("UnwrapPayload must fail with the wrong key")
	}
}

// TestWrapPayload_TamperedDataFails verifies AEAD integrity protection.
func TestWrapPayload_TamperedDataFails(t *testing.T) {
	key := mustGenerateKey(t)
	wrapped, _ := WrapPayload([]byte("important"), [][]byte{key})
	wrapped[len(wrapped)-1] ^= 0xFF // flip a byte in the AEAD tag

	_, err := UnwrapPayload(wrapped, key)
	if err == nil {
		t.Error("UnwrapPayload must fail on tampered ciphertext")
	}
}

// TestWrapPayload_EmptyPayload verifies that empty payloads are handled correctly.
func TestWrapPayload_EmptyPayload(t *testing.T) {
	key := mustGenerateKey(t)
	wrapped, err := WrapPayload([]byte{}, [][]byte{key})
	if err != nil {
		t.Fatalf("WrapPayload empty: %v", err)
	}
	result, err := UnwrapPayload(wrapped, key)
	if err != nil {
		t.Fatalf("UnwrapPayload empty: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d bytes", len(result))
	}
}

// TestWrapPayload_EmptyKeys verifies that WrapPayload returns an error when
// called with no keys (previously it would silently return the unencrypted payload).
func TestWrapPayload_EmptyKeys(t *testing.T) {
	_, err := WrapPayload([]byte("secret"), [][]byte{})
	if err == nil {
		t.Error("WrapPayload with empty keys must return an error")
	}
}

// TestWrapPayloadWithBuffers_TwoLayers verifies ping-pong multi-layer wrapping
// with pre-allocated destination and scratch buffers.
func TestWrapPayloadWithBuffers_TwoLayers(t *testing.T) {
	relayKey := mustGenerateKey(t)
	exitKey := mustGenerateKey(t)

	c1, err := chacha20poly1305.NewX(relayKey)
	if err != nil {
		t.Fatalf("NewX relay: %v", err)
	}
	c2, err := chacha20poly1305.NewX(exitKey)
	if err != nil {
		t.Fatalf("NewX exit: %v", err)
	}
	ciphers := []cipher.AEAD{c1, c2}

	payload := []byte("ping pong buffer test message")
	// Needed for 2 layers: len(payload) + 80
	dst := make([]byte, 0, len(payload)+80)
	scratch := make([]byte, 0, len(payload)+40)

	wrapped, err := WrapPayloadWithBuffers(payload, ciphers, dst, scratch)
	if err != nil {
		t.Fatalf("WrapPayloadWithBuffers: %v", err)
	}

	// Verify wrapped aliases dst backing array
	if len(wrapped) != len(payload)+80 {
		t.Fatalf("expected len %d, got %d", len(payload)+80, len(wrapped))
	}
	if &wrapped[0] != &dst[:cap(dst)][0] {
		t.Errorf("expected wrapped to alias dst buffer")
	}

	// Peeling: first relayKey (outer layer 0), then exitKey (inner layer 1)
	unwrappedRelay, err := UnwrapPayload(wrapped, relayKey)
	if err != nil {
		t.Fatalf("UnwrapPayload relay: %v", err)
	}
	unwrappedExit, err := UnwrapPayload(unwrappedRelay, exitKey)
	if err != nil {
		t.Fatalf("UnwrapPayload exit: %v", err)
	}

	if !bytes.Equal(unwrappedExit, payload) {
		t.Fatalf("payload mismatch: got %q, want %q", unwrappedExit, payload)
	}
}

// TestWrapPayloadWithBuffers_ThreeLayers verifies 3-layer ping-pong wrapping.
func TestWrapPayloadWithBuffers_ThreeLayers(t *testing.T) {
	k1 := mustGenerateKey(t)
	k2 := mustGenerateKey(t)
	k3 := mustGenerateKey(t)

	ciphers := make([]cipher.AEAD, 3)
	ciphers[0], _ = chacha20poly1305.NewX(k1)
	ciphers[1], _ = chacha20poly1305.NewX(k2)
	ciphers[2], _ = chacha20poly1305.NewX(k3)

	payload := []byte("three layers ping pong")
	dst := make([]byte, 0, len(payload)+120)
	scratch := make([]byte, 0, len(payload)+80)

	wrapped, err := WrapPayloadWithBuffers(payload, ciphers, dst, scratch)
	if err != nil {
		t.Fatalf("WrapPayloadWithBuffers: %v", err)
	}

	if &wrapped[0] != &dst[:cap(dst)][0] {
		t.Errorf("expected wrapped to alias dst buffer")
	}

	p1, err := UnwrapPayload(wrapped, k1)
	if err != nil {
		t.Fatalf("layer 1: %v", err)
	}
	p2, err := UnwrapPayload(p1, k2)
	if err != nil {
		t.Fatalf("layer 2: %v", err)
	}
	p3, err := UnwrapPayload(p2, k3)
	if err != nil {
		t.Fatalf("layer 3: %v", err)
	}

	if !bytes.Equal(p3, payload) {
		t.Fatalf("payload mismatch: got %q, want %q", p3, payload)
	}
}

