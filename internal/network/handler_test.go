package network

import (
	"strings"
	"testing"
)

// TestReadLineRaw_Simple verifies a normal newline-terminated line is parsed correctly.
func TestReadLineRaw_Simple(t *testing.T) {
	r := strings.NewReader("hello world\nmore data")
	line, err := readLineRaw(r)
	if err != nil {
		t.Fatalf("readLineRaw: %v", err)
	}
	if line != "hello world" {
		t.Errorf("got %q, want %q", line, "hello world")
	}
}

// TestReadLineRaw_StopsAtNewline verifies that readLineRaw does NOT consume
// bytes after the newline — critical for ECDH key exchange correctness.
func TestReadLineRaw_StopsAtNewline(t *testing.T) {
	input := "RELAY\n\x01\x02\x03" // ECDH key bytes follow the newline
	r := strings.NewReader(input)

	line, err := readLineRaw(r)
	if err != nil {
		t.Fatalf("readLineRaw: %v", err)
	}
	if line != "RELAY" {
		t.Errorf("got %q, want %q", line, "RELAY")
	}

	// The remaining bytes must still be available in the reader
	remaining := make([]byte, 3)
	n, _ := r.Read(remaining)
	if n != 3 || remaining[0] != 0x01 || remaining[1] != 0x02 || remaining[2] != 0x03 {
		t.Errorf("bytes after newline were consumed or corrupted: %v", remaining[:n])
	}
}

// TestReadLineRaw_TrimSpace verifies that trailing carriage returns and spaces are stripped.
func TestReadLineRaw_TrimSpace(t *testing.T) {
	r := strings.NewReader("example.com:443\r\n")
	line, err := readLineRaw(r)
	if err != nil {
		t.Fatalf("readLineRaw: %v", err)
	}
	if line != "example.com:443" {
		t.Errorf("got %q, want %q", line, "example.com:443")
	}
}

// TestReadLineRaw_EmptyLine verifies an empty line (just a newline) returns an empty string.
func TestReadLineRaw_EmptyLine(t *testing.T) {
	r := strings.NewReader("\n")
	line, err := readLineRaw(r)
	if err != nil {
		t.Fatalf("readLineRaw: %v", err)
	}
	if line != "" {
		t.Errorf("got %q, want empty string", line)
	}
}
