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

// ─── Node Roles ──────────────────────────────────────────────────────────────

// RoleRelay, RoleExit and RoleRelayExit name the roles a node can serve incoming
// streams under. A node only honours the commands its role allows, so a relay-only
// node never egresses traffic to the internet on behalf of peers.
const (
	RoleRelay     = "relay"
	RoleExit      = "exit"
	RoleRelayExit = "relay+exit"
)

// ─── Ports ───────────────────────────────────────────────────────────────────

// DefaultP2PPort is the default port for incoming libp2p P2P connections.
const DefaultP2PPort = 9000

// DefaultSOCKSPort is the default local SOCKS5 proxy port for entry nodes.
const DefaultSOCKSPort = 1080

// ─── Framing ─────────────────────────────────────────────────────────────────

// MaxFrameSize caps individual encrypted frames at 128 KiB to prevent OOM-DoS attacks.
const MaxFrameSize = 128 * 1024

// HKDFInfo is the legacy domain-separation label used during HKDF-SHA256 key derivation.
// Deprecated: Use HKDFInfoRelay or HKDFInfoExit for explicit domain separation across circuit hops.
const HKDFInfo = "shadowlink/v1/session-key"

// HKDFInfoRelay is the domain-separation label used during HKDF-SHA256 key derivation for relay hops.
const HKDFInfoRelay = "shadowlink-v2-relay-key"

// HKDFInfoExit is the domain-separation label used during HKDF-SHA256 key derivation for exit hops.
const HKDFInfoExit = "shadowlink-v2-exit-key"

// ─── Timeouts ────────────────────────────────────────────────────────────────

// DefaultConnectionTimeout is the maximum time an entry node spends building a
// circuit (peer discovery plus every relay/exit attempt) before giving up.
const DefaultConnectionTimeout = 30 * time.Second

// PeerLookupTimeout bounds a single DHT rendezvous lookup so a slow query cannot
// consume the whole circuit-building budget.
const PeerLookupTimeout = 15 * time.Second

// AnnounceRetryInterval is how long a relay/exit node waits before retrying a
// failed DHT advertisement (e.g. when the routing table is still empty at startup).
const AnnounceRetryInterval = 2 * time.Minute

// ─── File Names ──────────────────────────────────────────────────────────────

// EULAFileName is the sentinel file written to disk when the user accepts the EULA.
const EULAFileName = ".shadowlink_accepted"

// EULAEnvVar is the environment variable that can be set to "1" or "true" to accept the EULA non-interactively.
const EULAEnvVar = "SHADOWLINK_ACCEPT_EULA"

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
