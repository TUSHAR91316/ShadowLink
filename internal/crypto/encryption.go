package crypto

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// Encrypt payload using XChaCha20-Poly1305 with a random 24-byte nonce.
// Returns the nonce appended to the ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, errors.New("invalid key size: must be 32 bytes")
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	// Generate 24-byte random nonce for XChaCha20-Poly1305
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends the ciphertext to the provided slice (nil in this case),
	// and we prepend the nonce so it can be extracted during decryption.
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	
	// Prepend nonce to ciphertext
	result := append(nonce, ciphertext...)
	return result, nil
}

// Decrypt extracts the nonce and decrypts the payload using XChaCha20-Poly1305.
func Decrypt(key, encrypted []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, errors.New("invalid key size: must be 32 bytes")
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract nonce and actual ciphertext
	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	// Open decrypts the ciphertext and verifies the authentication tag
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed or data tampered with")
	}

	return plaintext, nil
}

// GenerateKey generates a random 32-byte key for ChaCha20-Poly1305.
// This is primarily used in tests and for one-off symmetric encryption.
// ECDH session keys are derived automatically by PerformECDH / RespondECDH
// and should NOT be generated with this function.
func GenerateKey() ([]byte, error) {
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

