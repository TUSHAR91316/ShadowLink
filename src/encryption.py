import os
import secrets
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.kdf.hkdf import HKDF
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

class ECDHKeyExchange:
    """Handles Elliptic Curve Diffie-Hellman Key Exchange to derive shared AES keys."""
    def __init__(self):
        # Generate ephemeral private key for this session using Curve25519 (X25519)
        self.private_key = ec.generate_private_key(ec.X25519())
        self.public_key = self.private_key.public_key()

    def get_public_bytes(self) -> bytes:
        """Returns the public key in bytes to send to the peer."""
        return self.public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo
        )

    def derive_shared_key(self, peer_public_bytes: bytes) -> bytes:
        """Derives a fast ephemeral AES-256 key from the peer's public key."""
        peer_public_key = serialization.load_pem_public_key(peer_public_bytes)
        shared_secret = self.private_key.exchange(ec.ECDH(), peer_public_key)
        
        # Derive a 32-byte (256-bit) AES key using HKDF
        derived_key = HKDF(
            algorithm=hashes.SHA256(),
            length=32,
            salt=None,
            info=b'shadowlink-handshake',
        ).derive(shared_secret)
        
        return derived_key

class TunnelEncryption:
    """AES-256-GCM Encryption for the tunnel."""
    def __init__(self, key: bytes):
        if len(key) != 32:
            raise ValueError("Key must be 32 bytes (256 bits) for AES-256")
        self.aesgcm = AESGCM(key)

    def encrypt(self, data: bytes) -> bytes:
        # 12 bytes nonce (standard for GCM)
        nonce = os.urandom(12)
        ciphertext = self.aesgcm.encrypt(nonce, data, None)
        return nonce + ciphertext

    def decrypt(self, data: bytes) -> bytes:
        if len(data) < 12:
            raise ValueError("Data too short")
        nonce = data[:12]
        ciphertext = data[12:]
        return self.aesgcm.decrypt(nonce, ciphertext, None)
