import asyncio
import socket
import logging
import time
from config import Config
from encryption import TunnelEncryption, ECDHKeyExchange

logging.basicConfig(level=Config.get_log_level(), format='%(asctime)s - [CLIENT] - %(message)s')

class ShadowClient:
    def __init__(self, server_host='127.0.0.1', server_port=Config.SERVER_PORT, stats_queue=None):
        self.server_host = server_host
        self.server_port = server_port
        self.stats_queue = stats_queue # Thread-safe queue for GUI updates
        self.bytes_sent = 0
        self.bytes_received = 0
        self.start_time = time.time()

    def update_stats(self, sent=0, received=0):
        self.bytes_sent += sent
        self.bytes_received += received
        if self.stats_queue:
            # Calculate speed? Or just push raw counters?
            # Let's push a dict
            self.stats_queue.put({
                'sent': self.bytes_sent,
                'recv': self.bytes_received,
                'speed_sent': sent, # Current chunk
                'speed_recv': received
            })

    async def handle_browser(self, reader, writer):
        try:
            # --- SOCKS5 HANDSHAKE ---
            # 1. Auth negotiation
            header = await reader.read(2)
            if not header or header[0] != 0x05: return
            nmethods = header[1]
            await reader.read(nmethods)
            writer.write(b'\x05\x00') # No Auth
            await writer.drain()

            # 2. Connection Request
            request_header = await reader.read(4)
            if not request_header: return
            ver, cmd, rsv, atyp = request_header
            if cmd != 0x01: return # Connect only

            if atyp == 0x01: # IPv4
                addr_bytes = await reader.read(4)
                dst_addr = socket.inet_ntoa(addr_bytes)
            elif atyp == 0x03: # Domain
                len_byte = await reader.read(1)
                domain_len = len_byte[0]
                dst_addr = (await reader.read(domain_len)).decode()
            else: return # IPv6 bad

            port_bytes = await reader.read(2)
            dst_port = int.from_bytes(port_bytes, 'big')

            logging.info(f"Connecting to {dst_addr}:{dst_port}")

            # --- SERVER CONNECTION & ECDH HANDSHAKE ---
            try:
                srv_reader, srv_writer = await asyncio.open_connection(self.server_host, self.server_port)
            except Exception as e:
                logging.error(f"Server refused: {e}")
                writer.close()
                return

            # Perform Key Exchange
            client_ecdh = ECDHKeyExchange()
            client_pub = client_ecdh.get_public_bytes()

            # 1. Read Server Pub Key
            len_bytes = await srv_reader.read(4)
            if not len_bytes: return
            server_pub_len = int.from_bytes(len_bytes, 'big')
            server_pub_bytes = await srv_reader.read(server_pub_len)

            # 2. Send Our Pub Key
            srv_writer.write(len(client_pub).to_bytes(4, 'big'))
            srv_writer.write(client_pub)
            await srv_writer.drain()

            # 3. Derive Secret
            shared_key = client_ecdh.derive_shared_key(server_pub_bytes)
            cipher = TunnelEncryption(shared_key)
            logging.info("Encrypted Tunnel Established")

            # --- REQUEST TUNNEL ---
            connect_msg = f"{dst_addr}:{dst_port}".encode()
            encrypted_connect = cipher.encrypt(connect_msg)
            
            srv_writer.write(len(encrypted_connect).to_bytes(4, 'big'))
            srv_writer.write(encrypted_connect)
            await srv_writer.drain()

            # Wait for OK
            len_bytes = await srv_reader.read(4)
            if not len_bytes: return
            enc_len = int.from_bytes(len_bytes, 'big')
            encrypted_resp = await srv_reader.read(enc_len)
            
            try:
                decrypted_resp = cipher.decrypt(encrypted_resp)
                if decrypted_resp != b"OK": raise Exception("Refused")
            except:
                writer.close()
                srv_writer.close()
                return

            # Reply to Browser (Success)
            writer.write(b'\x05\x00\x00\x01' + socket.inet_aton('0.0.0.0') + (0).to_bytes(2, 'big'))
            await writer.drain()

            # --- PIPE DATA ---
            await asyncio.gather(
                self.forward_encrypt(reader, srv_writer, cipher),
                self.forward_decrypt(srv_reader, writer, cipher)
            )

        except Exception as e:
            logging.error(f"Client Error: {e}")
        finally:
            writer.close()

    async def forward_encrypt(self, source, dest, cipher):
        try:
            while True:
                data = await source.read(4096)
                if not data: break
                
                encrypted = cipher.encrypt(data)
                dest.write(len(encrypted).to_bytes(4, 'big'))
                dest.write(encrypted)
                await dest.drain()
                self.update_stats(sent=len(data))
        except: pass

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
                self.update_stats(received=len(decrypted))
        except: pass

    async def start(self):
        server = await asyncio.start_server(
            self.handle_browser, '127.0.0.1', Config.CLIENT_PORT)
        logging.info(f"SOCKS5 Proxy on localhost:{Config.CLIENT_PORT}")
        async with server:
            await server.serve_forever()

if __name__ == '__main__':
    client = ShadowClient()
    try:
        asyncio.run(client.start())
    except KeyboardInterrupt:
        pass
