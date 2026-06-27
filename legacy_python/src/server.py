import asyncio
import logging
import urllib.request
from config import Config
from encryption import TunnelEncryption, ECDHKeyExchange
import sys
import os
import socket
import struct
import time
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from udp.server_udp import start_server_udp_relay

logging.basicConfig(level=Config.get_log_level(), format='%(asctime)s - [SERVER] - %(message)s')

# Constants
MAX_PACKET_SIZE = 65536  # 64KB max packet size to prevent DoS
READ_TIMEOUT = 30.0  # 30 second timeout for read operations
IP_CACHE_DURATION = 300  # Cache public IP for 5 minutes

class ShadowServer:
    def __init__(self, strict_mode=False, safe_isp_ip=None, event_callback=None):
        self.strict_mode = strict_mode
        self.safe_isp_ip = safe_isp_ip
        self.event_callback = event_callback
        self._cached_public_ip = None
        self._ip_check_time = 0
        self.fallback_mode = False  # Track if we're in fallback mode
        self.last_fallback_time = 0  # Prevent spam warnings

    def get_public_ip(self):
        """Get public IP with caching to avoid repeated lookups."""
        try:
            current_time = time.time()
            
            # Return cached IP if still valid
            if self._cached_public_ip and (current_time - self._ip_check_time) < IP_CACHE_DURATION:
                return self._cached_public_ip
            
            # Fetch fresh IP
            response = urllib.request.urlopen('https://api.ipify.org', timeout=3)
            ip = response.read().decode('utf8').strip()
            
            # Cache the result
            self._cached_public_ip = ip
            self._ip_check_time = current_time
            return ip
        except urllib.error.URLError as e:
            logging.warning(f"Failed to get public IP (network error): {e}")
            return None
        except Exception as e:
            logging.warning(f"Failed to get public IP: {e}")
            return None

    def check_safety(self):
        """Check if current IP matches safe ISP IP (strict mode)."""
        if not self.strict_mode:
            return True
        
        current_ip = self.get_public_ip()
        if not current_ip:
            logging.warning("Could not determine Public IP. Blocking traffic in Strict Mode.")
            return False
            
        if current_ip == self.safe_isp_ip:
            # VPN is down - trigger fallback mode
            self._trigger_fallback(current_ip)
            return False
        
        # If we were in fallback mode but VPN is back up, notify user
        if self.fallback_mode:
            self._notify_vpn_restored()
        
        return True

    def _trigger_fallback(self, current_ip):
        """Trigger fallback to normal network when VPN fails."""
        current_time = time.time()
        
        # Prevent spam warnings (only warn once per minute)
        if current_time - self.last_fallback_time < 60:
            return
            
        self.last_fallback_time = current_time
        self.fallback_mode = True
        
        logging.critical(f"SECURITY ALERT: VPN DOWN! Current IP ({current_ip}) matches ISP IP. Switching to fallback mode.")
        
        # Send warning to UI
        if self.event_callback:
            self.event_callback("warning", {
                "type": "vpn_failure",
                "message": f"VPN connection lost! Traffic is now using normal network (IP: {current_ip}).",
                "current_ip": current_ip,
                "fallback_active": True
            })
            
            # Trigger system proxy disable for fallback
            self.event_callback("fallback", {
                "action": "disable_proxy",
                "reason": "vpn_down",
                "message": "Disabling system proxy to allow direct internet access"
            })

    def _notify_vpn_restored(self):
        """Notify when VPN connection is restored."""
        self.fallback_mode = False
        logging.info("VPN connection restored - exiting fallback mode")
        
        if self.event_callback:
            self.event_callback("info", {
                "type": "vpn_restored", 
                "message": "VPN connection restored! Secure tunnel is active again.",
                "fallback_active": False
            })

    async def _read_with_timeout(self, reader, n_bytes):
        """Read n bytes with timeout protection."""
        try:
            return await asyncio.wait_for(
                reader.readexactly(n_bytes),
                timeout=READ_TIMEOUT
            )
        except asyncio.TimeoutError:
            logging.warning(f"Timeout waiting for {n_bytes} bytes")
            raise ConnectionError("Read timeout")
        except asyncio.IncompleteReadError as e:
            logging.debug("Connection closed unexpectedly during read")
            raise

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
            try:
                req_line = await asyncio.wait_for(
                    reader.readuntil(b'\r\n\r\n'),
                    timeout=READ_TIMEOUT
                )
            except asyncio.TimeoutError:
                logging.warning(f"Timeout reading handshake from {addr}")
                writer.close()
                return
            
            req_str = req_line.decode('utf-8', errors='ignore')
            
            # Extract client pub key from fake headers
            client_pub_b64 = ""
            for line in req_str.split('\r\n'):
                if line.startswith("X-Auth-Key:"):
                    client_pub_b64 = line.split(":", 1)[1].strip()
                    break
                    
            if not client_pub_b64:
                logging.error(f"Invalid obfuscated handshake from {addr}")
                writer.close()
                return
                
            import base64
            try:
                client_pub_bytes = base64.b64decode(client_pub_b64)
            except Exception as e:
                logging.error(f"Failed to decode client public key from {addr}: {e}")
                writer.close()
                return

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
            
            # Read encrypted target info with size validation
            try:
                masked_len_bytes = await self._read_with_timeout(reader, 4)
            except (ConnectionError, asyncio.IncompleteReadError):
                writer.close()
                return
            
            raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
            enc_len = int.from_bytes(raw_len_bytes, 'big')
            
            # Validate packet size to prevent DoS
            if enc_len > MAX_PACKET_SIZE or enc_len <= 0:
                logging.error(f"Invalid packet size from {addr}: {enc_len}")
                writer.close()
                return
            
            try:
                encrypted_data = await self._read_with_timeout(reader, enc_len)
            except (ConnectionError, asyncio.IncompleteReadError):
                writer.close()
                return
            
            try:
                target_info_bytes = cipher.decrypt(encrypted_data)
                target_info = target_info_bytes.decode()
            except Exception as e:
                logging.error(f"Decryption failed from {addr}: {e}")
                writer.close()
                return
            
            if target_info == "UDP:ASSOCIATE":
                logging.info(f"Target is UDP Association for {addr}")
                
                # Start an ephemeral UDP output socket
                udp_transport, udp_protocol = await start_server_udp_relay(writer, cipher)
                
                # Confirm connection to client (Encrypted "OK")
                try:
                    encrypted_ok = cipher.encrypt(b"OK")
                    raw_len = len(encrypted_ok).to_bytes(4, 'big')
                    masked_len = bytes(b ^ 0x6A for b in raw_len)
                    writer.write(masked_len)
                    writer.write(encrypted_ok)
                    await writer.drain()
                except Exception as e:
                    logging.error(f"Failed to send OK response to {addr}: {e}")
                    udp_transport.close()
                    writer.close()
                    return
                
                # Loop to read incoming securely-tunneled UDP packets from the client over TCP
                try:
                    while True:
                        try:
                            masked_len_bytes = await self._read_with_timeout(reader, 4)
                        except (ConnectionError, asyncio.IncompleteReadError):
                            break
                        
                        raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
                        length = int.from_bytes(raw_len_bytes, 'big')
                        
                        # Validate packet size
                        if length > MAX_PACKET_SIZE or length <= 0:
                            logging.error(f"Invalid UDP packet size: {length}")
                            break
                        
                        try:
                            encrypted = await self._read_with_timeout(reader, length)
                            decrypted = cipher.decrypt(encrypted)
                        except (ConnectionError, asyncio.IncompleteReadError, Exception) as e:
                            logging.debug(f"Error reading UDP packet: {e}")
                            break
                        
                        # SOCKS5 UDP Request Format:
                        # +----+------+------+----------+----------+----------+
                        # |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
                        # +----+------+------+----------+----------+----------+
                        # | 2  |  1   |  1   | Variable |    2     | Variable |
                        # +----+------+------+----------+----------+----------+
                        
                        if len(decrypted) > 10:
                            try:
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
                            except (struct.error, IndexError) as e:
                                logging.debug(f"Failed to parse UDP packet: {e}")
                                continue
                            
                except asyncio.IncompleteReadError:
                    pass # Normal disconnect
                except Exception as e:
                    logging.error(f"Server UDP Relay Error: {e}")
                finally:
                    udp_transport.close()

            else:
                # Normal TCP Connect
                try:
                    remote_host, remote_port_str = target_info.rsplit(':', 1)
                    remote_port = int(remote_port_str)
                except (ValueError, AttributeError) as e:
                    logging.error(f"Invalid target info format: {e}")
                    writer.close()
                    return
                
                logging.info(f"Forwarding TCP to {remote_host}:{remote_port}")
                
                try:
                    remote_reader, remote_writer = await asyncio.wait_for(
                        asyncio.open_connection(remote_host, remote_port),
                        timeout=READ_TIMEOUT
                    )
                except asyncio.TimeoutError:
                    logging.error(f"Timeout connecting to {remote_host}:{remote_port}")
                    writer.close()
                    return
                except Exception as e:
                    logging.error(f"Failed to connect to TCP target {remote_host}:{remote_port}: {e}")
                    writer.close()
                    return

                # Confirm connection to client (Encrypted "OK")
                try:
                    encrypted_ok = cipher.encrypt(b"OK")
                    
                    # DPI BYPASS: Mask length
                    raw_len = len(encrypted_ok).to_bytes(4, 'big')
                    masked_len = bytes(b ^ 0x6A for b in raw_len)
                    
                    writer.write(masked_len)
                    writer.write(encrypted_ok)
                    await writer.drain()
                except Exception as e:
                    logging.error(f"Failed to send OK response: {e}")
                    remote_writer.close()
                    writer.close()
                    return

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
        """Decrypt data from source and write to dest."""
        try:
            while True:
                try:
                    # Use readexactly to fix TCP framing
                    masked_len_bytes = await asyncio.wait_for(
                        source.readexactly(4),
                        timeout=READ_TIMEOUT
                    )
                except asyncio.TimeoutError:
                    logging.warning("Timeout reading encrypted packet length")
                    break
                
                raw_len_bytes = bytes(b ^ 0x6A for b in masked_len_bytes)
                length = int.from_bytes(raw_len_bytes, 'big')
                
                # Validate packet size
                if length > MAX_PACKET_SIZE or length <= 0:
                    logging.error(f"Invalid packet size: {length}")
                    break
                
                try:
                    encrypted = await asyncio.wait_for(
                        source.readexactly(length),
                        timeout=READ_TIMEOUT
                    )
                except asyncio.TimeoutError:
                    logging.warning("Timeout reading encrypted packet data")
                    break
                
                try:
                    decrypted = cipher.decrypt(encrypted)
                    dest.write(decrypted)
                    await dest.drain()
                except Exception as e:
                    logging.error(f"Decryption error: {e}")
                    break
        except asyncio.IncompleteReadError:
            pass # Normal disconnect
        except Exception as e:
            logging.debug(f"Forward decrypt loop ended: {e}")
        finally:
            try: dest.close() 
            except: pass

    async def forward_encrypt(self, source, dest, cipher):
        """Encrypt data from source and write to dest."""
        try:
            while True:
                try:
                    data = await source.read(4096)
                    if not data: 
                        break
                except Exception as e:
                    logging.debug(f"Error reading data: {e}")
                    break
                
                try:
                    encrypted = cipher.encrypt(data)
                except Exception as e:
                    logging.error(f"Encryption failed: {e}")
                    break
                
                try:
                    # DPI BYPASS: Mask length
                    raw_len = len(encrypted).to_bytes(4, 'big')
                    masked_len = bytes(b ^ 0x6A for b in raw_len)
                    
                    dest.write(masked_len)
                    dest.write(encrypted)
                    await dest.drain()
                except Exception as e:
                    logging.debug(f"Error writing encrypted data: {e}")
                    break
        except Exception as e:
            logging.debug(f"Forward encrypt loop ended: {e}")

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
