# ShadowLink - Decentralized Onion-Routed VPN

[![Version](https://img.shields.io/badge/version-2.0.0--alpha-blue.svg)](https://github.com/TUSHAR91316/ShadowLink)
[![Go](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

ShadowLink is a next-generation **Decentralized VPN (dVPN)** built in Go. Moving away from traditional client-server architectures, ShadowLink utilizes **Multi-hop Onion Routing** and a **Distributed Hash Table (DHT)** to provide a completely serverless, highly secure, and untraceable network tunnel. 

## 🚀 Key Features

### 🔒 Security & Privacy (Onion Routing)
- **Multi-Hop Architecture**: Traffic bounces through Entry, Relay, and Exit nodes. No single node knows both your IP and your destination, protecting you from tracking.
- **XChaCha20-Poly1305 AEAD**: State-of-the-art authenticated encryption with random nonces ensures robust security against tampering and replay attacks.
- **DPI Evasion**: Multi-layered encryption prevents Deep Packet Inspection from identifying traffic type.

### 🌐 Decentralized Networking (DHT)
- **Serverless Node Discovery**: Uses `libp2p` Kademlia DHT to find available nodes on the network dynamically. No central tracking servers to shut down.
- **Community Driven**: Purely free network. Anyone can run an Entry, Relay, or Exit node to contribute bandwidth.

### 💻 Cross-Platform & Mobile Ready
- **Written in Go**: Extremely fast, memory-safe, and highly concurrent networking.
- **Universal Support**: Compiles natively to Windows, macOS, Linux, and can be ported to iOS/Android via `gomobile`.
- **System-Wide Proxy**: Built-in OS proxy configuration (SOCKS5).

## 📦 Installation & Setup

### Prerequisites
- **Go 1.21+** (for building from source)
- **Git**

### 1. Install & Build
```bash
git clone https://github.com/TUSHAR91316/ShadowLink.git
cd ShadowLink

# Download dependencies
go mod tidy

# Build the binary
go build -o shadowlink
```

### 2. Run ShadowLink

ShadowLink operates as a unified node. You can run it in different roles using command-line flags.

**Start an Entry Node (Client) with System Proxy:**
```bash
./shadowlink --role entry --socks 1080 --sysproxy
```
*This starts a local SOCKS5 server on port 1080 and configures your OS to route traffic through it.*

**Start a Relay Node:**
```bash
./shadowlink --role relay --port 9001
```

**Start an Exit Node:**
```bash
./shadowlink --role exit --port 9002
```

## 🏗️ Architecture Migration (v1.x -> v2.x)

ShadowLink has been completely rewritten from Python to **Go**. 
- **Legacy Code**: The old Python Client-Server architecture (v1) has been archived in the `legacy_python/` directory.
- **New Core**: The new architecture embraces P2P networking via `libp2p` and replaces basic XOR obfuscation with strict `x/crypto` standards.

## 🤝 Contributing
As a decentralized network, the strength of ShadowLink relies on the community.
1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Commit changes and submit a Pull Request

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
