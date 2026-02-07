import os
import sys
import binascii
from cryptography.hazmat.primitives import serialization

# Ensure we can import from src
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from encryption import ECDHKeyExchange, TunnelEncryption

def print_hex(label, data):
    hex_str = binascii.hexlify(data).decode('utf-8')
    # Chunk for readability
    chunked = ' '.join(hex_str[i:i+2] for i in range(0, len(hex_str), 2))
    print(f"\n[{label}] ({len(data)} bytes):")
    print(chunked)

def main():
    print("=== ShadowLink Encryption Verification ===")
    
    # 1. Simulate Alice and Bob (Client and Server)
    print("\n1. Generating Keys (ECDH - X25519)...")
    client_auth = ECDHKeyExchange()
    server_auth = ECDHKeyExchange()
    
    client_pub = client_auth.get_public_bytes()
    server_pub = server_auth.get_public_bytes()
    
    print_hex("Client Public Key", client_pub)
    print_hex("Server Public Key", server_pub)

    # 2. Derive Shared Secrets
    print("\n2. Deriving Shared AES-256 Keys...")
    client_shared = client_auth.derive_shared_key(server_pub)
    server_shared = server_auth.derive_shared_key(client_pub)
    
    print_hex("Client Derived Secret", client_shared)
    print_hex("Server Derived Secret", server_shared)
    
    if client_shared == server_shared:
        print("\n[SUCCESS] Keys Match! Secure Channel Established.")
    else:
        print("\n[ERROR] Keys Mismatch!")
        return

    # 3. Simulate Traffic
    cipher = TunnelEncryption(client_shared)
    
    plaintext = b"GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"
    print(f"\n3. Simulating Traffic...")
    print(f"Original Data: {plaintext}")
    
    # 4. Encrypt
    encrypted = cipher.encrypt(plaintext)
    print_hex("Encrypted Packet (Nonce + Ciphertext + Tag)", encrypted)
    
    # 5. Decrypt (Server side)
    server_cipher = TunnelEncryption(server_shared)
    decrypted = server_cipher.decrypt(encrypted)
    
    print(f"\n4. Decrypting at Destination...")
    print(f"Decrypted Data: {decrypted}")
    
    if decrypted == plaintext:
        print("\n[VERIFIED] Encryption/Decryption Cycle Successful.")
        print("Traffic is fully encrypted with AES-256-GCM.")
    else:
        print("\n[FAILED] Decrypted data does not match original.")

if __name__ == "__main__":
    main()
