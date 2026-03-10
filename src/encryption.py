import os
import struct
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import x25519
from cryptography.hazmat.primitives.kdf.hkdf import HKDF
from cryptography.hazmat.primitives.ciphers.aead import ChaCha20Poly1305

class ECDHKeyExchange:
    """Handles Elliptic Curve Diffie-Hellman Key Exchange to derive shared keys."""
    def __init__(self):
        # Generate ephemeral private key for this session using X25519
        self.private_key = x25519.X25519PrivateKey.generate()
        self.public_key = self.private_key.public_key()

    def get_public_bytes(self) -> bytes:
        """Returns the public key in bytes to send to the peer."""
        return self.public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo
        )

    def derive_shared_key(self, peer_public_bytes: bytes) -> bytes:
        """Derives a fast ephemeral key from the peer's public key."""
        peer_public_key = serialization.load_pem_public_key(peer_public_bytes)
        shared_secret = self.private_key.exchange(peer_public_key)
        
        # Derive a 32-byte (256-bit) key using HKDF (WireGuard parameters)
        derived_key = HKDF(
            algorithm=hashes.BLAKE2s(32) if hasattr(hashes, 'BLAKE2s') else hashes.SHA256(),
            length=32,
            salt=None,
            info=b'shadowlink-wg-handshake',
        ).derive(shared_secret)
        
        return derived_key

class TunnelEncryption:
    """ChaCha20-Poly1305 Encryption for the tunnel (WireGuard primitives)."""
    def __init__(self, key: bytes):
        if len(key) != 32:
            raise ValueError("Key must be 32 bytes (256 bits) for ChaCha20-Poly1305")
        self.chacha = ChaCha20Poly1305(key)
        # We use a strict counter for nonces instead of random to save 12 bytes per packet
        self.tx_counter = 0
        self.rx_counter = 0

    def encrypt(self, data: bytes) -> bytes:
        # Nonce is 12 bytes. We will use an 8-byte counter padded with 4 zero bytes
        nonce = struct.pack('<Q', self.tx_counter) + b'\x00\x00\x00\x00'
        self.tx_counter += 1
        
        ciphertext = self.chacha.encrypt(nonce, data, None)
        return ciphertext # No need to prepend nonce, it's synchronized

    def decrypt(self, data: bytes) -> bytes:
        nonce = struct.pack('<Q', self.rx_counter) + b'\x00\x00\x00\x00'
        self.rx_counter += 1
        return self.chacha.decrypt(nonce, data, None)
