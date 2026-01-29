import customtkinter as ctk
import threading
import asyncio
import queue
import time
from config import Config
from client import ShadowClient
from server import ShadowServer

# Theme Settings
ctk.set_appearance_mode("Dark")
ctk.set_default_color_theme("green")

class ShadowLinkApp(ctk.CTk):
    def __init__(self):
        super().__init__()

        self.title("ShadowLink - Secure Tunnel")
        self.geometry("800x600")
        self.resizable(False, False)
        
        # Data
        self.running = False
        self.stats_queue = queue.Queue()
        self.log_queue = queue.Queue()
        
        # Layout
        self.grid_columnconfigure(0, weight=1)
        self.grid_rowconfigure(1, weight=1)

        # Header
        self.header_frame = ctk.CTkFrame(self, height=80, corner_radius=0)
        self.header_frame.grid(row=0, column=0, sticky="ew")
        
        self.logo_label = ctk.CTkLabel(self.header_frame, text="SHADOWLINK", font=("Orbitron", 24, "bold"), text_color="#00ff41")
        self.logo_label.pack(side="left", padx=20, pady=20)
        
        self.status_indicator = ctk.CTkLabel(self.header_frame, text="OFFLINE", text_color="red", font=("Roboto", 14))
        self.status_indicator.pack(side="right", padx=20)

        # Main Content
        self.main_frame = ctk.CTkFrame(self)
        self.main_frame.grid(row=1, column=0, sticky="nsew", padx=20, pady=20)
        self.main_frame.grid_columnconfigure(0, weight=1)
        
        # Stats Area
        self.stats_frame = ctk.CTkFrame(self.main_frame)
        self.stats_frame.grid(row=0, column=0, sticky="ew", pady=(0, 20))
        
        self.lbl_speed_up = ctk.CTkLabel(self.stats_frame, text="UPLOAD: 0 KB/s", font=("Consolas", 14))
        self.lbl_speed_up.pack(side="left", padx=20, pady=10)
        
        self.lbl_speed_down = ctk.CTkLabel(self.stats_frame, text="DOWNLOAD: 0 KB/s", font=("Consolas", 14))
        self.lbl_speed_down.pack(side="right", padx=20, pady=10)

        # Controls
        self.controls_frame = ctk.CTkFrame(self.main_frame)
        self.controls_frame.grid(row=1, column=0, sticky="ew", pady=(0, 20))
        
        self.btn_connect = ctk.CTkButton(self.controls_frame, text="INITIALIZE LINK", command=self.toggle_connection, 
                                         height=50, font=("Roboto", 16, "bold"), fg_color="forestgreen")
        self.btn_connect.pack(fill="x", padx=20, pady=10)
        
        self.switch_strict = ctk.CTkSwitch(self.controls_frame, text="STRICT MODE (Kill Switch)")
        self.switch_strict.pack(pady=10)

        # Logs
        self.log_box = ctk.CTkTextbox(self.main_frame, font=("Consolas", 12), text_color="#00ff41", fg_color="black")
        self.log_box.grid(row=2, column=0, sticky="nsew")
        self.log("System Initialized.")

        # Polling
        self.after(100, self.update_ui)

    def log(self, msg):
        timestamp = time.strftime("[%H:%M:%S]")
        self.log_box.insert("end", f"{timestamp} {msg}\n")
        self.log_box.see("end")

    def toggle_connection(self):
        if not self.running:
            self.start_services()
        else:
            self.stop_services()

    def start_services(self):
        self.running = True
        self.btn_connect.configure(text="TERMINATE LINK", fg_color="darkred")
        self.status_indicator.configure(text="SECURE CONNECTION ACTIVE", text_color="#00ff41")
        self.log("Starting ShadowLink Services...")
        
        # Start Threads
        strict = self.switch_strict.get() == 1
        
        self.server_thread = threading.Thread(target=self.run_server, args=(strict,), daemon=True)
        self.client_thread = threading.Thread(target=self.run_client, daemon=True)
        
        self.server_thread.start()
        self.client_thread.start()

    def stop_services(self):
        # Graceful shutdown is hard with asyncio.run in threads without complex signaling
        # For this PoC, we rely on daemon threads dying when app closes or just "abandoning" them
        # Ideally, we'd use a stop event.
        self.running = False
        self.btn_connect.configure(text="INITIALIZE LINK", fg_color="forestgreen")
        self.status_indicator.configure(text="OFFLINE", text_color="red")
        self.log("Stopping Services (Restart App to fully reset threads)")
        # In a real app we would cancel async tasks here.

    def run_server(self, strict):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        
        # Hardcode ISP Safe IP for now or fetch it
        # For simplicity, if strict is on, we assume the user configured Config.ISP_IP manually or we fetch it first?
        # Let's verify strict mode logic.
        server = ShadowServer(strict_mode=strict, safe_isp_ip=Config.ISP_IP_MARKER)
        self.log(f"Server Host initialized (Strict: {strict})")
        loop.run_until_complete(server.start())

    def run_client(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        
        client = ShadowClient(stats_queue=self.stats_queue)
        self.log("Client Proxy initialized on :1080")
        loop.run_until_complete(client.start())

    def update_ui(self):
        # Process Stats
        while not self.stats_queue.empty():
            stat = self.stats_queue.get()
            # Smooth out speed display?
            # stat['speed_sent'] is chunk size. We need to aggregate per second.
            # Simplified: Just show total for now or "Activity"
            self.lbl_speed_up.configure(text=f"UPLOAD: {stat['sent']//1024} KB")
            self.lbl_speed_down.configure(text=f"DOWNLOAD: {stat['recv']//1024} KB")

        self.after(100, self.update_ui)

if __name__ == "__main__":
    app = ShadowLinkApp()
    app.mainloop()
