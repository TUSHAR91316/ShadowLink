package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// Encrypt encrypts plaintext using XChaCha20-Poly1305 with a random 24-byte nonce.
// The returned slice is: [nonce (24 bytes)][ciphertext+tag].
// The key must be exactly 32 bytes (chacha20poly1305.KeySize).
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size: got %d, want %d bytes", len(key), chacha20poly1305.KeySize)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("chacha20poly1305 init: %w", err)
	}

	// Allocate the output in one shot: nonce || ciphertext || tag.
	nonceSize := aead.NonceSize()
	out := make([]byte, nonceSize+len(plaintext)+aead.Overhead())
	nonce := out[:nonceSize]

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation: %w", err)
	}

	// Seal appends ciphertext+tag directly after the nonce in out.
	aead.Seal(out[nonceSize:nonceSize], nonce, plaintext, nil)
	return out, nil
}

// Decrypt verifies and decrypts data produced by Encrypt.
// Returns an error if the key is wrong, the data is too short, or the AEAD tag fails.
func Decrypt(key, encrypted []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size: got %d, want %d bytes", len(key), chacha20poly1305.KeySize)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("chacha20poly1305 init: %w", err)
	}

	nonceSize := aead.NonceSize()
	minLen := nonceSize + aead.Overhead()
	if len(encrypted) < minLen {
		return nil, fmt.Errorf("ciphertext too short: got %d bytes, minimum %d", len(encrypted), minLen)
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Do not expose the raw AEAD error — it may leak timing information.
		return nil, errors.New("decryption failed: authentication tag mismatch or data tampered")
	}
	return plaintext, nil
}

// GenerateKey generates a cryptographically random 32-byte key suitable for
// XChaCha20-Poly1305 or any HKDF-derived key slot.
// NOTE: ECDH session keys are derived by PerformECDH / RespondECDH and must NOT
// be generated with this function.
func GenerateKey() ([]byte, error) {
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("key generation: %w", err)
	}
	return key, nil
}
