import asyncio
import socket
import logging
import time
from config import Config
from encryption import TunnelEncryption, ECDHKeyExchange
import sys
import os
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from udp.proxy_udp import start_udp_listener

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
            # --- PROTOCOL NEGOTIATION ---
            header = await reader.read(2)
            if not header: return

            # Initialize so all code paths have these defined
            cmd = 0x01  # Default to CONNECT
            is_socks = False

            # **HTTP Proxy Fallback (For browsers that ignore SOCKS settings)**
            if header.startswith(b'CO') or header.startswith(b'GE') or header.startswith(b'PO'):
                # Read the rest of the HTTP headers to find the Host
                request_line_rest = await reader.readuntil(b'\r\n')
                full_req_line = header + request_line_rest
                
                parts = full_req_line.decode().split(' ')
                if len(parts) >= 2:
                    url = parts[1]
                    if url.startswith('http'):
                        from urllib.parse import urlparse
                        parsedUrl = urlparse(url)
                        dst_addr = parsedUrl.hostname
                        dst_port = parsedUrl.port or (443 if parsedUrl.scheme == 'https' else 80)
                    else:
                        # CONNECT format (host:port)
                        host_port = url.split(':')
                        dst_addr = host_port[0]
                        dst_port = int(host_port[1]) if len(host_port) > 1 else 443
                        
                    # Consume remaining headers
                    while True:
                        line = await reader.readuntil(b'\r\n')
                        if line == b'\r\n': break
                        
                    # HTTP CONNECT Success Response
                    writer.write(b"HTTP/1.1 200 Connection Established\r\n\r\n")
                    await writer.drain()
                    cmd = 0x01  # HTTP proxies only handle CONNECT (TCP)
                else: return

            # **SOCKS5 Protocol**
            elif header[0] == 0x05:
                nmethods = header[1]
                await reader.read(nmethods)
                writer.write(b'\x05\x00') # No Auth
                await writer.drain()

                # 2. Connection Request
                request_header = await reader.read(4)
                if not request_header: return
                ver, cmd, rsv, atyp = request_header
                
                if cmd not in (0x01, 0x03): 
                    return # Connect (0x01) or UDP Associate (0x03) only

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
                
                # Flag for replying via SOCKS protocol
                is_socks = True
            else:
                return # Unsupported protocol

            logging.info(f"Connecting to {dst_addr}:{dst_port}")

            # --- SERVER CONNECTION & DPI OBFUSCATION HANDSHAKE ---
            try:
                srv_reader, srv_writer = await asyncio.open_connection(self.server_host, self.server_port)
            except Exception as e:
                logging.error(f"Server refused: {e}")
                writer.close()
                return

            # Perform Key Exchange
            client_ecdh = ECDHKeyExchange()
            client_pub = client_ecdh.get_public_bytes()

            # DPI BYPASS: Send Fake HTTP Request containing our public key
            import base64
            pub_b64 = base64.b64encode(client_pub).decode('utf-8')
            fake_req = f"GET / HTTP/1.1\r\nHost: www.microsoft.com\r\nUser-Agent: Mozilla/5.0\r\nX-Auth-Key: {pub_b64}\r\n\r\n"
            srv_writer.write(fake_req.encode())
            await srv_writer.drain()

            # DPI BYPASS: Read Fake HTTP Response containing server's public key
            resp_line = await srv_reader.readuntil(b'\r\n\r\n')
            resp_str = resp_line.decode('utf-8')
            
            # Extract server pub key
            server_pub_b64 = ""
            for line in resp_str.split('\r\n'):
                if line.startswith("X-Server-Key:"):
                    server_pub_b64 = line.split(":", 1)[1].strip()
                    break
            
            if not server_pub_b64:
                raise Exception("Invalid handshake response")

            server_pub_bytes = base64.b64decode(server_pub_b64)

            # 3. Derive Secret
            shared_key = client_ecdh.derive_shared_key(server_pub_bytes)
            cipher = TunnelEncryption(shared_key)
            logging.info("Encrypted WireGuard Tunnel Established (Obfuscated)")

            # --- REQUEST TUNNEL ---
            if cmd == 0x01:
                connect_msg = f"{dst_addr}:{dst_port}".encode()
            elif cmd == 0x03:
                connect_msg = b"UDP:ASSOCIATE"
                
            encrypted_connect = cipher.encrypt(connect_msg)
            
            # DPI BYPASS: Mask the length prefix
            raw_len = len(encrypted_connect).to_bytes(4, 'big')
            masked_len = bytes(b ^ 0x6A for b in raw_len) # Simple XOR mask
            
            srv_writer.write(masked_len)
            srv_writer.write(encrypted_connect)
            await srv_writer.drain()

            # Wait for OK
            masked_len_bytes = await srv_reader.readexactly(4)
            raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
            enc_len = int.from_bytes(raw_len_bytes, 'big')
            
            encrypted_resp = await srv_reader.readexactly(enc_len)
            
            try:
                decrypted_resp = cipher.decrypt(encrypted_resp)
                if decrypted_resp != b"OK": raise Exception("Refused")
            except:
                writer.close()
                srv_writer.close()
                return

            # Reply to Browser (Success)
            if 'is_socks' in locals() and is_socks:
                if cmd == 0x01:
                    writer.write(b'\x05\x00\x00\x01' + socket.inet_aton('0.0.0.0') + (0).to_bytes(2, 'big'))
                    await writer.drain()
                elif cmd == 0x03:
                    # UDP Associate: We must open a UDP socket and tell the client where to send packets
                    udp_transport, udp_protocol, udp_port = await start_udp_listener(srv_writer, cipher)
                    
                    # BND.ADDR and BND.PORT (Local IP and the ephemeral UDP listening port)
                    writer.write(b'\x05\x00\x00\x01' + socket.inet_aton('127.0.0.1') + udp_port.to_bytes(2, 'big'))
                    await writer.drain()

            # --- PIPE DATA ---
            if cmd == 0x01:
                await asyncio.gather(
                    self.forward_encrypt(reader, srv_writer, cipher),
                    self.forward_decrypt(srv_reader, writer, cipher)
                )
            elif cmd == 0x03:
                # For UDP, the client sends datagrams directly to the UDP port. 
                # This TCP connection must be held open to keep the association alive.
                # We also need to read server replies (UDP packets coming back) and push them to the UDP protocol.
                await asyncio.gather(
                    self.forward_decrypt_udp(srv_reader, udp_protocol, cipher),
                    self.hold_tcp_alive(reader)
                )

        except Exception as e:
            logging.error(f"Client Error: {e}")
        finally:
            writer.close()
            if 'udp_transport' in locals():
                udp_transport.close()

    async def forward_encrypt(self, source, dest, cipher):
        try:
            while True:
                data = await source.read(4096)
                if not data: break
                
                encrypted = cipher.encrypt(data)
                
                # DPI BYPASS: Mask length
                raw_len = len(encrypted).to_bytes(4, 'big')
                masked_len = bytes(b ^ 0x6A for b in raw_len)
                
                dest.write(masked_len)
                dest.write(encrypted)
                await dest.drain()
                self.update_stats(sent=len(data))
        except: pass

    async def forward_decrypt(self, source, dest, cipher):
        try:
            while True:
                # Use readexactly to fix TCP framing
                masked_len_bytes = await source.readexactly(4)
                raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
                length = int.from_bytes(raw_len_bytes, 'big')
                
                encrypted = await source.readexactly(length)
                
                decrypted = cipher.decrypt(encrypted)
                dest.write(decrypted)
                await dest.drain()
                self.update_stats(received=len(decrypted))
        except asyncio.IncompleteReadError:
            pass # Normal disconnect
        except Exception as e:
            pass

    async def forward_decrypt_udp(self, source, udp_protocol, cipher):
        try:
            while True:
                masked_len_bytes = await source.readexactly(4)
                raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
                length = int.from_bytes(raw_len_bytes, 'big')
                
                encrypted = await source.readexactly(length)
                decrypted = cipher.decrypt(encrypted)
                
                # Send the decrypted SOCKS5 UDP payload back to the local app
                udp_protocol.send_to_app(decrypted)
                self.update_stats(received=len(decrypted))
        except Exception:
            pass

    async def hold_tcp_alive(self, reader):
        try:
            while True:
                data = await reader.read(4096)
                if not data: break # Client dropped association
        except: pass

    async def start(self):
        self.server = await asyncio.start_server(
            self.handle_browser, '127.0.0.1', Config.CLIENT_PORT)
        logging.info(f"SOCKS5 Proxy on localhost:{Config.CLIENT_PORT}")
        async with self.server:
            await self.server.serve_forever()

    def stop(self):
        if hasattr(self, 'server') and self.server:
            self.server.close()
            logging.info("SOCKS5 Proxy stopped.")

if __name__ == '__main__':
    client = ShadowClient()
    try:
        asyncio.run(client.start())
    except KeyboardInterrupt:
        pass
