# ShadowLink - Decentralized Onion-Routed VPN

[![Version](https://img.shields.io/badge/version-2.0.0--alpha-blue.svg)](https://github.com/TUSHAR91316/ShadowLink)
[![Go](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

> **LEGAL DISCLAIMER:** ShadowLink is a decentralized, peer-to-peer open-source protocol. The developers operate **no infrastructure**, have **no control** over the network, and assume **zero liability** for any damages or legal repercussions caused by end-users. By downloading or using this software, you agree to the strict terms outlined in the [TERMS_AND_CONDITIONS.md](TERMS_AND_CONDITIONS.md). You assume 100% of the legal risk.

ShadowLink is a next-generation **Decentralized VPN (dVPN)** built in Go. Moving away from traditional client-server architectures, ShadowLink utilizes **Multi-hop Onion Routing** and a **Distributed Hash Table (DHT)** to provide a completely serverless, highly secure, and untraceable network tunnel. 

## 🚀 Key Features

### 🔒 Security & Privacy (Onion Routing)
- **Multi-Hop Architecture**: Traffic bounces through Entry, Relay, and Exit nodes (3-hop routing). No single node knows both your IP and your destination, protecting you from tracking.
- **Perfect Forward Secrecy**: Ephemeral X25519 ECDH key exchange generates a unique session key for every single connection.
- **XChaCha20-Poly1305 AEAD**: State-of-the-art authenticated encryption with random nonces ensures robust security against tampering and replay attacks.
- **DPI Evasion**: Multi-layered encryption with 4-byte length-prefix framing prevents Deep Packet Inspection from identifying traffic type.

### 🌐 Decentralized Networking (DHT)
- **Serverless Node Discovery**: Uses `libp2p` Kademlia DHT to find available nodes on the network dynamically. No central tracking servers to shut down.
- **Community Driven**: Purely free network. Anyone can run an Entry, Relay, or Exit node to contribute bandwidth.

### 💻 Cross-Platform & Mobile Ready
- **Go Backend**: Extremely fast, memory-safe, and highly concurrent networking.
- **Flutter Desktop GUI**: A beautiful, modern "cyber" aesthetic interface to manage your daemon connection and node roles.
- **Universal Support**: Compiles natively to Windows, macOS, Linux, and can be ported to iOS/Android via `gomobile`.
- **System-Wide Proxy**: Built-in OS proxy configuration (SOCKS5).

## 📦 Installation & Setup

### Prerequisites
- **Go 1.21+** (for building from source)
- **Git**

### 1. Install & Build
### 1. Build the Go Daemon
```bash
git clone https://github.com/TUSHAR91316/ShadowLink.git
cd ShadowLink

# Download dependencies
go mod tidy

# Build the binary
go build -o release/shadowlink-windows-x64.exe ./cmd/shadowlink
```

### 2. Run the Flutter GUI
```bash
cd shadowlink_gui
flutter pub get
flutter run -d windows
```

### 2. Run ShadowLink

ShadowLink operates as a unified node. You can run it in different roles concurrently using command-line flags.

**Start an Entry Node (Client) with System Proxy:**
```bash
./shadowlink --entry --socks 1080 --sysproxy
```
*This starts a local SOCKS5 server on port 1080 and configures your OS to route traffic through it.*

**Start a Relay Node:**
```bash
./shadowlink --relay --port 9001
```

**Start an Exit Node:**
```bash
./shadowlink --exit --port 9002
```

**Run Concurrent Roles:**
```bash
./shadowlink --entry --relay --port 9000
```

## 🏗️ Architecture Migration (v1.x -> v2.x)

For a complete breakdown of the new Flutter + Go architecture and how the system components interact, please refer to the [ARCHITECTURE.md](ARCHITECTURE.md) diagram and the [developer deep-dive](docs/ARCHITECTURE.md).

ShadowLink has been completely rewritten from Python to **Go** for the backend daemon, and from Electron/React to **Flutter** for the frontend GUI. 
- **Legacy Code**: The old Python Client-Server architecture (v1) has been archived in the `legacy_python/` directory. Legacy docs remain in `docs/` for historical context.
- **New Core**: The new architecture embraces P2P networking via `libp2p`, perfect forward secrecy via `crypto/ecdh`, and strict 3-hop circuit routing.

## 🤝 Contributing
As a decentralized network, the strength of ShadowLink relies on the community.
1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Commit changes and submit a Pull Request

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
