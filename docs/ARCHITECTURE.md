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
         │ 1. Checks peerCache (<1ms); on miss, queries Kad-DHT ("shadowlink-relay", "shadowlink-exit")
         │ 2. Randomizes peer lists via cryptoShuffle() (crypto/rand.Int Fisher-Yates)
         │ 3. Selects (relayPeer, exitPeer) pair; sets 15s handshake deadline context
         │ 4. Calls tryViaRelay(ctx, ds, relayPeer, exitPeer, "example.com:443")
         │ 5. On dial/handshake failure, calls ds.InvalidatePeer(peerID) for immediate eviction
         ▼
[ Handshake Phase: Entry ↔ Relay Hop (15s Deadline) ]
         │ Entry dials relay via libp2p Host.NewStream(relayPeer, "/shadowlink/1.0.0")
         │ Entry sends: "EXTEND\n<exitPeer.ID>\n"
         │ Entry & Relay perform X25519 ECDH → derives RelayKey via HKDF-SHA256
         │ Entry wraps stream into relayConn: libP2PConn{Conn: stream, AEADs: [relayAEAD]}
         ▼
[ Handshake Phase: Entry ↔ Exit Hop (Tunneled Through Relay, 15s Deadline) ]
         │ Relay resolves exitPeer via Kad-DHT/cache and dials NewStream(exitPeer, "/shadowlink/1.0.0")
         │ Relay creates upstreamConn: libP2PConn{Conn: s, AEADs: [relayAEAD]}
         │ Relay calls bridge(upstreamConn, streamAdapter{exitStream}) using copyBufferPool (32 KiB sync.Pool)
         │ Entry writes through relayConn: "CONNECT\nexample.com:443\n"
         │ Relay decrypts outer RelayKey layer and passes raw "CONNECT\nexample.com:443\n" to Exit
         │ Entry & Exit perform X25519 ECDH through relay tunnel → derives ExitKey via HKDF-SHA256
         │ Entry wraps relayConn into exitConn: libP2PConn{Conn: relayConn, AEADs: [exitAEAD]}
         │ Deadlines cleared on both streams (SetDeadline(time.Time{})) for streaming
         ▼
[ Data Transfer Phase: Onion Encapsulation ]
         │ Application writes N plaintext bytes to exitConn:
         │ 1. exitConn encrypts with ExitKey → ciphertext1
         │ 2. exitConn writes to relayConn with 4-byte BE length header
         │ 3. relayConn encrypts with RelayKey → ciphertext2
         │ 4. relayConn writes [4-byte BE length][ciphertext2] to wire in single atomic write
         ▼
[ Relay Transit Hop ]
         │ 1. Relay reads 4-byte length header from stream
         │ 2. Relay reads ciphertext2 from stream into reusable buffer
         │ 3. Relay decrypts ciphertext2 in-place (DecryptWithAEADInPlace) → yields [4-byte header][ciphertext1]
         │ 4. Relay forwards [4-byte header][ciphertext1] to exitStream (Relay NEVER sees plaintext)
         ▼
[ Exit Egress Hop ]
         │ 1. Exit reads 4-byte length header from exitStream
         │ 2. Exit reads ciphertext1 into reusable buffer
         │ 3. Exit decrypts ciphertext1 in-place (DecryptWithAEADInPlace) → yields original N plaintext bytes
         │ 4. Exit dials net.Dialer.DialContext(ctx, "tcp", "example.com:443") with 15s timeout
         │ 5. Exit proxies bidirectionally between encrypted dVPN stream and cleartext TCP socket via copyBufferPool
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
| `NewAEAD(key)` | `([]byte) (cipher.AEAD, error)` | Instantiates an XChaCha20-Poly1305 AEAD cipher instance from a 32-byte key. |
| `EncryptWithAEAD(aead, plaintext)` | `(cipher.AEAD, []byte) ([]byte, error)` | Encrypts using pre-allocated contiguous buffer: `[24-byte nonce][ciphertext + 16-byte tag]`. |
| `DecryptWithAEADInPlace(aead, ciphertext)` | `(cipher.AEAD, []byte) ([]byte, error)` | Decrypts directly within the slice without memory allocations, achieving >1.15 GB/s per core. |
| `Encrypt(key, plaintext)` | `([]byte, []byte) ([]byte, error)` | Convenience wrapper: instantiates AEAD and encrypts via `EncryptWithAEAD`. |
| `Decrypt(key, ciphertext)` | `([]byte, []byte) ([]byte, error)` | Convenience wrapper: instantiates AEAD and decrypts via `DecryptWithAEAD`. |
| `GenerateKey()` | `() ([]byte, error)` | Generates 32 cryptographically secure random bytes. |

---

### `internal/discovery`
Manages the libp2p host lifecycle, Kademlia DHT routing, and in-memory peer caching:
| Function / Method | Signature | Description |
|---|---|---|
| `NewDiscoveryService(ctx, port, rendezvous)` | `(context.Context, int, string) (*DiscoveryService, error)` | Starts libp2p host, connects to bootstrap peers (10s timeout), enables `dht.ModeAuto`, and initializes peer cache. |
| `FindPeers(ctx, rendezvous)` | `(context.Context, string) ([]peer.AddrInfo, error)` | Queries thread-safe `peerCache` (<1ms hit). On miss or expiration (45s TTL), queries Kad-DHT routing table and updates cache. |
| `InvalidatePeer(id)` | `(peer.ID)` | Evicts an unreachable or dead peer from `peerCache` immediately upon circuit dial failure. |
| `Close()` | `() error` | Shuts down Kad-DHT routing table and terminates libp2p host cleanly. |

