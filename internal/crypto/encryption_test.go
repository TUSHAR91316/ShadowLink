package crypto

import (
	"bytes"
	"testing"
)

// --- GenerateKey ---

func TestGenerateKey_Length(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(key))
	}
}

func TestGenerateKey_Uniqueness(t *testing.T) {
	k1, _ := GenerateKey()
	k2, _ := GenerateKey()
	if bytes.Equal(k1, k2) {
		t.Error("two consecutive GenerateKey calls returned identical keys (RNG failure?)")
	}
}

// --- Encrypt / Decrypt ---

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("hello shadowlink")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext must differ from plaintext")
	}

	recovered, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(recovered, plaintext) {
		t.Errorf("decrypted = %q, want %q", recovered, plaintext)
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	key, _ := GenerateKey()
	ct, err := Encrypt(key, []byte{})
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(pt) != 0 {
		t.Errorf("expected empty plaintext, got %d bytes", len(pt))
	}
}

func TestEncrypt_RandomNonce(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("same message")

	ct1, _ := Encrypt(key, plaintext)
	ct2, _ := Encrypt(key, plaintext)
	if bytes.Equal(ct1, ct2) {
		t.Error("two encryptions of the same plaintext must produce different ciphertexts (nonce reuse)")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	ct, _ := Encrypt(key, []byte("important data"))
	ct[len(ct)-1] ^= 0xFF // flip last byte (AEAD tag)

	_, err := Decrypt(key, ct)
	if err == nil {
		t.Error("Decrypt must fail on tampered ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	ct, _ := Encrypt(key1, []byte("secret"))

	_, err := Decrypt(key2, ct)
	if err == nil {
		t.Error("Decrypt must fail with wrong key")
	}
}

func TestEncrypt_InvalidKeySize(t *testing.T) {
	badKey := []byte("short")
	_, err := Encrypt(badKey, []byte("data"))
	if err == nil {
		t.Error("Encrypt must reject keys that are not 32 bytes")
	}
}

func TestDecrypt_TooShortCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	_, err := Decrypt(key, []byte("tiny"))
	if err == nil {
		t.Error("Decrypt must reject ciphertext shorter than nonce size")
	}
}

func TestEncryptDecrypt_LargePayload(t *testing.T) {
	key, _ := GenerateKey()
	large := make([]byte, 64*1024) // 64 KB
	for i := range large {
		large[i] = byte(i % 256)
	}
	ct, err := Encrypt(key, large)
	if err != nil {
		t.Fatalf("Encrypt large: %v", err)
	}
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt large: %v", err)
	}
	if !bytes.Equal(pt, large) {
		t.Error("large payload round-trip mismatch")
	}
}

// --- Benchmarks ---

func BenchmarkEncrypt_1KB(b *testing.B) {
	key, _ := GenerateKey()
	data := make([]byte, 1024)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = Encrypt(key, data)
	}
}

func BenchmarkDecrypt_1KB(b *testing.B) {
	key, _ := GenerateKey()
	data := make([]byte, 1024)
	ct, _ := Encrypt(key, data)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Make a copy so in-place decrypt doesn't corrupt subsequent runs
		ctCopy := append([]byte(nil), ct...)
		_, _ = Decrypt(key, ctCopy)
	}
}
