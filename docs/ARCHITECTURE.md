# ShadowLink Architecture (Technical Deep-Dive)

> **Note**: For the high-level overview, see [ARCHITECTURE.md](../ARCHITECTURE.md).
> This document covers the internals used for developer reference.

---

## Packet Lifecycle — Entry to Internet

```
Browser
  │  HTTP CONNECT example.com:443
  ▼
SOCKS5 Server (127.0.0.1:1080)
  │  net.Conn → custom dialer → DialCircuit()
  ▼
circuit.go — DialCircuit()
  │  FindPeers("shadowlink-relay") → [relay1, relay2]
  │  FindPeers("shadowlink-exit")  → [exit1, exit2]
  │  tryViaRelay(relay1, "example.com:443")
  │    │  Host.Connect(relay1)
  │    │  NewStream(relay1, ProtocolID)
  │    │  stream.Write("RELAY\nexample.com:443\n")
  │    │  PerformECDH(stream) → sessionKey K₁
  │    └─ return libP2PConn{stream, K₁}
  ▼
libP2PConn.Write(plaintext)
  │  onion.WrapPayload(plaintext, [K₁])  →  ciphertext
  │  binary.BigEndian.PutUint32(len)     →  4-byte header
  │  stream.Write(header + ciphertext)
  ▼
  [on relay's side]
handler.go — handleRelay()
  │  RespondECDH(stream) → K₁
  │  FindPeers("shadowlink-exit") → [exit1]
  │  NewStream(exit1, ProtocolID)
  │  exitStream.Write("example.com:443\n")
  │  PerformECDH(exitStream) → K₂
  │  upstreamConn  = libP2PConn{stream,     K₁}
  │  downstreamConn = libP2PConn{exitStream, K₂}
  │  io.Copy(downstreamConn, upstreamConn)
  ▼
  [on exit's side]
handler.go — handleExit()
  │  RespondECDH(stream) → K₂
  │  net.Dial("tcp", "example.com:443")
  │  io.Copy(outConn, libP2PConn{stream, K₂})
  ▼
example.com:443 (Public Internet)
```

---

## Internal Package Reference

### `internal/crypto`
| Function | Signature | Description |
|---|---|---|
| `GenerateKey()` | `([]byte, error)` | Generates a random 32-byte ChaCha20 key |
| `Encrypt(key, plaintext)` | `([]byte, error)` | XChaCha20-Poly1305 encrypt with random nonce |
| `Decrypt(key, ciphertext)` | `([]byte, error)` | Verify AEAD tag and decrypt |
| `PerformECDH(rw)` | `([]byte, error)` | Initiator X25519: send pubkey first |
| `RespondECDH(rw)` | `([]byte, error)` | Responder X25519: read pubkey first |

### `internal/network`
| Type/Function | Description |
|---|---|
| `DialCircuit(ctx, ds, network, addr)` | Returns `net.Conn` routed through dVPN |
| `HandleStream(s, role, ds)` | Dispatches relay or exit logic for incoming streams |
| `libP2PConn` | `net.Conn` wrapper with 4-byte length-prefix framing + AEAD |

### `internal/discovery`
| Function | Description |
|---|---|
| `NewDiscoveryService(ctx, port, bootstraps)` | Creates libp2p host + KadDHT |
| `Announce(ctx, rendezvous)` | Publishes presence under a rendezvous key |
| `FindPeers(ctx, rendezvous)` | Returns `[]peer.AddrInfo` of matching peers |

### `internal/onion`
| Function | Description |
|---|---|
| `WrapPayload(data, keys)` | Encrypts with each key from last→first (outermost = first key) |
| `UnwrapPayload(data, key)` | Peels exactly one encryption layer |

### `internal/socks5`
| Function | Description |
|---|---|
| `NewServer(port, dialer)` | Creates a SOCKS5 server with a custom dialer |
| `ListenAndServe(ctx)` | Blocks until ctx is cancelled; returns nil on clean shutdown |

### `internal/sysproxy`
| Function | Platform | Description |
|---|---|---|
| `EnableSOCKS5(host, port)` | Windows | Writes `SOCKS=host:port` to HKCU registry |
| `Disable()` | Windows | Clears `ProxyEnable` in registry |
| `Disable()` | macOS/Linux | no-op, returns nil |

---

## Wire Protocol

```
Stream from Entry to Relay (or Entry to Exit):
  Byte 0..N:  "<target:port>\n"  or  "RELAY\n<target:port>\n"   (ASCII text)
  Byte N+1..N+32:  Initiator X25519 public key                  (raw 32 bytes)
  Byte N+33..N+64: Responder X25519 public key                  (raw 32 bytes)
  [All subsequent bytes are length-framed encrypted frames:]
    Bytes 0..3:   uint32 big-endian frame length
    Bytes 4..N:   XChaCha20-Poly1305 ciphertext
```

---

## Test Coverage

| Package | Test File | Cases |
|---|---|---|
| `internal/crypto` | `encryption_test.go` | 8 — Encrypt/Decrypt round-trips, tamper, wrong key |
| `internal/crypto` | `ecdh_test.go` | 4 — Shared secret equality, length, forward secrecy |
| `internal/onion` | `onion_test.go` | 5 — 1-layer, 3-layer peel, tamper, empty |
| `internal/network` | `framing_test.go` | 4 — Round-trip, multi-message, wrong key, large |
| `internal/network` | `handler_test.go` | 4 — `readLineRaw` boundary correctness |
| `internal/socks5` | `server_test.go` | 4 — Nil dialer, shutdown, port-in-use |

Run: `go test ./... -count=1 -race`
