import asyncio
import logging
import socket

class ServerUDPProtocol(asyncio.DatagramProtocol):
    """
    Listens on an ephemeral outgoing UDP port.
    When a UDP packet arrives from the Internet, it wraps it in a SOCKS5 header,
    encrypts it, and sends it back down the TCP tunnel to the client.
    """
    def __init__(self, tcp_writer, cipher):
        self.tcp_writer = tcp_writer
        self.cipher = cipher
        self.transport = None

    def connection_made(self, transport):
        self.transport = transport

    def datagram_received(self, data, addr):
        # We got a reply from the internet (e.g. DNS server or Game Server)
        # We need to wrap it in a SOCKS5 UDP header so the local app knows where it came from
        
        host, port = addr
        
        try:
            # SOCKS5 Header for IPV4
            # +----+------+------+----------+----------+----------+
            # |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
            # +----+------+------+----------+----------+----------+
            header = b'\x00\x00\x00\x01' + socket.inet_aton(host) + port.to_bytes(2, 'big')
            socks_packet = header + data
            
            encrypted = self.cipher.encrypt(socks_packet)
            
            # DPI BYPASS
            raw_len = len(encrypted).to_bytes(4, 'big')
            masked_len = bytes(b ^ 0x6A for b in raw_len)
            
            self.tcp_writer.write(masked_len)
            self.tcp_writer.write(encrypted)
            
            # Fire and forget the drain so we don't block the synchronous datagram_received method
            asyncio.create_task(self.tcp_writer.drain())
        except Exception as e:
            logging.error(f"UDP Server Reply Error: {e}")

async def start_server_udp_relay(tcp_writer, cipher):
    """Starts the Datagram Protocol to relay packets to the internet"""
    loop = asyncio.get_running_loop()
    transport, protocol = await loop.create_datagram_endpoint(
        lambda: ServerUDPProtocol(tcp_writer, cipher),
        local_addr=('0.0.0.0', 0)
    )
    return transport, protocol
