# ShadowLink — Security Policy

## Supported Versions

| Version | Supported | Notes |
|---|---|---|
| `2.x.x` | ✅ Active | Go + Flutter architecture |
| `1.x.x` | ❌ End of Life | Legacy Electron + Python |
| `< 1.0` | ❌ End of Life | Legacy Python-only |

Only the `master` branch receives security patches. Older releases are not backported.

---

## Scope of Security Concerns

ShadowLink's security model depends on several independently important components.
We especially welcome reports in these areas:

| Area | Attack Surface |
|---|---|
| **Cryptography** | XChaCha20-Poly1305 encryption, X25519 ECDH key exchange, nonce reuse |
| **Traffic Deanonymization** | Timing attacks, circuit correlation, exit node logging |
| **DHT Poisoning** | Sybil attacks against libp2p Kademlia DHT, routing table manipulation |
| **Memory Safety** | Buffer overflows, use-after-free in the Go binary |
| **Exit Node Abuse** | The exit node's IP is exposed to third-party traffic — exit node operators could be targeted |
| **EULA Bypass** | Any mechanism allowing the binary to run without user accepting the EULA |
| **Proxy Leak** | Windows registry proxy not being restored after crash (system-wide traffic leak) |
| **GUI → Daemon IPC** | Process injection, argument injection into the child Go process |

---

## Reporting a Vulnerability

**DO NOT open a public GitHub issue for security vulnerabilities.**

Send a private report directly to the repository maintainer via GitHub's
[private vulnerability reporting](https://github.com/TUSHAR91316/ShadowLink/security/advisories/new).

### Your report should include:
1. **Description** — What is the vulnerability and what is its impact?
2. **Reproduction steps** — Minimal steps to reproduce
3. **Severity assessment** — Your estimate (Critical / High / Medium / Low)
4. **Affected versions** — Which binary version(s) are affected?
5. **Suggested mitigation** — If you have one (optional but very helpful)

We will:
- Acknowledge your report within **48 hours**
- Provide status updates every **7 days**
- Work with you on a coordinated disclosure timeline
- Credit you in the release notes (unless you request anonymity)

---

## Security Architecture Summary

- **Encryption**: XChaCha20-Poly1305 (256-bit key, 192-bit random nonce per message)
- **Key Exchange**: X25519 ECDH (stdlib `crypto/ecdh`) — ephemeral per-session keys
- **Forward Secrecy**: Yes — each connection uses a fresh ephemeral key pair
- **Authentication**: AEAD tag provides message authentication (not identity authentication)
- **No Central Server**: The developers operate zero infrastructure — all traffic is P2P
- **Exit Node Trust**: Exit node operators can see destination IPs; they cannot see source (entry) IPs

---

## Responsible Disclosure Policy

We follow a **90-day coordinated disclosure** window:
- Patch will be developed and tested privately
- Binary release published to GitHub Releases
- CVE requested (if applicable)
- Full public disclosure after 90 days or patch release, whichever is earlier
