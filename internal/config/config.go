// Package config defines shared constants used across the ShadowLink codebase.
// All hardcoded values should be placed here so they can be changed in one place.
package config

import "time"

// ─── Network Protocol ────────────────────────────────────────────────────────

// ProtocolID is the libp2p stream protocol identifier for ShadowLink.
// Changing this is a breaking protocol change — all nodes must agree on the same value.
const ProtocolID = "/shadowlink/1.0.0"

// ExtendHeader is the sentinel string written to a relay node, followed by the target exit node PeerID.
const ExtendHeader = "EXTEND"

// ConnectHeader is the sentinel string written to an exit node, followed by the target internet address.
const ConnectHeader = "CONNECT"

// RendezvousRelay is the DHT rendezvous key used by relay nodes to announce themselves.
const RendezvousRelay = "shadowlink-relay"

// RendezvousExit is the DHT rendezvous key used by exit nodes to announce themselves.
const RendezvousExit = "shadowlink-exit"

// ─── Ports ───────────────────────────────────────────────────────────────────

// DefaultP2PPort is the default port for incoming libp2p P2P connections.
const DefaultP2PPort = 9000

// DefaultSOCKSPort is the default local SOCKS5 proxy port for entry nodes.
const DefaultSOCKSPort = 1080

// ─── Framing ─────────────────────────────────────────────────────────────────

// MaxFrameSize caps individual encrypted frames at 128 KiB to prevent OOM-DoS attacks.
const MaxFrameSize = 128 * 1024

// ─── Crypto ──────────────────────────────────────────────────────────────────

// HKDFInfo is the domain-separation label used during HKDF-SHA256 key derivation.
// Changing this value is a breaking protocol change.
const HKDFInfo = "shadowlink/v1/session-key"

// ─── Timeouts ────────────────────────────────────────────────────────────────

// DefaultConnectionTimeout is the maximum time an entry node waits for the
// SOCKS5 proxy to report a successful circuit before giving up.
const DefaultConnectionTimeout = 30 * time.Second

// ─── File Names ──────────────────────────────────────────────────────────────

// EULAFileName is the sentinel file written to disk when the user accepts the EULA.
const EULAFileName = ".shadowlink_accepted"

// ─── Bootstrap ───────────────────────────────────────────────────────────────

// DefaultBootstrapPeers are the well-known libp2p bootstrap nodes used to seed the DHT.
// Without these, a fresh node cannot discover any peers on the public network.
var DefaultBootstrapPeers = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
}
