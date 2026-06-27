# Release v1.0.0 - Electron Evolution: Hybrid Architecture & Cyber UI

> **Major Update**: Replaced legacy GUI with a high-performance Electron + React frontend.

Includes `ShadowLink Setup 1.0.0.exe` for Windows x64.

---

## 🚀 Key Improvements

- **New UI**: Built with React, TailwindCSS, and Framer Motion for smooth animations and a modern "cyber" aesthetic.
- **Hybrid Core**:
  - **Frontend**: Electron (Process Management & UI).
  - **Backend**: Python (Encryption & Networking Logic) running as a secure subprocess.
- **Enhanced Security**: 
  - Verification script included (`src/verify_encryption.py`) to audit X25519 + AES-256-GCM implementation.
  - Strict separation of concerns between UI and Encryption layers.
- **System-Wide Proxy**: One-click toggle to route all system traffic through the secure tunnel.

## 📦 Download

| File | SHA256 Checksum |
| :--- | :--- |
| **ShadowLink Setup 1.0.0.exe** | `d761e1bdccaa57187424602958087146522881223963456860000a45e78041c5` |

*(Verify checksum after download using `certutil -hashfile filename SHA256` in PowerShell)*

## 🛠️ Comparison vs Standard Proxy

| Feature | Standard SOCKS5 | **ShadowLink v1.0** |
| :--- | :--- | :--- |
| **Encryption** | None | **AES-256-GCM** |
| **Keys** | Static | **Ephemeral X25519** (Rotates per session) |
| **Scope** | App-specific | **System-Wide** or App-specific |

## ⚠️ Notes
- Requires **Windows 10/11 x64**.
- If SmartScreen warns you, choose "Run Anyway" (as this is a self-signed open-source build).

---

**Full Changelog**: https://github.com/TUSHAR91316/ShadowLink/compare/legacy...v1.0.0
