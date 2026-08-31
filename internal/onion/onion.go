package onion

import (
	"crypto/cipher"
	"errors"
	"fmt"

	"github.com/shadowlink/core/internal/crypto"
)

// WrapPayload applies multiple layers of encryption to payload (innermost first).
//
// Key ordering: keys[0] is the outermost layer (entry node), keys[len-1] is the
// innermost layer (exit node). Encryption is applied from exit → entry so that
// during routing, each node can peel exactly one layer.
//
// Returns an error if keys is empty (no encryption would be applied).
func WrapPayload(payload []byte, keys [][]byte) ([]byte, error) {
	if len(keys) == 0 {
		return nil, errors.New("onion: WrapPayload requires at least one key")
	}

	current := payload
	var err error
	// Encrypt from the innermost (exit) key to the outermost (entry) key.
	for i := len(keys) - 1; i >= 0; i-- {
		current, err = crypto.Encrypt(keys[i], current)
		if err != nil {
			return nil, fmt.Errorf("onion: WrapPayload at key index %d: %w", i, err)
		}
	}
	return current, nil
}

// WrapPayloadWithCiphers wraps payload through pre-instantiated AEAD ciphers in reverse order.
// When a single cipher is provided, dst is used directly with zero allocations.
func WrapPayloadWithCiphers(payload []byte, ciphers []cipher.AEAD, dst []byte) ([]byte, error) {
	if len(ciphers) == 0 {
		return nil, errors.New("onion: WrapPayloadWithCiphers requires at least one cipher")
	}

	if len(ciphers) == 1 {
		return crypto.EncryptWithAEAD(ciphers[0], payload, dst)
	}

	// For multi-layer wrapping, sequentially wrap to avoid buffer overlap corruption.
	current := payload
	var err error
	for i := len(ciphers) - 1; i >= 0; i-- {
		current, err = crypto.EncryptWithAEAD(ciphers[i], current, nil)
		if err != nil {
			return nil, fmt.Errorf("onion: WrapPayloadWithCiphers at layer %d: %w", i, err)
		}
	}
	return current, nil
}

// UnwrapPayload peels one layer of encryption from encryptedPayload using key.
// Each node in the circuit calls this once with its own session key.
func UnwrapPayload(encryptedPayload []byte, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errors.New("onion: UnwrapPayload requires a non-empty key")
	}
	plaintext, err := crypto.Decrypt(key, encryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("onion: UnwrapPayload: %w", err)
	}
	return plaintext, nil
}

// UnwrapPayloadInPlace peels one layer of encryption in-place using a pre-initialized AEAD cipher.
func UnwrapPayloadInPlace(encryptedPayload []byte, aead cipher.AEAD) ([]byte, error) {
	if aead == nil {
		return nil, errors.New("onion: UnwrapPayloadInPlace requires a non-nil cipher")
	}
	plaintext, err := crypto.DecryptWithAEADInPlace(aead, encryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("onion: UnwrapPayloadInPlace: %w", err)
	}
	return plaintext, nil
}
