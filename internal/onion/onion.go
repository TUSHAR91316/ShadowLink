package onion

import (
	"github.com/shadowlink/core/internal/crypto"
)

// WrapPayload wraps a payload in multiple layers of encryption.
// Keys should be ordered from entry node to exit node.
// e.g. keys[0] = EntryKey, keys[1] = RelayKey, keys[2] = ExitKey.
// The payload is encrypted from exit back to entry.
func WrapPayload(payload []byte, keys [][]byte) ([]byte, error) {
	current := payload
	var err error

	// Encrypt from the innermost (exit) to outermost (entry)
	for i := len(keys) - 1; i >= 0; i-- {
		current, err = crypto.Encrypt(keys[i], current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// UnwrapPayload peels off the outermost layer of encryption.
// A node calls this with its symmetric session key.
func UnwrapPayload(encryptedPayload []byte, key []byte) ([]byte, error) {
	return crypto.Decrypt(key, encryptedPayload)
}
