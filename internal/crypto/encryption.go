package crypto

import (
	"crypto/cipher"
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

	return EncryptWithAEAD(aead, plaintext, nil)
}

// EncryptWithAEAD writes [24-byte nonce][ciphertext + 16-byte tag] into dst.
// If dst has sufficient capacity (len(plaintext) + 40), no heap allocation occurs.
func EncryptWithAEAD(aead cipher.AEAD, plaintext, dst []byte) ([]byte, error) {
	nonceSize := aead.NonceSize()
	overhead := aead.Overhead()
	needed := nonceSize + len(plaintext) + overhead

	if cap(dst) < needed {
		dst = make([]byte, needed)
	} else {
		dst = dst[:needed]
	}

	nonce := dst[:nonceSize]
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation: %w", err)
	}

	// Seal writes ciphertext + 16-byte tag directly after nonce in dst.
	aead.Seal(dst[nonceSize:nonceSize], nonce, plaintext, nil)
	return dst, nil
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

	return DecryptWithAEADInPlace(aead, encrypted)
}

// DecryptWithAEADInPlace verifies and decrypts data in-place using an existing AEAD instance.
// The plaintext is written directly into encrypted[nonceSize:], eliminating heap allocation.
func DecryptWithAEADInPlace(aead cipher.AEAD, encrypted []byte) ([]byte, error) {
	nonceSize := aead.NonceSize()
	minLen := nonceSize + aead.Overhead()
	if len(encrypted) < minLen {
		return nil, fmt.Errorf("ciphertext too short: got %d bytes, minimum %d", len(encrypted), minLen)
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	// Decrypt in-place into ciphertext[:0] without heap allocations
	plaintext, err := aead.Open(ciphertext[:0], nonce, ciphertext, nil)
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
