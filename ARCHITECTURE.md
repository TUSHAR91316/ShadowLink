# ShadowLink v2 Architecture

Below is the high-level architecture diagram detailing how the Flutter GUI (v2), the Go Daemon, and the P2P Network interact.

```mermaid
graph TD
    %% User Space
    User((User))
    
    %% Frontend (Flutter)
    subgraph Frontend [Flutter Cross-Platform GUI]
        UI[Dashboard Screen]
        EULA[EULA Enforcement]
        DaemonMgr[Daemon Service]
        
        UI -->|Reads Status| DaemonMgr
        UI -->|Toggles Roles| DaemonMgr
        EULA -->|Writes .shadowlink_accepted| LocalFS[(Local File System)]
    end

    %% Backend (Go CLI Daemon)
    subgraph Backend [ShadowLink Go Daemon]
        CLI[main.go - Process Manager]
        SysProxy[SysProxy Manager]
        SOCKS[SOCKS5 Proxy Server]
        Onion[Multi-hop Onion Router]
        DHT[Kademlia DHT Discovery]
        LibP2P[libp2p Host Layer]

        CLI -->|Validates EULA| LocalFS
        CLI -->|Configures Windows Registry| SysProxy
        CLI -->|Spawns| SOCKS
        CLI -->|Spawns| Onion
        CLI -->|Spawns| DHT
        
        SOCKS -->|Dials via| Onion
        Onion -->|Locates Peers via| DHT
        Onion -->|Transmits over| LibP2P
        DHT -->|Announces/Queries| LibP2P
    end

    %% Network Layer
    subgraph Network [Decentralized P2P Network]
        EntryNode[Entry Nodes]
        RelayNode[Relay Nodes]
        ExitNode[Exit Nodes]
        
        EntryNode -.->|Encrypted Circuits| RelayNode
        RelayNode -.->|Encrypted Circuits| ExitNode
        ExitNode -.->|Cleartext| Clearnet((Public Internet))
    end

    %% Interactions
    User -->|Interacts| UI
    DaemonMgr -->|Spawns child process| CLI
    DaemonMgr -->|Reads stdout/stderr| CLI
    
    LibP2P <==>|Peer connections| Network
    
    %% Styling
    classDef flutter fill:#00569e,stroke:#00F0FF,stroke-width:2px,color:white;
    classDef go fill:#00add8,stroke:#161B22,stroke-width:2px,color:white;
    classDef p2p fill:#161B22,stroke:#2EA043,stroke-width:2px,color:white;
    
    class UI,EULA,DaemonMgr flutter;
    class CLI,SysProxy,SOCKS,Onion,DHT,LibP2P go;
    class EntryNode,RelayNode,ExitNode p2p;
```

### Component Breakdown
1. **Flutter GUI**: Operates entirely in user-space, providing the visual interface. It acts as an orchestrator, spawning the Go binary in the background and monitoring its `stdout`.
2. **Go Daemon**: The powerhouse of the application. It runs the libp2p node, handles multiplexed streams, wraps TCP in SOCKS5, and modifies system proxy settings.
3. **P2P Network**: The decentralized layer where traffic is bounced through relays before exiting to the clearnet.
