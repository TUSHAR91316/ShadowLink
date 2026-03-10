import asyncio
import logging
import socket

class ClientUDPProtocol(asyncio.DatagramProtocol):
    """
    Listens on a local UDP port for SOCKS5 UDP packets from the application.
    Forwards payload to the Server over the encrypted TCP tunnel.
    """
    def __init__(self, tcp_writer, cipher):
        self.tcp_writer = tcp_writer
        self.cipher = cipher
        self.transport = None
        self.client_addr = None # Address of the local application (e.g. browser)

    def connection_made(self, transport):
        self.transport = transport

    def datagram_received(self, data, addr):
        # We only accept packets from the app that initiated the SOCKS Associate
        # SOCKS5 UDP Request Format:
        # +----+------+------+----------+----------+----------+
        # |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
        # +----+------+------+----------+----------+----------+
        # | 2  |  1   |  1   | Variable |    2     | Variable |
        # +----+------+------+----------+----------+----------+
        
        self.client_addr = addr # Remember client to send replies back
        
        try:
            # We encrypt the entire raw SOCKS UDP packet and tunnel it via TCP
            encrypted = self.cipher.encrypt(data)
            
            # DPI BYPASS: Mask length
            raw_len = len(encrypted).to_bytes(4, 'big')
            masked_len = bytes(b ^ 0x6A for b in raw_len)
            
            self.tcp_writer.write(masked_len)
            self.tcp_writer.write(encrypted)
            
            # Fire and forget the drain so we don't block the synchronous datagram_received method
            asyncio.create_task(self.tcp_writer.drain())
        except Exception as e:
            logging.error(f"UDP Client Error: {e}")

    def send_to_app(self, data):
        """Called when a packet is received from the server TCP tunnel"""
        if self.transport and self.client_addr:
            # Data should still have the SOCKS5 UDP headers which the app expects
            self.transport.sendto(data, self.client_addr)

async def start_udp_listener(tcp_writer, cipher):
    """Spawns an ephemeral UDP listener and returns its port"""
    loop = asyncio.get_running_loop()
    transport, protocol = await loop.create_datagram_endpoint(
        lambda: ClientUDPProtocol(tcp_writer, cipher),
        local_addr=('127.0.0.1', 0) # Random available port
    )
    port = transport.get_extra_info('sockname')[1]
    return transport, protocol, port
