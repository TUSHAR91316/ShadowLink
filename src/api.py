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
        self._lock = threading.Lock()  # Protect shared state from race conditions
        self.stats_queue = Queue()
        self.server_thread = None
        self.client_thread = None
        self.loop_server = None
        self.loop_client = None
        self.event_callback = event_callback
        self.server_instance = None
        self.client_instance = None

    def send_event(self, event_type, data):
        """Send JSON event to Electron via stdout or direct callback"""
        if self.event_callback:
            self.event_callback(event_type, data)
        else:
            msg = json.dumps({"type": event_type, "data": data})
            sys.stdout.write(msg + "\n")
            sys.stdout.flush()

    def start_services(self, strict=False, sysproxy_on=False):
        """Start server and client services with thread safety."""
        with self._lock:
            if self.running:
                self.send_event("log", {"message": "Services already running"})
                return

            self.running = True

        self.send_event("status", {"state": "starting"})

        try:
            # Start Server Thread (non-daemon for clean shutdown)
            self.server_thread = threading.Thread(
                target=self.run_server, 
                args=(strict,), 
                daemon=False,
                name="ShadowLink-Server"
            )
            self.server_thread.start()

            # Start Client Thread (non-daemon for clean shutdown)
            self.client_thread = threading.Thread(
                target=self.run_client, 
                daemon=False,
                name="ShadowLink-Client"
            )
            self.client_thread.start()

            # System Proxy
            if sysproxy_on and SystemProxyManager:
                try:
                    if SystemProxyManager.set_system_proxy('127.0.0.1', Config.CLIENT_PORT, True):
                        self.send_event("log", {"message": "System-Wide Proxy ENABLED"})
                    else:
                        self.send_event("log", {"message": "ERROR: Could not set System Proxy"})
                except Exception as e:
                    logging.error(f"Failed to set system proxy: {e}")
                    self.send_event("log", {"message": f"System Proxy Error: {str(e)}"})

            self.send_event("status", {"state": "running"})
        except Exception as e:
            logging.error(f"Failed to start services: {e}")
            self.send_event("status", {"state": "error"})
            with self._lock:
                self.running = False
            raise

    def stop_services(self):
        """Stop services with thread safety."""
        with self._lock:
            if not self.running:
                return
            self.running = False

        self.send_event("status", {"state": "stopping"})
        
        try:
            # Disable System Proxy
            if SystemProxyManager:
                try:
                    SystemProxyManager.set_system_proxy('127.0.0.1', Config.CLIENT_PORT, False)
                    self.send_event("log", {"message": "System-Wide Proxy DISABLED"})
                except Exception as e:
                    logging.error(f"Failed to disable system proxy: {e}")

            # Stop server and client gracefully
            if self.server_instance:
                try:
                    self.server_instance.stop()
                    logging.info("Server instance stopped")
                except Exception as e:
                    logging.error(f"Error stopping server: {e}")

            if self.client_instance:
                try:
                    self.client_instance.stop()
                    logging.info("Client instance stopped")
                except Exception as e:
                    logging.error(f"Error stopping client: {e}")

            # Wait for threads to finish with timeout
            if self.server_thread and self.server_thread.is_alive():
                self.server_thread.join(timeout=2)
                if self.server_thread.is_alive():
                    logging.warning("Server thread did not stop gracefully")

            if self.client_thread and self.client_thread.is_alive():
                self.client_thread.join(timeout=2)
                if self.client_thread.is_alive():
                    logging.warning("Client thread did not stop gracefully")

            # Close event loops safely
            if self.loop_server and not self.loop_server.is_closed():
                try:
                    self.loop_server.call_soon_threadsafe(self.loop_server.stop)
                except Exception as e:
                    logging.error(f"Error stopping server loop: {e}")

            if self.loop_client and not self.loop_client.is_closed():
                try:
                    self.loop_client.call_soon_threadsafe(self.loop_client.stop)
                except Exception as e:
                    logging.error(f"Error stopping client loop: {e}")

        except Exception as e:
            logging.error(f"Error during stop_services: {e}")
        finally:
            self.send_event("status", {"state": "stopped"})

    def run_server(self, strict):
        """Run the server in a dedicated thread with proper event loop."""
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        self.loop_server = loop
        
        try:
            self.server_instance = ShadowServer(strict_mode=strict, safe_isp_ip=Config.ISP_IP_MARKER)
            self.send_event("log", {"message": f"Server initialized (Strict Mode: {strict})"})
            logging.info(f"Starting server (strict_mode={strict})")
            loop.run_until_complete(self.server_instance.start())
        except asyncio.CancelledError:
            logging.info("Server task cancelled")
        except Exception as e:
            logging.error(f"Server error: {e}")
            self.send_event("log", {"message": f"Server Error: {str(e)}"})
            self.send_event("status", {"state": "error"})
            with self._lock:
                self.running = False
        finally:
            try:
                loop.close()
            except Exception as e:
                logging.error(f"Error closing server loop: {e}")

    def run_client(self):
        """Run the client in a dedicated thread with proper event loop."""
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        self.loop_client = loop
        
        try:
            self.client_instance = ShadowClient(stats_queue=self.stats_queue)
            self.send_event("log", {"message": "Client Proxy initialized"})
            logging.info("Starting client proxy")
            loop.run_until_complete(self.client_instance.start())
        except asyncio.CancelledError:
            logging.info("Client task cancelled")
        except Exception as e:
            logging.error(f"Client error: {e}")
            self.send_event("log", {"message": f"Client Error: {str(e)}"})
            self.send_event("status", {"state": "error"})
            with self._lock:
                self.running = False
        finally:
            try:
                loop.close()
            except Exception as e:
                logging.error(f"Error closing client loop: {e}")

    def process_stats(self):
        """Check stats queue and send events."""
        while not self.stats_queue.empty():
            try:
                stat = self.stats_queue.get_nowait()
                self.send_event("stats", stat)
            except Empty:
                break

    def run(self, use_stdin=True):
        """Main event loop."""
        self.send_event("status", {"state": "ready"})
        
        # Main loop: Read stdin, process commands, flush stats
        if use_stdin:
            stdin_thread = threading.Thread(target=self.input_loop, daemon=True, name="ShadowLink-Input")
            stdin_thread.start()
            
        try:
            while True:
                self.process_stats()
                time.sleep(0.5) # Update rate
        except KeyboardInterrupt:
            logging.info("Received interrupt signal")
            self.stop_services()
        except Exception as e:
            logging.error(f"Error in main loop: {e}")
            self.stop_services()

    def input_loop(self):
        """Read commands from stdin."""
        try:
            for line in sys.stdin:
                if not line: 
                    break
                try:
                    cmd = json.loads(line)
                    self.handle_command(cmd)
                except json.JSONDecodeError:
                    logging.error("Invalid JSON received")
                except Exception as e:
                    logging.error(f"Command error: {e}")
        except EOFError:
            logging.info("stdin closed")
        except Exception as e:
            logging.error(f"Error in input loop: {e}")

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
