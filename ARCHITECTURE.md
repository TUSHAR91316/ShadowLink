# 🏛️ ShadowLink System Architecture

ShadowLink is a **Decentralized Virtual Private Network (dVPN)** engineered on `libp2p` peer-to-peer networking, ephemeral **X25519 ECDH** session derivation, and **XChaCha20-Poly1305** multi-hop layered onion routing. All network participants operate as autonomous peers without reliance on centralized directory authorities or tracking servers.

---

## 1. System Overview

```mermaid
graph TD
    subgraph "User Device (Client / Entry Node)"
        App["Web Browser / System Applications"]
        GUI["Flutter Desktop / Mobile GUI"]
        SOCKS["Local SOCKS5 Proxy\n(127.0.0.1:1080)"]
        Core["ShadowLink Go Engine\n(Daemon / gomobile)"]
        SysProxy["OS System Proxy Settings\n(Windows Registry / Mobile VPN)"]
    end

    subgraph "Decentralized P2P Network (libp2p DHT)"
        RelayNode["Relay Node (Hop 1)\nBlind Routing Intermediate"]
        ExitNode["Exit Node (Hop 2)\nInternet Egress Gateway"]
        DHT["Kademlia DHT\n(Autonomous Discovery)"]
    end

    Internet(("Public Internet Target\n(e.g., example.com:443)"))

    App -->|"Plaintext / TLS SOCKS5"| SOCKS
    GUI -->|"Process lifecycle / MethodChannel"| Core
    Core -->|"Configure SOCKS5 proxy"| SysProxy
    SysProxy -->|"Redirects network traffic"| SOCKS
    SOCKS -->|"Custom Dialer (DialCircuit)"| Core

    Core <-->|"FindPeers / Announce"| DHT
    RelayNode <-->|"Announce('shadowlink-relay')"| DHT
    ExitNode <-->|"Announce('shadowlink-exit')"| DHT

    Core -->|"Outer Wrap: Encrypt(RelayKey)"| RelayNode
    RelayNode -->|"Inner Wrap: Encrypt(ExitKey)"| ExitNode
    ExitNode -->|"TCP Dial"| Internet
```

---

## 2. True Onion Routing: Circuit Negotiation Protocol

Every connection generates independent, ephemeral **X25519 ECDH** key pairs with **HKDF-SHA256** domain separation. Forward secrecy is absolute: keys are never written to disk or reused across circuits.

```mermaid
sequenceDiagram
    autonumber
    participant E as Entry Node (User)
    participant R as Relay Node (Intermediate)
    participant X as Exit Node (Egress)
    participant I as Target Internet Host

    Note over E,R: Phase 1 — Entry to Relay Handshake
    E->>R: Stream Open: "EXTEND\n<ExitPeerID>\n"
    E->>R: X25519 Public Key (32 bytes) [Initiator]
    R->>E: X25519 Public Key (32 bytes) [Responder]
    Note over E,R: Both derive shared secret: RelayKey (via HKDF-SHA256)

    Note over R,X: Phase 2 — Relay Bridges to Exit
    R->>X: DHT Lookup & Connect to ExitPeerID
    R->>X: Stream Open (/shadowlink/1.0.0)
    Note over R: Relay establishes transparent bidirectional bridge

    Note over E,X: Phase 3 — End-to-End Entry to Exit Handshake (Through Relay)
    E->>R: Encrypt(RelayKey, "CONNECT\n<TargetAddr>\n")
    R->>X: Decrypt(RelayKey) → Forwards "CONNECT\n<TargetAddr>\n"
    E->>R: Encrypt(RelayKey, X25519 Public Key [32 bytes])
    R->>X: Decrypt(RelayKey) → Forwards X25519 Public Key
    X->>R: X25519 Public Key (32 bytes) [Responder]
    R->>E: Encrypt(RelayKey, X25519 Public Key)
    Note over E,X: Both derive shared secret: ExitKey (via HKDF-SHA256)

    Note over E,I: Phase 4 — Double-Encrypted Onion Data Tunnel
    E->>E: Payload → Encrypt(ExitKey) → Encrypt(RelayKey) → Prepend 4-byte FrameLen
    E->>R: [Length Header][Double Encrypted Frame]
    R->>R: Reads Frame → Decrypt(RelayKey) → Yields [Single Encrypted Frame]
    R->>X: Forwards [Single Encrypted Frame] (Relay NEVER sees plaintext)
    X->>X: Decrypt(ExitKey) → Yields Original Payload
    X->>I: TCP connect & transfer to TargetAddr
    I->>X: Response Data
    X->>X: Encrypt(ExitKey, Response)
    X->>R: Forwards [Single Encrypted Frame]
    R->>R: Encrypt(RelayKey, Frame)
    R->>E: Forwards [Double Encrypted Frame]
    E->>E: Decrypt(RelayKey) → Decrypt(ExitKey) → Original Response
```

---

## 3. Node Roles & Responsibilities

