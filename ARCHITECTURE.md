# ShadowLink Architecture

ShadowLink is a **decentralized VPN (dVPN)** built on libp2p P2P networking and XChaCha20-Poly1305 onion routing. All nodes are equal peers — there are no central servers or coordination points.

---

## 1. System Overview

```mermaid
graph TD
    subgraph "User Device"
        Browser["Browser / App"]
        GUI["Flutter Desktop GUI"]
        SOCKS["Local SOCKS5 Proxy\n(127.0.0.1:1080)"]
        GoBinary["Go Daemon Binary"]
        Registry["Windows Registry\n(System Proxy)"]
    end

    subgraph "dVPN Network (libp2p P2P)"
        Entry["Entry Node\n(This device)"]
        Relay["Relay Node\n(Volunteer peer)"]
        Exit["Exit Node\n(Volunteer peer)"]
    end

    Internet(("Public Internet"))
    DHT["Kademlia DHT\n(Peer Discovery)"]

    Browser -->|"SOCKS5 traffic"| SOCKS
    GUI -->|"spawn + monitor"| GoBinary
    GoBinary -->|"EnableSOCKS5()"| Registry
    Registry -->|"redirects all\nsystem traffic"| SOCKS
    SOCKS --> GoBinary
    GoBinary --> Entry

    Entry -->|"ECDH + AES stream"| Relay
    Relay -->|"ECDH + AES stream"| Exit
    Exit -->|"TCP cleartext"| Internet

    Entry <-->|"Announce/FindPeers"| DHT
    Relay <-->|"Announce"| DHT
    Exit <-->|"Announce"| DHT
```

---

## 2. Circuit Negotiation Protocol

Every connection uses **ephemeral X25519 ECDH** — no session key is ever reused.

```mermaid
sequenceDiagram
    participant E as Entry Node
    participant R as Relay Node
    participant X as Exit Node
    participant I as Internet

    Note over E,R: Stream 1: Entry → Relay
    E->>R: "RELAY\n<target:port>\n"
    E->>R: X25519 Public Key (32 bytes)
    R->>E: X25519 Public Key (32 bytes)
    Note over E,R: Both derive shared secret K₁

    Note over R,X: Stream 2: Relay → Exit (relay initiates)
    R->>X: "<target:port>\n"
    R->>X: X25519 Public Key (32 bytes)
    X->>R: X25519 Public Key (32 bytes)
    Note over R,X: Both derive shared secret K₂

    Note over E,I: Data flows (double-hop encrypted)
    E->>R: Encrypt(K₁, payload)
    R->>X: Decrypt(K₁) → Encrypt(K₂, payload)
    X->>I: Decrypt(K₂) → TCP connect(target)
    I->>X: Response
    X->>R: Encrypt(K₂, response)
    R->>E: Decrypt(K₂) → Encrypt(K₁, response)
```

---

## 3. Node Roles

A single binary can run as any combination of roles simultaneously:

| Flag | Role | Behaviour |
|---|---|---|
| `--entry` | Entry Node | Opens local SOCKS5 proxy; routes browser traffic into dVPN |
| `--relay` | Relay Node | Accepts streams; bridges Entry→Exit with re-encryption |
| `--exit` | Exit Node | Connects to real internet target; proxies cleartext traffic |
| `--sysproxy` | (modifier) | Configures Windows registry to route all system traffic |

---

## 4. Serverless Peer Discovery (DHT)

```mermaid
sequenceDiagram
    participant R as Relay/Exit Node
    participant DHT as Kademlia DHT
    participant E as Entry Node

    R->>DHT: Announce("shadowlink-relay")
    R->>DHT: Announce("shadowlink-exit")

    E->>DHT: FindPeers("shadowlink-relay") → [relay1, relay2]
    E->>DHT: FindPeers("shadowlink-exit")  → [exit1, exit2]

    Note over E: Builds circuit: relay1 → exit1
    E->>R: Connect + NewStream(ProtocolID)
```

Bootstrap nodes seed the initial DHT connection (public libp2p/IPFS peers).
After bootstrapping, node discovery is fully decentralized.

---

## 5. Encryption Stack

```
┌─────────────────────────────────────────────┐
│              Entry→Relay Tunnel              │
│   XChaCha20-Poly1305  key=K₁ (ECDH-X25519)  │
│  ┌───────────────────────────────────────┐  │
│  │           Relay→Exit Tunnel           │  │
│  │  XChaCha20-Poly1305  key=K₂           │  │
│  │  ┌─────────────────────────────────┐  │  │
│  │  │    Plaintext TCP Payload        │  │  │
│  │  │    (e.g. TLS to example.com)    │  │  │
│  │  └─────────────────────────────────┘  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

Each hop peels one encryption layer:
- **Relay** decrypts with K₁, re-encrypts with K₂ — never sees plaintext
- **Exit** decrypts with K₂, sees plaintext — never knows the entry IP

Every frame is prefixed with a **4-byte big-endian length header** to prevent TCP partial-read decryption corruption.

---

## 6. GUI ↔ Daemon IPC

```mermaid
graph LR
    Flutter["Flutter GUI\n(Windows Desktop)"]
    Daemon["Go Daemon\n(shadowlink.exe)"]
    SOCKS["SOCKS5\n:1080"]
    Proxy["Windows Registry\nSystem Proxy"]

    Flutter -->|"Process.start(args)"| Daemon
    Flutter -->|"stdout log monitoring"| Daemon
    Flutter -->|"Process.kill() + --reset-proxy"| Daemon
    Daemon --> SOCKS
    Daemon --> Proxy
```

The Flutter GUI manages the Go binary as a child process. On abnormal exit,
Flutter invokes `shadowlink.exe --reset-proxy` as a failsafe to restore
the Windows system proxy to its original state, preventing a "broken internet" scenario.

---

## 7. File Structure

```
cmd/shadowlink/main.go          ← Entrypoint, flags, EULA, signal handling
internal/
  crypto/
    encryption.go               ← ChaCha20-Poly1305 Encrypt/Decrypt/GenerateKey
    ecdh.go                     ← X25519 PerformECDH / RespondECDH
  discovery/dht.go              ← libp2p host + KadDHT Announce/FindPeers
  network/
    circuit.go                  ← DialCircuit: 3-hop routing with ECDH + framing
    handler.go                  ← HandleStream: relay forwarding + exit proxying
  onion/onion.go                ← WrapPayload / UnwrapPayload (layered AEAD)
  socks5/server.go              ← Local SOCKS5 server (entry node only)
  sysproxy/
    sysproxy_windows.go         ← Windows registry proxy management
    sysproxy_other.go           ← no-op stub for macOS/Linux
shadowlink_gui/lib/
  main.dart                     ← App entrypoint → EulaScreen
  screens/eula_screen.dart      ← EULA enforcement (mandatory, non-skippable)
  screens/dashboard_screen.dart ← Connect/Disconnect + role toggles
  services/daemon_service.dart  ← Go binary lifecycle management
  theme/app_theme.dart          ← Design system (Deep Obsidian + Neon Cyan)
```
