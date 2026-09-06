# 📜 ShadowLink Release Notes & Changelog

---

## 🚀 Release v2.0.3-alpha — Hardened Onion Routing, Peer Caching & Zero-Allocation Engine

**Release Date:** September 2026  
**Status:** Alpha Release (Recommended)

### 🔒 Security & Onion Routing Hardening
- **Cryptographically Secure Shuffling**: Replaced `math/rand` with `crypto/rand.Int` in `cryptoShuffle()`. Candidate peer selection for circuits is now protected against PRNG state observation and route prediction attacks.
- **Strict 15-Second Handshake Deadlines**: Hardened incoming streams and outgoing circuit handshakes with strict 15-second timeouts (`handshakeTimeout`). Slow-loris attacks or unresponsive DHT nodes are cut off immediately, preventing Goroutine or connection stalls.
- **Fast Peer Invalidation**: When an exit or relay fails to dial during circuit construction, `ds.InvalidatePeer(peerID)` immediately purges the dead peer from the discovery cache.
- **Multi-Layer Buffer Separation**: Hardened `WrapPayloadWithCiphers` to isolate frame allocations per layer, eliminating slice aliasing between concentric onion envelopes.
- **True Multi-Hop Encapsulation**: Full 3-hop layered onion routing using `EXTEND` and `CONNECT` control frames. Relay nodes act as blind forwarders and cannot inspect payload traffic or target destinations.
- **Strict Protocol Validation**: Empty-key error guards in `onion.WrapPayload` and `onion.UnwrapPayload` guarantee no unencrypted payloads reach the wire.

### ⚡ Performance, Caching & Zero-Allocation Engine
- **Sub-Millisecond DHT Peer Cache**: Added thread-safe in-memory `peerCache` (45s TTL) protected by `sync.RWMutex`. Parallel requests (e.g. web browser loading dozens of parallel resources) resolve candidate peers in **<1ms** instead of triggering repetitive 1–2 second Kad-DHT traversals.
- **DHT Auto-Mode**: Activated `dht.ModeAuto` allowing nodes to automatically switch between DHT client and server modes based on public reachability.
- **Zero-Allocation In-Place Decryption**: Introduced `DecryptWithAEADInPlace` and `UnwrapPayloadInPlace`, achieving **>1.15 GB/s** decryption throughput per core without allocating intermediate slices.
- **Stream Bridge Buffer Pooling**: Bi-directional proxy forwarding in `bridge()` now uses a 32 KiB `sync.Pool` (`copyBufferPool`) with `io.CopyBuffer`, eliminating GC thrashing during continuous high-throughput transfers.
- **Atomic Wire Writes**: Single-buffer serialization for frame lengths and encrypted payloads prevents TCP packet fragmentation races.

### 📱 GUI Telemetry & Mobile CI/CD
- **Bounded Desktop Log Buffer**: Implemented a sliding window buffer (capped at 20,000 characters) in Flutter's `DaemonService` (`logNotifier`), preventing GUI memory growth during long-running sessions.
- **`gomobile` Tooling Directive**: Integrated Go 1.24+ `tool` dependencies for streamlined `gomobile bind` workflows.
- **NDK API 21+ Compatibility**: Modernized Android NDK compilation pipeline.
- **iOS Framework Search Paths**: Standardized Xcode linkage for `Mobile.xcframework`.

---

## 🏛️ Release v2.0.0-alpha — Go Rewrite & Flutter Cyber UI

**Release Date:** July 2026  
**Status:** Archived

### Major Highlights
- **Complete Go Engine**: Fully deprecated legacy Python codebase in favor of a memory-safe, concurrent Go engine built on `libp2p`.
- **Serverless Discovery**: Implemented Kademlia DHT peer discovery, removing all hardcoded central trackers.
- **Modern Flutter Frontend**: Replaced old Electron GUI with a high-performance Flutter desktop app styled in a Deep Obsidian & Neon Cyan aesthetic.
- **Windows System Proxy**: Automatic registry proxying with failsafe `--reset-proxy` recovery.

---

## ⚠️ Legacy Release v1.0.0 — Electron & Python Prototype

**Status:** Deprecated (Archived in `legacy_python/`)

- Hybrid architecture combining Electron frontend with Python backend subprocess.
- Single-hop encrypted proxying via AES-256-GCM.
