# ShadowLink v2.0.0-alpha Release

Welcome to the new era of ShadowLink! This release completely transitions the network from the legacy Python architecture to an insanely fast, memory-safe Go core featuring native Decentralized VPN capabilities.

> **LEGAL DISCLAIMER:** By downloading or executing any of these binaries, you legally bind yourself to the [TERMS_AND_CONDITIONS.md](../TERMS_AND_CONDITIONS.md). The developers assume absolutely zero liability for your actions. Running an Exit Node exposes your IP address to third-party traffic; you do so entirely at your own personal and legal risk.

## Cross-Platform Binaries
Included in this folder are statically linked, zero-dependency binaries for all major operating systems:

- `shadowlink-windows-x64.exe`: For Windows 10/11 (64-bit)
- `shadowlink-macos-intel`: For macOS (Intel processors)
- `shadowlink-macos-apple-silicon`: For macOS (Apple Silicon M1/M2/M3)
- `shadowlink-linux-x64`: For standard Linux distros (Ubuntu, Debian, Fedora, Arch)

*(Note: iOS and Android framework compilation requires `gomobile` and will be handled in a mobile-specific pipeline later).*

## Getting Started

### Windows
1. Open PowerShell or Command Prompt.
2. Run as a client (entry node):
   ```powershell
   .\shadowlink-windows-x64.exe --role entry --socks 1080 --sysproxy
   ```
   *This automatically configures your Windows proxy settings to use the dVPN!*

### macOS / Linux
1. Open Terminal.
2. Make the binary executable (e.g., for Linux): 
   ```bash
   chmod +x shadowlink-linux-x64
   ```
3. Run as a client:
   ```bash
   ./shadowlink-linux-x64 --role entry --socks 1080
   ```

### Community Participation
ShadowLink is a purely free, decentralized network. You can help power the network by running your machine as a relay or exit node.

**Start a Relay Node:**
```bash
./shadowlink-linux-x64 --role relay --port 9001
```

**Start an Exit Node:**
```bash
./shadowlink-linux-x64 --role exit --port 9002
```
