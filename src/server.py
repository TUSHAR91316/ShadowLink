import asyncio
import logging
import urllib.request
from config import Config
from encryption import TunnelEncryption, ECDHKeyExchange
import sys
import os
import socket
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from udp.server_udp import start_server_udp_relay

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
            # --- DPI OBFUSCATION HANDSHAKE & ECDH ---
            
            # DPI BYPASS: Read Fake HTTP Request
            req_line = await reader.readuntil(b'\r\n\r\n')
            req_str = req_line.decode('utf-8')
            
            # Extract client pub key from fake headers
            client_pub_b64 = ""
            for line in req_str.split('\r\n'):
                if line.startswith("X-Auth-Key:"):
                    client_pub_b64 = line.split(":", 1)[1].strip()
                    break
                    
            if not client_pub_b64:
                logging.error("Invalid obfuscated handshake")
                writer.close()
                return
                
            import base64
            client_pub_bytes = base64.b64decode(client_pub_b64)

            # Generate our ephemeral key pair
            server_ecdh = ECDHKeyExchange()
            server_pub = server_ecdh.get_public_bytes()
            
            # DPI BYPASS: Send Fake HTTP Response containing our public key
            server_pub_b64 = base64.b64encode(server_pub).decode('utf-8')
            fake_resp = f"HTTP/1.1 200 OK\r\nServer: Microsoft-IIS/10.0\r\nX-Server-Key: {server_pub_b64}\r\n\r\n"
            writer.write(fake_resp.encode())
            await writer.drain()
            
            # Derive shared session key
            shared_key = server_ecdh.derive_shared_key(client_pub_bytes)
            cipher = TunnelEncryption(shared_key)
            logging.info(f"Secure WireGuard Tunnel Established with {addr} (Obfuscated)")

            # 3. Handle Encrypted Traffic
            # We expect the first message to be the Target Host info
            # Protocol: [Length 4][Encrypted Data]
            # Encrypted Data Decrypts to: "HOST:PORT"
            
            # Read encrypted target info
            masked_len_bytes = await reader.readexactly(4)
            raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
            enc_len = int.from_bytes(raw_len_bytes, 'big')
            
            encrypted_data = await reader.readexactly(enc_len)
            
            target_info_bytes = cipher.decrypt(encrypted_data)
            target_info = target_info_bytes.decode()
            
            if target_info == "UDP:ASSOCIATE":
                logging.info(f"Target is UDP Association for {addr}")
                
                # Start an ephemeral UDP output socket
                udp_transport, udp_protocol = await start_server_udp_relay(writer, cipher)
                
                # Confirm connection to client (Encrypted "OK")
                encrypted_ok = cipher.encrypt(b"OK")
                raw_len = len(encrypted_ok).to_bytes(4, 'big')
                masked_len = bytes(b ^ 0x6A for b in raw_len)
                writer.write(masked_len)
                writer.write(encrypted_ok)
                await writer.drain()
                
                # Loop to read incoming securely-tunneled UDP packets from the client over TCP
                try:
                    while True:
                        masked_len_bytes = await reader.readexactly(4)
                        raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
                        length = int.from_bytes(raw_len_bytes, 'big')
                        
                        encrypted = await reader.readexactly(length)
                        decrypted = cipher.decrypt(encrypted) # This is a SOCKS5 UDP payload
                        
                        # SOCKS5 UDP Request Format:
                        # +----+------+------+----------+----------+----------+
                        # |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
                        # +----+------+------+----------+----------+----------+
                        # | 2  |  1   |  1   | Variable |    2     | Variable |
                        # +----+------+------+----------+----------+----------+
                        
                        if len(decrypted) > 10:
                            frag = decrypted[2]
                            if frag != 0: continue # Fragmentation not supported yet
                            
                            atyp = decrypted[3]
                            data_idx = 4
                            
                            if atyp == 0x01: # IPv4
                                dst_addr = socket.inet_ntoa(decrypted[data_idx:data_idx+4])
                                data_idx += 4
                            elif atyp == 0x03: # Domain
                                domain_len = decrypted[data_idx]
                                data_idx += 1
                                dst_addr = decrypted[data_idx:data_idx+domain_len].decode('utf-8')
                                data_idx += domain_len
                            else: continue # IPv6 / error
                                
                            dst_port = int.from_bytes(decrypted[data_idx:data_idx+2], 'big')
                            data_idx += 2
                            
                            payload = decrypted[data_idx:]
                            
                            # Send the raw payload to the target on the internet!
                            udp_transport.sendto(payload, (dst_addr, dst_port))
                            
                except asyncio.IncompleteReadError:
                    pass # Normal disconnect
                except Exception as e:
                    logging.error(f"Server UDP Relay Error: {e}")
                finally:
                    udp_transport.close()

            else:
                # Normal TCP Connect
                remote_host, remote_port_str = target_info.split(':')
                remote_port = int(remote_port_str)
                
                logging.info(f"Forwarding TCP to {remote_host}:{remote_port}")
                
                try:
                    remote_reader, remote_writer = await asyncio.open_connection(remote_host, remote_port)
                except Exception as e:
                    logging.error(f"Failed to connect to TCP target: {e}")
                    writer.close()
                    return

                # Confirm connection to client (Encrypted "OK")
                encrypted_ok = cipher.encrypt(b"OK")
                
                # DPI BYPASS: Mask length
                raw_len = len(encrypted_ok).to_bytes(4, 'big')
                masked_len = bytes(b ^ 0x6A for b in raw_len)
                
                writer.write(masked_len)
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
                # Use readexactly to fix TCP framing
                masked_len_bytes = await source.readexactly(4)
                raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
                length = int.from_bytes(raw_len_bytes, 'big')
                
                encrypted = await source.readexactly(length)
                
                decrypted = cipher.decrypt(encrypted)
                dest.write(decrypted)
                await dest.drain()
        except asyncio.IncompleteReadError:
            pass # Normal disconnect
        except Exception as e:
            pass
        finally:
            try: dest.close() 
            except: pass

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
        except: pass

    async def start(self):
        self.server = await asyncio.start_server(
            self.handle_client, '0.0.0.0', Config.SERVER_PORT)
        
        logging.info(f"ShadowLink Server running on 0.0.0.0:{Config.SERVER_PORT}")
        logging.info(f"Strict Mode: {self.strict_mode}")
        
        async with self.server:
            await self.server.serve_forever()

    def stop(self):
        if hasattr(self, 'server') and self.server:
            self.server.close()
            logging.info("ShadowLink Server stopped.")

if __name__ == '__main__':
    # For testing, strictly relying on args would be better, but default is OFF
    server = ShadowServer()
    try:
        asyncio.run(server.start())
    except KeyboardInterrupt:
        pass
