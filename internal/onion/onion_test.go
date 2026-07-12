package onion

import (
	"bytes"
	"testing"

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
