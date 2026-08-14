# 🔬 ShadowLink Architecture — Technical Deep-Dive

> **Reference**: For the high-level architecture overview, see [ARCHITECTURE.md](../ARCHITECTURE.md).
> This document specifies the low-level protocols, internal package APIs, and wire formats for developers and protocol engineers.

---

## 1. Complete Packet Lifecycle: Application to Internet

```
[ Application / Browser ]
         │ (HTTP/HTTPS, TCP over SOCKS5 on 127.0.0.1:1080)
         ▼
[ internal/socks5 — Server.ListenAndServe() ]
         │ Intercepts connection, reads destination (e.g., example.com:443)
         │ Invokes custom dialer: dialer(ctx, "tcp", "example.com:443")
         ▼
[ internal/network — DialCircuit() ]
         │ 1. Queries DHT for active relays ("shadowlink-relay") and exits ("shadowlink-exit")
         │ 2. Randomizes peer lists via Fisher-Yates shuffle
         │ 3. Selects (relayPeer, exitPeer) combination
         │ 4. Calls tryViaRelay(ctx, ds, relayPeer, exitPeer, "example.com:443")
         ▼
[ Handshake Phase: Entry ↔ Relay Hop ]
         │ Entry dials relay via libp2p Host.NewStream(relayPeer, "/shadowlink/1.0.0")
         │ Entry sends: "EXTEND\n<exitPeer.ID>\n"
         │ Entry & Relay perform X25519 ECDH → derives RelayKey via HKDF-SHA256
         │ Entry wraps stream into relayConn: libP2PConn{Conn: stream, Keys: [RelayKey]}
         ▼
[ Handshake Phase: Entry ↔ Exit Hop (Tunneled Through Relay) ]
         │ Relay resolves exitPeer via Kad-DHT and dials NewStream(exitPeer, "/shadowlink/1.0.0")
         │ Relay creates upstreamConn: libP2PConn{Conn: s, Keys: [RelayKey]}
         │ Relay calls bridge(upstreamConn, streamAdapter{exitStream}) — transparent byte forwarding
         │ Entry writes through relayConn: "CONNECT\nexample.com:443\n"
         │ Relay decrypts outer RelayKey layer and passes raw "CONNECT\nexample.com:443\n" to Exit
         │ Entry & Exit perform X25519 ECDH through relay tunnel → derives ExitKey via HKDF-SHA256
         │ Entry wraps relayConn into exitConn: libP2PConn{Conn: relayConn, Keys: [ExitKey]}
         ▼
[ Data Transfer Phase: Onion Encapsulation ]
         │ Application writes N plaintext bytes to exitConn:
         │ 1. exitConn encrypts with ExitKey → ciphertext1
         │ 2. exitConn writes to relayConn with 4-byte BE length header
         │ 3. relayConn encrypts with RelayKey → ciphertext2
         │ 4. relayConn writes [4-byte BE length][ciphertext2] to wire
         ▼
[ Relay Transit Hop ]
         │ 1. Relay reads 4-byte length header from stream
         │ 2. Relay reads ciphertext2 from stream into reusable buffer
         │ 3. Relay decrypts ciphertext2 with RelayKey → yields [4-byte header][ciphertext1]
         │ 4. Relay forwards [4-byte header][ciphertext1] to exitStream (Relay NEVER sees plaintext)
         ▼
[ Exit Egress Hop ]
         │ 1. Exit reads 4-byte length header from exitStream
         │ 2. Exit reads ciphertext1 into reusable buffer
         │ 3. Exit decrypts ciphertext1 with ExitKey → yields original N plaintext bytes
         │ 4. Exit dials net.Dialer.DialContext(ctx, "tcp", "example.com:443")
         │ 5. Exit proxies bidirectionally between encrypted dVPN stream and cleartext TCP socket
         ▼
[ Target Host: example.com:443 (Public Internet) ]
```

---

## 2. Internal Package API Specification

### `internal/config`
Defines protocol-wide constants to eliminate magic values:
| Constant | Value | Purpose |
|---|---|---|
| `ProtocolID` | `"/shadowlink/1.0.0"` | libp2p stream protocol identifier |
| `ExtendHeader` | `"EXTEND"` | Sentinel command to relay nodes for circuit extension |
| `ConnectHeader` | `"CONNECT"` | Sentinel command to exit nodes for internet egress |
| `RendezvousRelay` | `"shadowlink-relay"` | Kad-DHT rendezvous namespace for relay nodes |
| `RendezvousExit` | `"shadowlink-exit"` | Kad-DHT rendezvous namespace for exit nodes |
| `DefaultP2PPort` | `9000` | Default listening port for libp2p incoming streams |
| `DefaultSOCKSPort` | `1080` | Default listening port for local SOCKS5 proxy |
| `MaxFrameSize` | `131072` (128 KiB) | Maximum allowed frame size to prevent OOM attacks |
| `HKDFInfo` | `"shadowlink/v1/session-key"` | Cryptographic domain separation string for HKDF-SHA256 |

---

