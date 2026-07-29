package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"

	"github.com/shadowlink/core/internal/config"
)

// X25519KeySize is the length of an X25519 public key in bytes.
const X25519KeySize = 32

// PerformECDH is the INITIATOR side of an ephemeral X25519 Diffie-Hellman handshake.
// It sends its public key first, then reads the peer's public key.
// Used by: Entry node (to Relay), Relay node (to Exit).
// Returns a 32-byte derived key ready for use with ChaCha20-Poly1305.
func PerformECDH(rw io.ReadWriter) ([]byte, error) {
	return doECDH(rw, true)
}

// RespondECDH is the RESPONDER side of an ephemeral X25519 Diffie-Hellman handshake.
// It reads the peer's public key first, then sends its own.
// Used by: Relay node (from Entry), Exit node (from Relay or Entry).
// Returns a 32-byte derived key ready for use with ChaCha20-Poly1305.
func RespondECDH(rw io.ReadWriter) ([]byte, error) {
	return doECDH(rw, false)
}

// doECDH performs the X25519 key exchange followed by HKDF-SHA256 key derivation.
//
// Both parties generate a fresh ephemeral key pair per session — forward secrecy guaranteed.
// The raw X25519 shared secret is NOT used directly as an encryption key. Instead, it is
// passed through HKDF-SHA256 with config.HKDFInfo for domain separation, ensuring the
// derived key material is cryptographically uniform and protocol-bound.
func doECDH(rw io.ReadWriter, initiator bool) ([]byte, error) {
	curve := ecdh.X25519()

	// Generate a fresh, random ephemeral key pair.
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ecdh: key generation failed: %w", err)
	}
	ownPubBytes := privKey.PublicKey().Bytes() // always 32 bytes for X25519
	peerPubBytes := make([]byte, X25519KeySize)

	if initiator {
		// Initiator sends its public key first, then waits for the peer's.
		if _, err := rw.Write(ownPubBytes); err != nil {
			return nil, fmt.Errorf("ecdh: failed to write public key: %w", err)
		}
		if _, err := io.ReadFull(rw, peerPubBytes); err != nil {
			return nil, fmt.Errorf("ecdh: failed to read peer public key: %w", err)
		}
	} else {
		// Responder reads the initiator's public key first, then sends its own.
		if _, err := io.ReadFull(rw, peerPubBytes); err != nil {
			return nil, fmt.Errorf("ecdh: failed to read peer public key: %w", err)
		}
		if _, err := rw.Write(ownPubBytes); err != nil {
			return nil, fmt.Errorf("ecdh: failed to write public key: %w", err)
		}
	}

	peerKey, err := curve.NewPublicKey(peerPubBytes)
	if err != nil {
		return nil, fmt.Errorf("ecdh: invalid peer public key: %w", err)
	}

	// Compute the raw X25519 shared secret. Both sides derive the same value independently.
	rawSecret, err := privKey.ECDH(peerKey)
	if err != nil {
		return nil, fmt.Errorf("ecdh: key agreement failed: %w", err)
	}

	// Derive the final encryption key using HKDF-SHA256.
	// config.HKDFInfo provides domain separation; changing it is a breaking protocol change.
	hkdfReader := hkdf.New(sha256.New, rawSecret, nil, []byte(config.HKDFInfo))
	derivedKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, derivedKey); err != nil {
		return nil, fmt.Errorf("ecdh: HKDF key derivation failed: %w", err)
	}

	return derivedKey, nil
}
