import asyncio
import logging
import urllib.request
from config import Config
from encryption import TunnelEncryption, ECDHKeyExchange

logging.basicConfig(level=Config.get_log_level(), format='%(asctime)s - [SERVER] - %(message)s')

class ShadowServer:
    def __init__(self, strict_mode=False, safe_isp_ip=None):
        self.strict_mode = strict_mode
        self.safe_isp_ip = safe_isp_ip

    def get_public_ip(self):
        try:
            return urllib.request.urlopen('https://api.ipify.org', timeout=3).read().decode('utf8')
        except Exception:
            return None

    def check_safety(self):
        if not self.strict_mode:
            return True
        
        current_ip = self.get_public_ip()
        if not current_ip:
            logging.warning("Could not determine Public IP. Blocking traffic in Strict Mode.")
            return False
            
        if current_ip == self.safe_isp_ip:
            logging.critical(f"SECURITY ALERT: VPN DOWN! Current IP ({current_ip}) matches ISP IP. Blocking traffic.")
            return False
        
        return True

    async def handle_client(self, reader, writer):
        addr = writer.get_extra_info('peername')
        logging.info(f"New connection from {addr}")

        # 1. Kill Switch / Strict Mode Check
        if not self.check_safety():
            logging.error("Connection rejected due to Strict Mode violation.")
            writer.close()
            await writer.wait_closed()
            return

        try:
            # 2. Key Exchange (ECDH)
            # Generate our ephemeral key pair
            server_ecdh = ECDHKeyExchange()
            server_pub = server_ecdh.get_public_bytes()
            
            # Send our public key
            writer.write(len(server_pub).to_bytes(4, 'big'))
            writer.write(server_pub)
            await writer.drain()
            
            # Read client's public key
            len_bytes = await reader.read(4)
            if not len_bytes: return
            client_pub_len = int.from_bytes(len_bytes, 'big')
            client_pub_bytes = await reader.read(client_pub_len)
            
            # Derive shared session key (AES-256)
            shared_key = server_ecdh.derive_shared_key(client_pub_bytes)
            cipher = TunnelEncryption(shared_key)
            logging.info(f"Secure Tunnel Established with {addr} (AES-256)")

            # 3. Handle Encrypted Traffic
            # We expect the first message to be the Target Host info
            # Protocol: [Length 4][Encrypted Data]
            # Encrypted Data Decrypts to: "HOST:PORT"
            
            # Read encrypted target info
            len_bytes = await reader.read(4)
            if not len_bytes: return
            enc_len = int.from_bytes(len_bytes, 'big')
            encrypted_data = await reader.read(enc_len)
            
            target_info_bytes = cipher.decrypt(encrypted_data)
            target_info = target_info_bytes.decode()
            
            remote_host, remote_port_str = target_info.split(':')
            remote_port = int(remote_port_str)
            
            logging.info(f"Forwarding to {remote_host}:{remote_port}")
            
            try:
                remote_reader, remote_writer = await asyncio.open_connection(remote_host, remote_port)
            except Exception as e:
                logging.error(f"Failed to connect to target: {e}")
                # Send Encrypted Failure? Or just close.
                writer.close()
                return

            # Confirm connection to client (Encrypted "OK")
            encrypted_ok = cipher.encrypt(b"OK")
            writer.write(len(encrypted_ok).to_bytes(4, 'big'))
            writer.write(encrypted_ok)
            await writer.drain()

            # Pipe data
            await asyncio.gather(
                self.forward_decrypt(reader, remote_writer, cipher),
                self.forward_encrypt(remote_reader, writer, cipher)
            )

        except Exception as e:
            logging.error(f"Error handling client {addr}: {e}")
        finally:
            writer.close()

    async def forward_decrypt(self, source, dest, cipher):
        try:
            while True:
                len_bytes = await source.read(4)
                if not len_bytes: break
                length = int.from_bytes(len_bytes, 'big')
                encrypted = await source.read(length)
                if not encrypted: break
                
                decrypted = cipher.decrypt(encrypted)
                dest.write(decrypted)
                await dest.drain()
        except: pass
        finally:
            try: dest.close() 
            except: pass

    async def forward_encrypt(self, source, dest, cipher):
        try:
            while True:
                data = await source.read(4096)
                if not data: break
                
                encrypted = cipher.encrypt(data)
                dest.write(len(encrypted).to_bytes(4, 'big'))
                dest.write(encrypted)
                await dest.drain()
        except: pass

    async def start(self):
        server = await asyncio.start_server(
            self.handle_client, '0.0.0.0', Config.SERVER_PORT)
        
        logging.info(f"ShadowLink Server running on 0.0.0.0:{Config.SERVER_PORT}")
        logging.info(f"Strict Mode: {self.strict_mode}")
        
        async with server:
            await server.serve_forever()

if __name__ == '__main__':
    # For testing, strictly relying on args would be better, but default is OFF
    server = ShadowServer()
    try:
        asyncio.run(server.start())
    except KeyboardInterrupt:
        pass