---

### `internal/network`
Implements core onion circuit negotiation, framing, stream dispatching, and buffer pooling:
| Type / Function | Description |
|---|---|
| `DialCircuit(ctx, ds, network, addr)` | Discovers DHT peers, randomizes candidates via `cryptoShuffle`, applies 15s handshake deadline, builds onion circuit, and invalidates dead peers via `InvalidatePeer`. |
| `HandleStream(ctx, s, role, ds)` | Stream dispatcher: enforces 15s deadline while reading sentinel command (`EXTEND` or `CONNECT`) and completing ECDH, clears deadline, and routes to `handleRelay` or `handleExit`. |
| `libP2PConn` | `net.Conn` implementation featuring 4-byte BE length framing, zero-allocation in-place AEAD decryption (`DecryptWithAEADInPlace`), and atomic wire writes. |
| `copyBufferPool` | `sync.Pool` yielding reusable 32 KiB byte slices for `io.CopyBuffer` in `bridge()`, eliminating GC allocations during proxying. |
| `streamAdapter` | Adapts libp2p `network.Stream` to `net.Conn` interface by providing stub address methods. |

---

### `internal/onion`
Encapsulates recursive layered onion wrapping and peeling:
| Function | Signature | Description |
|---|---|---|
| `WrapPayloadWithCiphers(data, ciphers)` | `([]byte, []cipher.AEAD) ([]byte, error)` | Concentric onion encapsulation: allocates isolated buffer per layer (`ciphers[len-1]` innermost → `ciphers[0]` outermost) preventing slice aliasing. |
| `WrapPayload(data, keys)` | `([]byte, [][]byte) ([]byte, error)` | Instantiates AEAD ciphers for keys and executes `WrapPayloadWithCiphers`. Validates non-empty keys. |
| `UnwrapPayloadInPlace(data, key)` | `([]byte, []byte) ([]byte, error)` | In-place zero-allocation peeling of exactly one onion encryption layer using symmetric key. |
| `UnwrapPayloadWithAEADInPlace(data, aead)` | `([]byte, cipher.AEAD) ([]byte, error)` | Peels one encryption layer in-place using pre-existing `cipher.AEAD`. |
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
During the handshake phase, both incoming streams and outgoing circuit handshakes enforce a strict **15-second deadline** (`handshakeTimeout`). If a remote peer stalls during protocol command transmission or public key exchange, the connection is aborted immediately:

```
1. Entry to Relay Initial Stream (15s Deadline):
   ASCII text: "EXTEND\n<exit_peer_id>\n"
   Binary:     32 Bytes Entry Ephemeral X25519 Public Key
   Binary:     32 Bytes Relay Ephemeral X25519 Public Key (in response)

2. Entry to Exit Stream Forwarded by Relay (15s Deadline):
   ASCII text: "CONNECT\n<target_host:port>\n" (wrapped in RelayKey encryption)
   Binary:     32 Bytes Entry Ephemeral X25519 Public Key (wrapped in RelayKey encryption)
   Binary:     32 Bytes Exit Ephemeral X25519 Public Key (in response, wrapped in RelayKey encryption)

3. Post-Handshake Transition:
   Deadlines cleared on both streams (SetDeadline(time.Time{})) for unrestricted continuous streaming.
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

## 4. Test Suite & Benchmarks Reference

### Test Suites
| Test Suite | Package | Covered Scenarios |
|---|---|---|
| `ecdh_test.go` | `internal/crypto` | Ephemeral key generation, shared secret parity, HKDF derivation length, forward secrecy across multiple handshakes. |
| `encryption_test.go` | `internal/crypto` | XChaCha20-Poly1305 round-trips, in-place AEAD decryption, wrong key rejection, ciphertext tampering detection. |
| `onion_test.go` | `internal/onion` | 1-hop wrap/unwrap, 3-hop layered encapsulation, in-place unwrap, empty payload handling, empty keys error guard. |
| `framing_test.go` | `internal/network` | Stream framing round-trips, atomic writes, multiple back-to-back frames, large 32KiB payloads, wrong key AEAD rejection. |
| `handler_test.go` | `internal/network` | Byte-by-byte `readLineRaw` bounds, stop at newline without consuming key bytes, CRLF trimming, empty lines. |
| `server_test.go` | `internal/socks5` | Nil dialer fallback, custom circuit dialer, context cancellation clean shutdown, port collision handling. |

To execute the test suite with race detection:
```bash
go test -race -v ./...
```

### Performance Benchmarks
Execute the benchmark suite:
```bash
go test -bench="." -benchmem ./internal/crypto ./internal/network ./internal/onion
```

**Recorded Performance (Intel Core i5-11400H @ 2.70GHz, single core):**
| Benchmark | Speed / Throughput | Latency | Allocations |
|---|---|---|---|
| `BenchmarkDecrypt_1KB` | **1,173.23 MB/s** (~1.17 GB/s) | 872.8 ns/op | 2 allocs/op (1,184 B) |
| `BenchmarkEncrypt_1KB` | **1,031.34 MB/s** (~1.03 GB/s) | 992.9 ns/op | 2 allocs/op (1,184 B) |
| `BenchmarkFraming_Throughput_4KB` | **505.12 MB/s** | 8,109 ns/op | **1 alloc/op** (48 B) |
| `PeerCache Lookup` | **< 1 ms** | ~0.2 µs | 0 allocs/op |
