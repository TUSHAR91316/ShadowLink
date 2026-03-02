import sys
import json
import threading
import asyncio
import time
import logging
from queue import Queue, Empty

# Add current directory to path if needed, though usually not if run from root
sys.path.append('src')

from config import Config
from client import ShadowClient
from server import ShadowServer
# Conditional import for sysproxy to avoid failures if not needed immediately
try:
    from sysproxy import SystemProxyManager
except ImportError:
    SystemProxyManager = None

# Configure logging to write to stderr so it doesn't corrupt stdout JSON
logging.basicConfig(stream=sys.stderr, level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

class ShadowAPI:
    def __init__(self, event_callback=None):
        self.running = False
        self.stats_queue = Queue()
        self.server_thread = None
        self.client_thread = None
        self.loop_server = None
        self.loop_client = None
        self.event_callback = event_callback

    def send_event(self, event_type, data):
        """Send JSON event to Electron via stdout or direct callback"""
        if self.event_callback:
            self.event_callback(event_type, data)
        else:
            msg = json.dumps({"type": event_type, "data": data})
            sys.stdout.write(msg + "\n")
            sys.stdout.flush()

    def start_services(self, strict=False, sysproxy_on=False):
        if self.running:
            self.send_event("log", {"message": "Services already running"})
            return

        self.running = True
        self.send_event("status", {"state": "starting"})

        # Start Server Thread
        self.server_thread = threading.Thread(target=self.run_server, args=(strict,), daemon=True)
        self.server_thread.start()

        # Start Client Thread
        self.client_thread = threading.Thread(target=self.run_client, daemon=True)
        self.client_thread.start()

        # System Proxy
        if sysproxy_on and SystemProxyManager:
            if SystemProxyManager.set_system_proxy('127.0.0.1', Config.CLIENT_PORT, True):
                self.send_event("log", {"message": "System-Wide Proxy ENABLED"})
            else:
                self.send_event("log", {"message": "ERROR: Could not set System Proxy"})

        self.send_event("status", {"state": "running"})

    def stop_services(self):
        if not self.running:
            return

        self.running = False
        self.send_event("status", {"state": "stopping"})
        
        # Disable System Proxy
        if SystemProxyManager:
            SystemProxyManager.set_system_proxy('127.0.0.1', Config.CLIENT_PORT, False)
            self.send_event("log", {"message": "System-Wide Proxy DISABLED"})

        # Threads are daemons, they will die when we exit or we can leave them 
        # hanging if we plan to restart. For API, usually we just restart the process 
        # or we need complex cancellation logic.
        # For now, we update state.
        
        self.send_event("status", {"state": "stopped"})

    def run_server(self, strict):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        self.loop_server = loop
        
        try:
            # Check port first or just try to start? 
            # asyncio start() usually binds port.
            server = ShadowServer(strict_mode=strict, safe_isp_ip=Config.ISP_IP_MARKER)
            self.send_event("log", {"message": f"Server Host initialized (Strict: {strict})"})
            loop.run_until_complete(server.start())
        except Exception as e:
            logging.error(f"Server error: {e}")
            self.send_event("log", {"message": f"Server Error: {str(e)}"})
            self.send_event("status", {"state": "stopped"})
            # We should probably stop the client too if server fails
            # But self.stop_services() might be tricky from a thread if it touches shared state poorly
            # For now, sending stopped status is enough to inform UI.

    def run_client(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        self.loop_client = loop
        
        client = ShadowClient(stats_queue=self.stats_queue)
        self.send_event("log", {"message": "Client Proxy initialized"})
        try:
            loop.run_until_complete(client.start())
        except Exception as e:
            logging.error(f"Client error: {e}")
            self.send_event("log", {"message": f"Client Error: {str(e)}"})
            self.send_event("status", {"state": "stopped"})

    def process_stats(self):
        """Check stats queue and send events"""
        while not self.stats_queue.empty():
            try:
                stat = self.stats_queue.get_nowait()
                self.send_event("stats", stat)
            except Empty:
                break

    def run(self, use_stdin=True):
        self.send_event("status", {"state": "ready"})
        
        # Main loop: Read stdin, process commands, flush stats
        if use_stdin:
            threading.Thread(target=self.input_loop, daemon=True).start()
            
        while True:
            self.process_stats()
            time.sleep(0.5) # Update rate

    def input_loop(self):
        for line in sys.stdin:
            if not line: break
            try:
                cmd = json.loads(line)
                self.handle_command(cmd)
            except json.JSONDecodeError:
                logging.error("Invalid JSON received")
            except Exception as e:
                logging.error(f"Command error: {e}")

    def handle_command(self, cmd):
        action = cmd.get("cmd")
        if action == "start":
            args = cmd.get("config", {})
            self.start_services(strict=args.get("strict", False), sysproxy_on=args.get("sysproxy", False))
        elif action == "stop":
            self.stop_services()
        elif action == "quit":
            self.stop_services()
            sys.exit(0)

if __name__ == "__main__":
    api = ShadowAPI()
    api.run()
