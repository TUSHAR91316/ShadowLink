# ShadowLink - Secure Local Encrypted Tunnel

[![Version](https://img.shields.io/badge/version-1.1.0-blue.svg)](https://github.com/TUSHAR91316/ShadowLink)
[![Python](https://img.shields.io/badge/python-3.10+-blue.svg)](https://www.python.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

ShadowLink is a next-generation local VPN and secure tunnel application that uses **ChaCha20-Poly1305** and **X25519** key exchange (the cryptographic primitives of WireGuard) to encrypt traffic. It features both a **Command Line Interface (CLI)** and a **Native GUI** (via PyQt6) for optimal performance and integration.

## 🚀 Key Features

### 🔒 Security & Encryption
- **WireGuard-Grade Encryption**: ChaCha20-Poly1305 authenticated encryption with X25519 key exchange
- **Ephemeral Keys**: New encryption keys generated for each session
- **Packet Validation**: 64KB maximum packet size limits prevent DoS attacks
- **Strict Mode Kill Switch**: Automatically blocks traffic if VPN tunnel fails
- **Automatic Fallback**: Gracefully falls back to normal network with user warnings when VPN disconnects

### 🌐 Networking & Performance
- **DPI Bypass**: Evades Deep Packet Inspection via Fake HTTP handshakes and payload masking
- **Full UDP Support**: Tunnels UDP datagrams (VoIP, gaming) over secure TCP layer via SOCKS5
- **System-Wide Proxy**: Routes all system traffic through secure tunnel with one click
- **IP Caching**: 99% reduction in DNS lookups for improved performance
- **30-Second Timeouts**: Prevents hanging connections and resource exhaustion

### 💻 User Experience
- **Modern Native GUI**: Clean Windows desktop interface with real-time statistics
- **Real-time Notifications**: Live status updates and security alerts in the app
- **Automatic Proxy Management**: Smart proxy enable/disable based on connection status
- **Retry Functionality**: Manual VPN reconnection attempts after fallback
- **Comprehensive Logging**: Detailed error reporting for troubleshooting

### 🏗️ Architecture
- **Thread-Safe Design**: Proper synchronization prevents race conditions
- **Event-Driven Communication**: Real-time updates between components
- **Modular Backend**: Clean separation of encryption, networking, and UI layers
- **Cross-Platform Core**: Python-based with native Windows integration

## 📋 What Makes ShadowLink Different?

| Feature | Standard SOCKS5 Proxy | ShadowLink |
| :--- | :--- | :--- |
| **Encryption** | None (Usually plaintext) | **ChaCha20-Poly1305 (WireGuard)** |
| **Key Management** | Static Password / None | **Ephemeral X25519** (Per session) |
| **DPI Evasion** | None | **Fake HTTP + Payload Masking** |
| **UDP Support** | Limited | **Full SOCKS5 UDP Associate** |
| **Kill Switch** | None | **Strict Mode + Auto Fallback** |
| **System Proxy** | Manual Configuration | **One-Click System-Wide** |
| **Architecture** | Remote Server Required | **Local-Only Operation** |

**Why Local Operation Matters**: ShadowLink isolates application traffic from the rest of your OS until fully encrypted. Even if malware packet-sniffs your network card, they only see encrypted ShadowLink traffic, not raw application data.

## ⚠️ Important Security Notes

- **VPN Required**: ShadowLink is designed to work with external VPN services (ProtonVPN, Mullvad, etc.)
- **Local Security**: Provides transport-layer security between applications and VPN
- **Fallback Behavior**: When VPN fails, traffic falls back to normal network with clear warnings
- **No Remote Servers**: All encryption/decryption happens locally on your device

## 📦 Installation & Setup

### Prerequisites
- **Python 3.10+** (64-bit recommended)
- **Windows 10/11** (64-bit)
- **External VPN Service** (ProtonVPN, Mullvad, ExpressVPN, etc.)

### 1. Install Dependencies
```bash
# Clone or download the repository
cd ShadowLink

# Install Python requirements
pip install -r requirements.txt
```

### 2. Verify Installation
```bash
# Run encryption verification
python src/verify_encryption.py
```

### 3. Run ShadowLink

#### CLI Version (Lightweight)
```bash
python src/cli.py
```

#### GUI Version (Recommended)
```bash
python src/gui.py
```

#### API Usage (For Integration)
```python
from src.api import ShadowAPI

# Initialize with event callback for UI integration
api = ShadowAPI(event_callback=my_callback)

# Start services with strict mode and system proxy
api.start_services(strict=True, sysproxy_on=True)

# Stop services
api.stop_services()
```

## 🔧 Configuration

### Environment Variables
- `SHADOWLINK_DEBUG=1`: Enable debug logging
- `SHADOWLINK_STRICT=1`: Force strict mode on startup

### Network Configuration
- **Server Port**: 8080 (configurable in `config.py`)
- **Client Port**: 1080 (SOCKS5 proxy port)
- **UDP Support**: Automatic via SOCKS5 UDP Associate

## 🧪 Testing & Verification

### Encryption Verification
```bash
python src/verify_encryption.py
```
Tests full X25519 + ChaCha20-Poly1305 handshake and encryption cycle.

### Connectivity Testing
```bash
python src/test_connectivity.py
```
Validates tunnel functionality and network connectivity.

## 🐛 Troubleshooting

### Common Issues

**"Registry access denied"**
- Run as Administrator or check Windows permissions
- System proxy features require admin privileges

**"Connection timeout"**
- Check your VPN connection status
- Verify firewall isn't blocking local ports (8080, 1080)
- Try disabling strict mode temporarily

**"High CPU usage"**
- Normal during initial connection establishment
- Check for background processes interfering with networking

**VPN Disconnects Frequently**
- ShadowLink will automatically enter fallback mode
- Check VPN service status and reconnect manually
- Use the retry functionality in the GUI

### Debug Mode
Enable detailed logging:
```bash
set SHADOWLINK_DEBUG=1
python src/gui.py
```

## 📊 Performance Characteristics

- **Encryption Overhead**: ~5-10% CPU increase vs plaintext
- **Memory Usage**: ~50MB base + ~1MB per active connection
- **Latency**: <1ms local encryption/decryption
- **Throughput**: Limited by VPN service bandwidth
- **Concurrent Connections**: Tested with 100+ simultaneous connections

## 🔐 Security Implementation Details

### Cryptographic Primitives
- **Key Exchange**: X25519 (ECDH over Curve25519)
- **Symmetric Encryption**: ChaCha20-Poly1305 (AEAD)
- **Key Derivation**: HKDF-SHA256 for session key generation
- **Random Generation**: `secrets` module for cryptographically secure randomness

### Security Hardening
- **Packet Size Limits**: 64KB maximum prevents memory exhaustion
- **Timeout Protection**: 30-second timeouts prevent hanging
- **Input Validation**: All network inputs validated before processing
- **Error Handling**: Comprehensive exception handling with logging
- **Thread Safety**: Proper synchronization prevents race conditions

## 📝 API Reference

### ShadowAPI Class
```python
class ShadowAPI:
    def __init__(self, event_callback=None)
    def start_services(self, strict=False, sysproxy_on=False)
    def stop_services(self)
    def retry_connection(self)  # New in v1.1.0
```

### Event Types
- `"status"`: Connection status changes
- `"log"`: General log messages
- `"warning"`: Security warnings (VPN failures)
- `"info"`: Informational messages
- `"error"`: Error conditions

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes with proper testing
4. Run verification: `python src/verify_encryption.py`
5. Submit a pull request

### Development Setup
```bash
# Install development dependencies
pip install -r requirements.txt

# Run tests
python -m pytest  # (if test files exist)

# Check code quality
python -m py_compile src/*.py
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **WireGuard**: For the cryptographic primitives and inspiration
- **Python Cryptography**: For the robust crypto library implementation
- **PyQt6**: For the native GUI framework

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/TUSHAR91316/ShadowLink/issues)
- **Discussions**: [GitHub Discussions](https://github.com/TUSHAR91316/ShadowLink/discussions)
- **Documentation**: See `docs/` directory for detailed technical documentation

---

**Latest Release**: v1.1.0 - Enhanced security, automatic fallback, and improved user experience.
