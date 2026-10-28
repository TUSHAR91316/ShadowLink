package network

import (
	"strings"
	"testing"

	"github.com/shadowlink/core/internal/config"
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

// TestReadLineRaw_ExceedsMaxLineLen verifies that lines exceeding maxLineLen return an error.
func TestReadLineRaw_ExceedsMaxLineLen(t *testing.T) {
	// Create an input stream of 4097 bytes without a newline
	oversized := strings.Repeat("A", 4097)
	r := strings.NewReader(oversized)

	_, err := readLineRaw(r)
	if err == nil {
		t.Fatal("expected error when line exceeds maxLineLen, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum allowed") {
		t.Errorf("expected max length error message, got: %v", err)
	}
}


// TestRoleAllows verifies each role only serves the commands it opted into —
// in particular that a relay-only node never acts as an exit.
func TestRoleAllows(t *testing.T) {
	cases := []struct {
		role, cmd string
		want      bool
	}{
		{config.RoleRelay, config.ExtendHeader, true},
		{config.RoleRelay, config.ConnectHeader, false},
		{config.RoleExit, config.ConnectHeader, true},
		{config.RoleExit, config.ExtendHeader, false},
		{config.RoleRelayExit, config.ExtendHeader, true},
		{config.RoleRelayExit, config.ConnectHeader, true},
		{config.RoleRelayExit, "BOGUS", false},
		{"", config.ConnectHeader, false},
	}
	for _, c := range cases {
		if got := roleAllows(c.role, c.cmd); got != c.want {
			t.Errorf("roleAllows(%q, %q) = %v, want %v", c.role, c.cmd, got, c.want)
		}
	}
}

// TestExitDialControl verifies exits refuse internal targets but allow public ones.
func TestExitDialControl(t *testing.T) {
	blocked := []string{
		"127.0.0.1:80", "[::1]:80", "10.1.2.3:22", "192.168.1.1:80", "172.16.0.5:443",
		"169.254.169.254:80", "[fe80::1]:80", "[fc00::1]:80", "0.0.0.0:80",
		"100.64.0.1:80", "255.255.255.255:80", "224.0.0.1:80", "[::ffff:127.0.0.1]:80",
	}
	for _, addr := range blocked {
		if err := exitDialControl("tcp", addr, nil); err == nil {
			t.Errorf("exitDialControl(%s) = nil, want refusal", addr)
		}
	}
	allowed := []string{"1.1.1.1:443", "93.184.216.34:80", "[2606:4700:4700::1111]:443"}
	for _, addr := range allowed {
		if err := exitDialControl("tcp", addr, nil); err != nil {
			t.Errorf("exitDialControl(%s) = %v, want nil", addr, err)
		}
	}
}
