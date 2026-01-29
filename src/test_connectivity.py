import asyncio
import aiohttp
from src.config import Config

async def test_proxy():
    proxy_url = f"http://127.0.0.1:{Config.CLIENT_PORT}"
    print(f"Testing connection through ShadowLink Client at {proxy_url}...")
    
    try:
        async with aiohttp.ClientSession() as session:
            # We use the SOCKS5 proxy to connect to a public IP echo service
            # Note: aiohttp requires 'aiohttp-socks' for socks proxy support usually,
            # but standard SOCKS5 support might need to be explicitly handled or we use 'requests' with pysocks.
            # To avoid adding more dependencies just for this test, let's try using a simple socket check 
            # or IF we can, use 'requests' if the user has it, but safe bet is just a basic handshake check.
            
            # Actually, let's just make a simple TCP request to the local client port 
            # and Send a SOCKS handshake to see if it responds.
            reader, writer = await asyncio.open_connection('127.0.0.1', Config.CLIENT_PORT)
            
            # 1. Send SOCKS5 Greeting
            writer.write(b'\x05\x01\x00') # VER 5, 1 Method, No Auth
            await writer.drain()
            
            # 2. Read Response
            data = await reader.read(2)
            if data == b'\x05\x00':
                print("[PASS] Client accepted SOCKS5 handshake (No Auth).")
            else:
                print(f"[FAIL] Client rejected handshake: {data}")
                return

            print("Test Complete. The Client is listening and speaking SOCKS5.")
            writer.close()
            await writer.wait_closed()
            
    except Exception as e:
        print(f"[FAIL] Connection error: {e}")

if __name__ == "__main__":
    asyncio.run(test_proxy())