| Role | CLI Flag | Functionality | Cryptographic Visibility |
|---|---|---|---|
| **Entry Node** | `--entry` | Runs local SOCKS5 proxy (`127.0.0.1:1080`); wraps data with `[ExitKey, RelayKey]`. | Knows User IP and Relay IP. Does not expose target to Relay. |
| **Relay Node** | `--relay` | Accepts `EXTEND` requests; bridges traffic to the designated Exit node. | Knows Entry IP and Exit IP. **Cannot read traffic payload or target destination.** |
| **Exit Node** | `--exit` | Accepts `CONNECT` requests; dials target host on the public internet. | Knows Target Host and Relay IP. **Does not know User IP.** |
| **System Proxy** | `--sysproxy` | Directs OS-wide TCP traffic to local SOCKS5 proxy via Windows Registry. | Client OS automation only. |

---

## 4. Serverless Peer Discovery (Kademlia DHT)

```mermaid
graph LR
    subgraph "DHT Rendezvous Namespace"
        RelayRendezvous["'shadowlink-relay'"]
        ExitRendezvous["'shadowlink-exit'"]
    end

    Relay1["Relay Peer 1"] -->|Advertise| RelayRendezvous
    Relay2["Relay Peer 2"] -->|Advertise| RelayRendezvous
    Exit1["Exit Peer 1"] -->|Advertise| ExitRendezvous
    Exit2["Exit Peer 2"] -->|Advertise| ExitRendezvous

    Client["Entry Client"] -->|FindPeers| RelayRendezvous
    Client -->|FindPeers| ExitRendezvous
```

1. **Bootstrap Phase**: The node dials multiaddresses of well-known decentralized seed peers (`DefaultBootstrapPeers`).
2. **Advertisement Phase**: Relays and Exits advertise their respective roles to the Kad-DHT routing table.
3. **Discovery Phase**: Entry nodes query the DHT for active peers, shuffle results using Fisher-Yates randomization to eliminate traffic analysis bias, and dial circuits on demand.

---

## 5. Layered Cryptographic Encapsulation

```
┌────────────────────────────────────────────────────────┐
│ Outer Layer: Entry ↔ Relay Hop                         │
│ Cipher: XChaCha20-Poly1305 AEAD                        │
│ Key: RelayKey (X25519 ECDH + HKDF-SHA256)              │
│ ┌────────────────────────────────────────────────────┐ │
│ │ Inner Layer: Entry ↔ Exit End-to-End Hop           │ │
│ │ Cipher: XChaCha20-Poly1305 AEAD                    │ │
│ │ Key: ExitKey (X25519 ECDH + HKDF-SHA256)           │ │
│ │ ┌────────────────────────────────────────────────┐ │ │
│ │ │ Plaintext Application Payload                  │ │ │
│ │ │ (HTTP/HTTPS, SOCKS5 TCP Stream)                │ │ │
│ │ └────────────────────────────────────────────────┘ │ │
│ └────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────┘
```

### Framing & Zero-Allocation Memory Model
All frames transmitted through `libP2PConn` follow a strict binary layout:
```
[4 Bytes Big-Endian Length Prefix][N Bytes XChaCha20-Poly1305 Ciphertext + 16 Byte Tag]
```
- **Max Frame Cap**: Individual frames are capped at `MaxFrameSize` (128 KiB) to prevent Out-Of-Memory (OOM) DoS attacks.
- **Zero Allocations on Read**: The connection wrapper reuses a pre-allocated internal buffer (`frameBuf`), preventing garbage collection thrashing during sustained high-bandwidth operations.

---

## 6. GUI Control Plane & Lifecycle Management

```mermaid
graph TD
    subgraph "Desktop Platforms (Windows / macOS / Linux)"
        FlutterDesktop["Flutter Desktop GUI"] -->|"Process.start(args)"| DaemonProcess["Go Daemon Binary (shadowlink.exe)"]
        DaemonProcess -->|"Real-time stdout log streaming"| FlutterDesktop
        FlutterDesktop -->|"Process.kill() + --reset-proxy failsafe"| DaemonProcess
    end

    subgraph "Mobile Platforms (Android / iOS)"
        FlutterMobile["Flutter Mobile App"] -->|"MethodChannel (connect/disconnect)"| MobileBindings["gomobile Bindings (MobileNode)"]
        MobileBindings -->|"In-process libp2p Host & SOCKS5"| NetworkInterface["Local Network Loopback"]
    end
```

---

## 7. Component Map

```
cmd/shadowlink/main.go            # Daemon CLI entrypoint, flag parsing, signal handling & EULA
internal/
  config/config.go                # Centralized protocol constants, rendezvous keys, port defaults
  crypto/
    ecdh.go                       # Ephemeral X25519 key negotiation & HKDF-SHA256 derivation
    encryption.go                 # XChaCha20-Poly1305 AEAD encryption & decryption
  discovery/dht.go                # libp2p host instantiation, Kad-DHT routing, peer discovery
  network/
    circuit.go                    # Circuit builder (3-hop via relay, 1-hop direct), libP2PConn framing
    handler.go                    # Protocol dispatcher (EXTEND relaying & CONNECT exit proxying)
  onion/onion.go                  # Recursive multi-layer encryption wrap & unwrap logic
  socks5/server.go                # RFC 1928 SOCKS5 proxy server with custom onion dialer
  sysproxy/
    sysproxy_windows.go           # Windows registry system proxy automation (HKCU)
    sysproxy_other.go             # Non-Windows fallback stub
mobile/shadowlink.go              # gomobile exported MobileNode interface for iOS and Android
shadowlink_gui/lib/               # Flutter cross-platform UI with cyber aesthetic
```
