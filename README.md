# ShadowLink 🛡️

![ShadowLink Logo](docs/logo.png)

> **Local Double-Encryption Secure Tunnel with Ephemeral Keys**

ShadowLink is a specialized, local-only VPN tunnel designed for maximum privacy and security. Unlike standard VPN clients, ShadowLink runs a local encrypted loopback (Client → Server) on your machine, ensuring that your traffic is double-encrypted before it even hits your network card or upstream VPN (like ProtonVPN).

## 🚀 Features

-   **Double Encryption**:
    -   **Layer 1**: ShadowLink (AES-256-GCM) - Encryption happens *inside* your User Space.
    -   **Layer 2**: Your System VPN (e.g., ProtonVPN) - Encryption happens at the Network Interface level.
-   **Maximum Security Protocol**:
    -   **AES-256-GCM**: Military-grade encryption for the data payload.
    -   **X25519 (Curve25519)**: Ephemeral Elliptic Curve Diffie-Hellman Key Exchange.
    -   **Forward Secrecy**: A unique, random session key is generated for **every single connection**. Keys exist only in RAM and are wiped on disconnect.
-   **Strict Mode (Kill Switch)**:
    -   Optionally blocks traffic if it detects your public IP matches your ISP's IP (prevents accidental leaks if your VPN drops).
-   **Cyber-Aesthetic GUI**:
    -   Built with `customtkinter` for a modern, dark-mode "hacker" aesthetic.
    -   Real-time bandwidth visualization.

## 🏗️ Architecture

ShadowLink differs from traditional VPNs by acting as a **Pre-Encryption Wrapper**.

```mermaid
graph TD
    subgraph Localhost ["Your PC (Localhost)"]
        Browser["Browser / App"] -->|"Plain SOCKS5"| Client["ShadowLink Client :1080"]
        Client == "Layer 1 Encrypted (AES-256)" ==> Server["ShadowLink Server :8443"]
        
        Server -- "Strict Mode Check (Kill Switch)" --> Check{"Public IP != ISP?"}
        Check -- "Safe" --> OS["OS Network Stack"]
        Check -- "Unsafe" --> Block["BLOCK TRAFFIC"]
        
        style Client fill:#333,stroke:#0ff,stroke-width:2px,color:#fff
        style Server fill:#333,stroke:#f0f,stroke-width:2px,color:#fff
        style Check fill:#550,stroke:#ff0,stroke-width:2px,color:#fff
    end
    
    subgraph System ["System Network Level"]
        OS -->|"VPN On"| Proton["ProtonVPN / Upstream VPN"]
        OS -.->|"VPN Off"| Direct["ISP Gateway (Unmasked)"]
        
        Proton == "Layer 2 Encrypted (Tunnel)" ==> Masked["ISP Gateway (Masked)"]
    end
    
    Masked --> Internet
    Direct --> Internet
    
    classDef secure fill:#004d00,stroke:#0f0,color:#fff;
    classDef danger fill:#4d0000,stroke:#f00,color:#fff;
    class Proton secure;
    class Direct danger;
```

## ❓ What Makes It Different?

| Feature | Standard SOCKS5 Proxy | ShadowLink |
| :--- | :--- | :--- |
| **Encryption** | None (Usually plaintext) | **AES-256-GCM** |
| **Key Management** | Static Password / None | **Ephemeral X25519** (New key per session) |
| **Purpose** | IP Masking | **Traffic Obfuscation** & Layered Security |
| **Dependency** | Remote Server | **Local-Only** (Server runs on your localhost) |

**Why use this locally?** 
It isolates your application traffic from the rest of the OS until it is fully encrypted. Even if malware on your PC packet-sniffs your network card, they only see the encrypted ShadowLink traffic, not the raw application data.

## ⚠️ Limitations

1.  **TCP Only**: Currently supports SOCKS5 CONNECT method (TCP). UDP (e.g., for gaming/VoIP) is not yet supported.
2.  **Performance Overhead**: Double encryption (ShadowLink + ProtonVPN) adds a small amount of latency and CPU overhead.
3.  **Manual Proxy Config**: You must configure your browser/app to use `SOCKS5 127.0.0.1:1080`. It does not automatically route *all* system traffic (split-tunneling by design).

## 🛠️ Installation & Usage

### Binaries
Download the latest `ShadowLink_Setup.exe` from the Releases page.

### Running from Source

**Requirements**:
-   Python 3.10+
-   `pip install -r requirements.txt`

**Start the GUI**:
```bash
python src/gui.py
```

## 📄 License

MIT License. Built for educational and privacy-enhancing purposes.
