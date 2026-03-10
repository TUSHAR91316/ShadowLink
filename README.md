# ShadowLink - Secure Local Encrypted Tunnel

ShadowLink is a next-generation local VPN and secure tunnel application that uses **ChaCha20-Poly1305** and **X25519** key exchange (the cryptographic primitives of WireGuard) to encrypt traffic. It comes with both a **Command Line Interface (CLI)** and a **Native GUI** (via PyQt6) for optimal performance and integration.

## 🚀 Features

- **Double Encryption**: Traffic is encrypted locally using ephemeral keys before leaving your device.
- **DPI Bypass (Obfuscation)**: Evades Deep Packet Inspection (DPI) on strict university/corporate networks via Fake HTTP handshakes and payload masking.
- **Strict Mode (Kill Switch)**: Automatically cuts traffic if the secure tunnel drops.
- **System-Wide Proxy**: Routes all system traffic through the secure tunnel with one click.
- **Modern UI**: Clean native Windows GUI with real-time stats (PyQt6).
- **Cross-Platform**: Built natively with Python.

## 🛠 Architecture

ShadowLink now uses a fully native Python architecture:
- **Core Backend**: `src/api.py`, handling specific encryption, connection tunnels, and socket operations.
- **CLI Frontend**: `src/cli.py`, a lightweight command line application.
- **GUI Frontend**: `src/gui.py`, a native Windows desktop GUI using `PyQt6`.

## ❓ What Makes It Different?

| Feature | Standard SOCKS5 Proxy | ShadowLink |
| :--- | :--- | :--- |
| **Encryption** | None (Usually plaintext) | **ChaCha20-Poly1305 (WireGuard)** |
| **Key Management** | Static Password / None | **Ephemeral X25519** (New key per session) |
| **Purpose** | IP Masking | **DPI Obfuscation** & Layered Security |
| **Dependency** | Remote Server | **Local-Only** (Server runs on your localhost) |

**Why use this locally?** 
It isolates your application traffic from the rest of the OS until it is fully encrypted. Even if malware on your PC packet-sniffs your network card, they only see the encrypted ShadowLink traffic, not the raw application data.

## ⚠️ Limitations

1.  **TCP Only**: Currently supports SOCKS5 CONNECT method (TCP). UDP (e.g., for gaming/VoIP) is not yet supported.
2.  **Performance Overhead**: Double encryption (ShadowLink + ProtonVPN) adds a small amount of latency and CPU overhead.
3.  **Manual Proxy Config**: You must configure your browser/app to use the provided SOCKS5 proxy if not using System-Wide mode.

## 📦 Installation & Usage

### Prerequisites
- Python 3.10+

### 1. Setup
```bash
# Install Python dependencies
pip install -r requirements.txt
```

### 2. Run CLI Version
For a lightweight, terminal-based experience:
```bash
python src/cli.py
```

### 3. Run GUI Version
For a fully native Windows desktop graphical interface:
```bash
python src/gui.py
```

## 🔐 Verification
To verify the encryption implementation (X25519 + ChaCha20-Poly1305), run the included verification script:

```bash
python src/verify_encryption.py
```
This script simulates a full handshake and encryption cycle, printing the keys and ciphertext to the console for inspection.

## 📄 License
MIT
