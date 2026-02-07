# ShadowLink - Secure Local Encrypted Tunnel

ShadowLink is a next-generation local VPN and secure tunnel application that uses **AES-256-GCM** and **X25519** key exchange to encrypt traffic. It features a modern, animated user interface built with **Electron** and **React**.

![ShadowLink UI](https://via.placeholder.com/800x600.png?text=ShadowLink+UI+Preview)

## 🚀 Features

- **Double Encryption**: Traffic is encrypted locally using ephemeral keys before leaving your device.
- **Strict Mode (Kill Switch)**: Automatically cuts traffic if the secure tunnel drops.
- **System-Wide Proxy**: Routes all system traffic through the secure tunnel with one click.
- **Modern UI**: Polished, cyber-aesthetic interface with real-time stats and animations.
- **Cross-Platform**: Built on Electron (Frontend) and Python (Backend).

## 🛠 Architecture

ShadowLink uses a hybrid architecture:
- **Frontend**: Electron, React, TailwindCSS, Framer Motion.
- **Backend**: Python (compiled to single-file executable), handling specific encryption and socket operations.
- **IPC**: The frontend communicates with the backend via standard input/output (stdio) using JSON-RPC.

## 📦 Installation & Build

### Prerequisites
- Node.js (v18+)
- Python 3.10+
- `pip` packages: `cryptography`

### 1. Setup
```bash
# Install Python dependencies
pip install -r requirements.txt

# Install Node dependencies
cd electron
npm install
```

### 2. Development Mode
Run the app locally with hot-reloading:
```bash
# In 'electron/' directory
npm run dev
```

### 3. Build & Package
To create the Windows installer (`.exe`):
```bash
# In 'electron/' directory
npm run build:css   # Generate Tailwind styles
npm run build:win   # Package app
```
The installer will be in `electron/dist_installer/`.

## 🔐 Verification
To verify the encryption implementation (X25519 + AES-256-GCM), run the included verification script:

```bash
python src/verify_encryption.py
```
This script simulates a full handshake and encryption cycle, printing the keys and ciphertext to the console for inspection.

## 📄 License
MIT
