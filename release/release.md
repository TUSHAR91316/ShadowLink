# ShadowLink v2.0.0-alpha Release

Welcome to the new era of ShadowLink! This release completely transitions the network from the legacy Python architecture to an insanely fast, memory-safe Go core featuring native Decentralized VPN capabilities.

## Cross-Platform Binaries
Included in this folder are statically linked, zero-dependency binaries for all major operating systems:

- `shadowlink-windows-amd64.exe`: For Windows 10/11 (64-bit)
- `shadowlink-darwin-amd64`: For macOS (Intel processors)
- `shadowlink-darwin-arm64`: For macOS (Apple Silicon M1/M2/M3)
- `shadowlink-linux-amd64`: For standard Linux distros (Ubuntu, Debian, Fedora, Arch)

*(Note: iOS and Android framework compilation requires `gomobile` and will be handled in a mobile-specific pipeline later).*

## Getting Started

### Windows
1. Open PowerShell or Command Prompt.
2. Run as a client (entry node):
   ```powershell
   .\shadowlink-windows-amd64.exe --role entry --socks 1080 --sysproxy
   ```
   *This automatically configures your Windows proxy settings to use the dVPN!*

### macOS / Linux
1. Open Terminal.
2. Make the binary executable (e.g., for Linux): 
   ```bash
   chmod +x shadowlink-linux-amd64
   ```
3. Run as a client:
   ```bash
   ./shadowlink-linux-amd64 --role entry --socks 1080
   ```

### Community Participation
ShadowLink is a purely free, decentralized network. You can help power the network by running your machine as a relay or exit node.

**Start a Relay Node:**
```bash
./shadowlink-linux-amd64 --role relay --port 9001
```

**Start an Exit Node:**
```bash
./shadowlink-linux-amd64 --role exit --port 9002
```
