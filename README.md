# ShadowLink 🛡️

![ShadowLink Logo](docs/logo.png)

> **Local Double-Encryption Secure Tunnel with Ephemeral Keys**

ShadowLink is a specialized, local-only VPN tunnel designed for maximum privacy and security. Unlike standard VPN clients, ShadowLink runs a local encrypted loopback (Client → Server) on your machine, ensuring that your traffic is double-encrypted before it even hits your network card or upstream VPN (like ProtonVPN).

## 🚀 Features

-   **Double Encryption**:
    -   **Layer 1**: ShadowLink (AES-256-GCM) - Encryption happens *inside* your User Space.
    -   **Layer 2**: Your System VPN (e.g., ProtonVPN, NordVPN, etc.) - Encryption happens at the Network Interface level.
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

![Architecture Schematic](docs/architecture.png)

### Data Flow Diagram

```mermaid
flowchart LR
    %% Styles
    classDef client fill:#2d2d2d,stroke:#00ff41,stroke-width:2px,color:#fff;
    classDef server fill:#1a1a1a,stroke:#00bfff,stroke-width:2px,color:#fff;
    classDef vpn fill:#1a1a1a,stroke:#ff00ff,stroke-width:2px,stroke-dasharray: 5 5,color:#fff;
    classDef internet fill:#000000,stroke:#fff,stroke-width:1px,color:#fff;
    classDef warning fill:#3d0000,stroke:#ff0000,stroke-width:2px,color:#fff;

    subgraph UserPC ["🖥️ YOUR PC (Localhost)"]
        direction LR
        App[("📱 Application<br/>(Browser/Game)")] -->|Plain Traffic| Client["🔒 Client<br/>(Encyptor)"]
        Client ===|"⚡ Layer 1: AES-256"| Server["🛡️ Server<br/>(Decryptor)"]
        
        Server --"Strict Mode Check"--> Gate{"Safe IP?"}
    end

    subgraph System ["🌐 SYSTEM NETWORK"]
        direction LR
        Gate --"Yes"--> SysStack["Network Interface"]
        Gate --"No"--> KillSwitch["⛔ BLOCK"]
        
        SysStack -.->|"Plain ISP Traffic"| ISP_Danger["⚠️ Unsafe ISP Node"]
        SysStack ==>|"⚡ Layer 2: VPN"| Proton["🛡️ ProtonVPN"]
    end

    Proton ==>|"Double Encrypted"| ISP_Safe["☁️ ISP Gateway"]
    ISP_Safe --> WWW(("🌍 Internet"))
    ISP_Danger --> WWW

    %% Apply Styles
    class App,SysStack client;
    class Client,Server server;
    class Proton vpn;
    class WWW internet;
    class KillSwitch,ISP_Danger warning;
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
