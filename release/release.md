# 📦 ShadowLink v2.0.3-alpha Release

Welcome to **ShadowLink v2.0.3-alpha**! This release marks the completion of the transition from legacy Python to a high-performance Go core featuring true multi-hop Onion Routing, zero-allocation memory framing, and universal cross-platform Flutter support.

> **⚖️ LEGAL DISCLAIMER:** By downloading, distributing, or running any of these binaries, you legally bind yourself to the [TERMS_AND_CONDITIONS.md](../TERMS_AND_CONDITIONS.md). The developers assume zero liability for your actions. Operating an Exit Node exposes your IP address to third-party egress traffic; you do so entirely at your own personal and legal risk.

---

## 🚀 Key Release Highlights

- **🔒 True 3-Hop Onion Routing**: Relays act as transparent intermediaries via the `EXTEND` protocol without access to plaintext application data or destination endpoints.
- **⚡ Zero-Allocation Framing Engine**: Reusable buffer architecture within `libP2PConn` eliminates garbage collection pauses during high-bandwidth downloads.
- **🔑 Forward-Secret Cryptography**: Ephemeral **X25519 ECDH** + **HKDF-SHA256** session key derivation paired with **XChaCha20-Poly1305 AEAD**.
- **🌐 Serverless Kad-DHT Discovery**: Automatic decentralized node discovery over `libp2p`.
- **📱 Universal Desktop & Mobile Engine**: Native binaries for Windows, macOS, and Linux, plus `gomobile` packages for Android and iOS.

---

## 📥 Available Artifacts

### 🖥️ Desktop CLI Binaries
- **Windows (x64)**: `shadowlink-windows-x64.exe`
- **Linux (x64)**: `shadowlink-linux-x64`
- **macOS (Apple Silicon M1/M2/M3/M4)**: `shadowlink-macos-apple-silicon`
- **macOS (Intel x64)**: `shadowlink-macos-intel`

### 📱 Mobile Frameworks (via CI/CD)
- **Android**: `shadowlink_gui/android/app/libs/mobile.aar` (Compiled targeting Android API 21+)
- **iOS**: `shadowlink_gui/ios/Mobile.xcframework` (Multi-architecture XCFramework for iOS Device & Simulator)

---

## 🛠️ Usage Instructions

### 1. Windows (Client / Entry Node with System Proxy)
Open PowerShell and run:
```powershell
.\shadowlink-windows-x64.exe --entry --socks 1080 --sysproxy
```
*This starts the local SOCKS5 proxy on port 1080 and automatically configures Windows Internet Settings to route traffic through the dVPN.*

To reset proxy settings at any time:
```powershell
.\shadowlink-windows-x64.exe --reset-proxy
```

### 2. Linux & macOS (Client / Entry Node)
Open Terminal and run:
```bash
# Make binary executable
chmod +x shadowlink-linux-x64

# Start Entry Node
./shadowlink-linux-x64 --entry --socks 1080
```

### 3. Community Nodes (Relay & Exit)
Help power the decentralized network by volunteering bandwidth:

#### Run a Relay Node (Blind Traffic Hop):
```bash
./shadowlink-linux-x64 --relay --port 9001
```

#### Run an Exit Node (Internet Egress Gateway):
```bash
./shadowlink-linux-x64 --exit --port 9002
```

#### Run Combined Roles:
```bash
./shadowlink-linux-x64 --entry --relay --port 9000 --socks 1080
```

---

## 🔍 Binary Integrity Verification

Verify your downloaded binary against the published SHA-256 checksums:

```powershell
# On Windows PowerShell
Get-FileHash shadowlink-windows-x64.exe -Algorithm SHA256

# On Linux / macOS
sha256sum shadowlink-linux-x64
```