### `internal/crypto`
Provides primitives for forward secrecy and authenticated encryption:
| Function | Signature | Description |
|---|---|---|
| `PerformECDH(rw)` | `(io.ReadWriter) ([]byte, error)` | Initiator side: generates ephemeral X25519 key, sends public key, reads peer public key, derives 32-byte key via HKDF-SHA256. |
| `RespondECDH(rw)` | `(io.ReadWriter) ([]byte, error)` | Responder side: reads peer public key, generates ephemeral X25519 key, sends public key, derives 32-byte key via HKDF-SHA256. |
| `Encrypt(key, plaintext)` | `([]byte, []byte) ([]byte, error)` | Encrypts via XChaCha20-Poly1305 with single allocation: `[24-byte nonce][ciphertext + 16-byte tag]`. |
| `Decrypt(key, ciphertext)` | `([]byte, []byte) ([]byte, error)` | Extracts nonce, decrypts, and verifies AEAD Poly1305 authentication tag. |
| `GenerateKey()` | `() ([]byte, error)` | Generates 32 cryptographically secure random bytes (used for testing). |

---

### `internal/network`
Implements the core onion circuit negotiation, framing, and stream dispatching:
| Type / Function | Description |
|---|---|
| `DialCircuit(ctx, ds, network, addr)` | Entrypoint: discovers DHT peers, builds 3-hop onion circuit (or 1-hop fallback), returns `net.Conn`. |
| `HandleStream(ctx, s, role, ds)` | Stream dispatcher: reads initial command (`EXTEND` or `CONNECT`) and routes to `handleRelay` or `handleExit`. |
| `libP2PConn` | `net.Conn` implementation with 4-byte BE length prefix framing, layered encryption, and zero-allocation read buffers. |
| `streamAdapter` | Adapts libp2p `network.Stream` to `net.Conn` interface by providing stub address methods. |

---

### `internal/onion`
Encapsulates recursive layered onion wrapping and peeling:
| Function | Signature | Description |
|---|---|---|
| `WrapPayload(data, keys)` | `([]byte, [][]byte) ([]byte, error)` | Encrypts data with keys in reverse order (`keys[len-1]` innermost → `keys[0]` outermost). Validates non-empty keys. |
| `UnwrapPayload(data, key)` | `([]byte, []byte) ([]byte, error)` | Peels exactly one encryption layer using the supplied symmetric key. |

---

### `internal/socks5`
Standard RFC 1928 SOCKS5 server for desktop/mobile local client entry:
| Type / Function | Description |
|---|---|
| `NewServer(port, dialer)` | Creates SOCKS5 listener with custom circuit dialer function. |
| `ListenAndServe(ctx)` | Serves connections until context cancellation; handles clean teardown without error leakage. |

---

### `internal/sysproxy`
System-wide proxy automation:
| Function | Platform | Implementation |
|---|---|---|
| `EnableSOCKS5(host, port)` | Windows | Sets `ProxyEnable=1` and `ProxyServer="SOCKS=host:port"` in `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`. |
| `Disable()` | Windows | Sets `ProxyEnable=0` to restore standard direct networking. |
| `EnableSOCKS5(host, port)` | POSIX | Logs instructions for manual network proxy setup. |
| `Disable()` | POSIX | No-op safe stub returning nil. |

---

### `mobile`
Cross-platform bindings compiled via `gomobile`:
| Function / Method | Description |
|---|---|
| `StartEntryNode(socksPort)` | Instantiates mobile discovery service (port 0, opportunistic DHT) and starts SOCKS5 server. |
| `DefaultSOCKSPort()` | Returns standard SOCKS5 port (`1080`) as `int64` for Swift/Kotlin. |
| `MobileNode.Stop()` | Cancels context and closes libp2p host. |

---

## 3. Wire Protocol Specification

### Header & Handshake Phase
```
1. Entry to Relay Initial Stream:
   ASCII text: "EXTEND\n<exit_peer_id>\n"
   Binary:     32 Bytes Entry Ephemeral X25519 Public Key
   Binary:     32 Bytes Relay Ephemeral X25519 Public Key (in response)

2. Entry to Exit Stream (Forwarded by Relay):
   ASCII text: "CONNECT\n<target_host:port>\n" (wrapped in RelayKey encryption)
   Binary:     32 Bytes Entry Ephemeral X25519 Public Key (wrapped in RelayKey encryption)
   Binary:     32 Bytes Exit Ephemeral X25519 Public Key (in response, wrapped in RelayKey encryption)
```

### Data Framing Phase
All post-handshake frames on the wire obey the following binary layout:
```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  Frame Length (uint32 BE)                     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  24-Byte XChaCha20 Nonce                      |
|                           ...                                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  Ciphertext Data (Variable)                   |
|                           ...                                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  16-Byte Poly1305 AEAD Tag                    |
|                           ...                                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

---

## 4. Test Suite Reference

| Test Suite | Package | Covered Scenarios |
|---|---|---|
| `ecdh_test.go` | `internal/crypto` | Ephemeral key generation, shared secret parity, HKDF derivation length, forward secrecy across multiple handshakes. |
| `encryption_test.go` | `internal/crypto` | XChaCha20-Poly1305 round-trips, AEAD tag verification, wrong key rejection, ciphertext tampering detection. |
| `onion_test.go` | `internal/onion` | 1-hop wrap/unwrap, 3-hop layered encapsulation and peeling, empty payload handling, empty keys error guard. |
| `framing_test.go` | `internal/network` | Full stream round-trips, multiple back-to-back frames, large 32KiB payloads, wrong key AEAD rejection. |
| `handler_test.go` | `internal/network` | Byte-by-byte `readLineRaw` validation, stop at newline without consuming key bytes, CRLF trimming, empty lines. |
| `server_test.go` | `internal/socks5` | Nil dialer fallback, custom circuit dialer, context cancellation clean shutdown, port collision handling. |

To execute the test suite:
```bash
go test -race -v ./...
```
