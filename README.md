# 🛡️ ShadowLink — Decentralized Onion-Routed VPN

[![Version](https://img.shields.io/badge/version-2.0.3--alpha-00ffff?style=for-the-badge&logo=shield&logoColor=black)](https://github.com/TUSHAR91316/ShadowLink)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/)
[![Flutter](https://img.shields.io/badge/Flutter-3.x-02569B?style=for-the-badge&logo=flutter&logoColor=white)](https://flutter.dev/)
[![License](https://img.shields.io/badge/license-MIT-00ff88?style=for-the-badge)](LICENSE)
[![DHT Protocol](https://img.shields.io/badge/P2P-libp2p%20Kad--DHT-blueviolet?style=for-the-badge)](https://libp2p.io/)
[![Cryptography](https://img.shields.io/badge/Crypto-XChaCha20--Poly1305%20%7C%20X25519-ff007f?style=for-the-badge)](internal/crypto/)

> **⚠️ LEGAL DISCLAIMER:** ShadowLink is an open-source, peer-to-peer decentralized network routing utility. The developers operate **zero central infrastructure**, maintain **no servers**, and possess **no cryptographic capability** to monitor, inspect, or intercept network traffic. By downloading, compiling, or using this software, you agree to the terms in [TERMS_AND_CONDITIONS.md](TERMS_AND_CONDITIONS.md) and assume 100% of all legal liability.

---

## 🌟 What is ShadowLink?

ShadowLink is a next-generation **Decentralized VPN (dVPN)** engineered in Go and paired with a high-performance Flutter interface. Moving completely away from centralized server models that store logs and expose single points of failure, ShadowLink employs **Multi-Hop Layered Onion Routing** over a serverless **Kademlia Distributed Hash Table (DHT)**.

Every connection is dynamically routed through independent community volunteer nodes, with nested end-to-end cryptographic encapsulation ensuring that **no single node ever learns both who you are and where you are going**.

---

## ✨ Key Architectural Highlights

```
┌──────────────┐     EXTEND      ┌──────────────┐     CONNECT     ┌──────────────┐     TCP      ┌──────────────┐
│  Entry Node  │ ──────────────> │  Relay Node  │ ──────────────> │  Exit Node   │ ───────────> │ Target Host  │
│ (Your Client)│ [Outer: RelayK] │ (Blind Hop)  │ [Inner: ExitK]  │ (Egress Hop) │ (Cleartext)  │ (Internet)   │
└──────────────┘                 └──────────────┘                 └──────────────┘              └──────────────┘
```

### 🔒 True Multi-Hop Onion Routing
- **End-to-End Cryptographic Encapsulation**: Payloads are multi-layer encrypted. Relay nodes only peel their outer layer (`relayKey`) and forward raw inner ciphertext without ever accessing plaintext application data.
- **Perfect Forward Secrecy (PFS)**: Ephemeral X25519 Elliptic-Curve Diffie-Hellman (ECDH) key exchanges generate distinct session keys derived via **HKDF-SHA256** for every circuit.
- **Munitions-Grade Encryption**: Powered by **XChaCha20-Poly1305 AEAD** with 24-byte random nonces, providing quantum-resistant authenticated encryption against tampering, replay, and active MITM attacks.
- **Deep Packet Inspection (DPI) Resistance**: Structured 4-byte big-endian framed packet streams prevent signature matching and protocol fingerprinting.

### 🌐 Serverless Peer Discovery (libp2p Kad-DHT)
- **Zero Central Trackers**: Node discovery occurs dynamically via the Kademlia DHT. Nodes announce themselves under rendezvous strings (`shadowlink-relay`, `shadowlink-exit`) and find peers peer-to-peer.
- **Autonomous Bootstrapping**: Automatically connects to decentralized IPFS/libp2p bootstrap nodes on launch.

### ⚡ High-Throughput & Zero-Allocation Engine
- **Memory Optimization**: Employs reusable frame buffers within `libP2PConn` to eliminate Garbage Collection (GC) thrashing during multi-megabyte transfers.
- **Context-Aware Lifecycle**: All network dials, stream bridges, and discovery calls propagate contexts to guarantee zero Goroutine resource leaks upon disconnection.

### 📱 Unified Cross-Platform Support
- **Desktop**: Native headless daemon and Flutter desktop GUI for Windows, macOS (Intel & Apple Silicon), and Linux.
- **Mobile**: Native static libraries (`mobile.aar` for Android and `Mobile.xcframework` for iOS) generated via `gomobile` with battery-optimized opportunistic discovery.

---

## 📁 Repository Structure

```
ShadowLink/
├── cmd/
│   └── shadowlink/            # CLI daemon entrypoint & signal handling
├── internal/
│   ├── config/                # Centralized protocol constants & port defaults
│   ├── crypto/                # X25519 ECDH, HKDF-SHA256 & XChaCha20-Poly1305
│   ├── discovery/             # libp2p host lifecycle & Kademlia DHT routing
│   ├── network/               # Onion circuit builder, framing, stream handlers & bridge
│   ├── onion/                 # Multi-layered payload wrap/unwrap primitives
│   ├── socks5/                # Local SOCKS5 proxy server with custom onion dialer
│   └── sysproxy/              # OS-level proxy automation (Windows Registry & POSIX stubs)
├── mobile/                    # gomobile export bindings (MobileNode for Swift/Kotlin)
├── shadowlink_gui/            # Flutter desktop & mobile UI (Cyber Aesthetic)
├── release/                   # Pre-compiled multi-platform release binaries
├── docs/                      # Technical deep-dive documentation & release history
└── .github/workflows/         # CI/CD cross-compilation matrix (Go, Flutter, gomobile)
```

---

## 🚀 Quick Start & Installation

### Prerequisites
- **Go**: Version 1.22 or higher ([Download Go](https://go.dev/dl/))
- **Flutter**: Version 3.10+ (for building the GUI)
- **Git**: For cloning the repository

### 1. Clone & Build the Go Daemon

```bash
git clone https://github.com/TUSHAR91316/ShadowLink.git
cd ShadowLink

# Download & verify Go module dependencies
go mod tidy

# Build native binary
go build -o shadowlink ./cmd/shadowlink
```

### 2. Run the CLI Daemon

ShadowLink functions as a modular node. You can run client and routing roles concurrently:

#### 🔹 Client Mode (Entry Node + System Proxy)
Starts a local SOCKS5 proxy on `127.0.0.1:1080` and configures your OS to route traffic through the dVPN:
```bash
# Windows / Linux / macOS
./shadowlink --entry --socks 1080 --sysproxy
```

#### 🔹 Relay Node (Middleman Hop)
Help power the decentralized network by forwarding encrypted traffic without seeing plaintext:
```bash
./shadowlink --relay --port 9001
```

#### 🔹 Exit Node (Internet Egress)
Provide public internet egress for dVPN circuits:
```bash
./shadowlink --exit --port 9002
```

#### 🔹 Combined Node
Run client and relay operations simultaneously:
```bash
./shadowlink --entry --relay --port 9000 --socks 1080
```

#### 🔹 Reset System Proxy
If needed, restore OS network settings immediately:
```bash
./shadowlink --reset-proxy
```

---

## 🖥️ Running the Flutter GUI

ShadowLink includes a desktop and mobile GUI styled in a custom **Deep Obsidian & Neon Cyan** palette.

```bash
cd shadowlink_gui

# Fetch dependencies
flutter pub get

# Run on your desktop platform
flutter run -d windows    # On Windows
flutter run -d macos      # On macOS
flutter run -d linux      # On Linux
```

---

## 🛠️ CLI Flag Reference

| Flag | Type | Default | Description |
|---|---|---|---|
| `--entry` | `bool` | `false` | Run as an Entry client (starts local SOCKS5 proxy) |
| `--relay` | `bool` | `false` | Run as a Relay node (transits encrypted traffic) |
| `--exit` | `bool` | `false` | Run as an Exit node (egresses traffic to the internet) |
| `--port` | `int` | `9000` | Port for incoming P2P libp2p connections (`0` = OS-assigned) |
| `--socks` | `int` | `1080` | Port for local SOCKS5 proxy listener |
| `--sysproxy` | `bool` | `false` | Automatically configure OS system proxy (Windows) |
| `--reset-proxy` | `bool` | `false` | Reset OS system proxy settings and exit immediately |

---

## 🧪 Testing & Verification

ShadowLink includes comprehensive unit and integration tests covering the cryptographic pipeline, framing integrity, layered onion wrapping, and DHT discovery:

```bash
# Run all Go unit tests with race detection
go test -race -v ./...

# Run static analysis
go vet ./...

# Run Flutter GUI analysis
cd shadowlink_gui && flutter analyze && flutter test
```

---

## 📚 Technical Documentation

- 📖 [**Architecture Deep-Dive**](docs/ARCHITECTURE.md): Wire protocols, stream lifecycle, and packet framing specifications.
- 🏛️ [**System Architecture Overview**](ARCHITECTURE.md): Architectural component diagrams, sequence diagrams, and threat models.
- 📦 [**Release Notes**](docs/RELEASE_NOTES.md): Detailed changelog across releases.
- ⚖️ [**Terms & Conditions**](TERMS_AND_CONDITIONS.md): Complete legal agreement and disclaimers.

---

## 🤝 Contributing

We welcome community contributions! Please review [CONTRIBUTING.md](CONTRIBUTING.md) and our [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before submitting Pull Requests.

---

## 📄 License

ShadowLink is open-source software licensed under the [MIT License](LICENSE).
