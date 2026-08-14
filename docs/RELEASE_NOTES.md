# 📜 ShadowLink Release Notes & Changelog

---

## 🚀 Release v2.0.3-alpha — True Onion Routing & Zero-Allocation Engine

**Release Date:** August 2026  
**Status:** Alpha Release (Recommended)

### 🔒 Security & Onion Routing Overhaul
- **True Multi-Hop Encapsulation**: Migrated from hop-by-hop re-encryption to full 3-hop layered onion routing using `EXTEND` and `CONNECT` control frames. Relay nodes now operate as blind forwarders and cannot inspect payload traffic or target destinations.
- **Nested `libP2PConn` Chaining**: Dynamic crypto wrapping applies inner `exitKey` and outer `relayKey` layers with 4-byte big-endian framing.
- **Strict Protocol Validation**: Empty-key error guards added to `onion.WrapPayload` and `onion.UnwrapPayload` to prevent unencrypted transmissions.
- **Slow-Loris Guard**: Added strict line-length bounds in `readLineRaw` to protect nodes from memory-exhaustion attacks.

### ⚡ Performance & Memory
- **Zero-Allocation Read Path**: Pre-allocated, reusable buffer (`frameBuf`) in `libP2PConn` eliminates Garbage Collection pauses during large file transfers.
- **Atomic Wire Writes**: Frame length header and ciphertext are merged into a single atomic write buffer to prevent partial-packet wire races.
- **Goroutine Leak Prevention**: Fixed `bridge()` to forcibly close companion streams upon peer disconnection, ensuring zero lingering routines.

### 📱 Mobile & CI/CD
- **`gomobile` Tooling Directive**: Integrated Go 1.24+ `tool` dependencies for clean `gomobile bind` workflows.
- **NDK API 21+ Compatibility**: Fixed Android build pipeline by explicitly targeting modern Android NDK toolchains.
- **iOS Framework Search Paths**: Standardized Xcode project linkage for `Mobile.xcframework`.

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
