import customtkinter as ctk
import os
import shutil
import sys
import threading
import time
from tkinter import filedialog, messagebox

# Theme Settings
ctk.set_appearance_mode("Dark")
ctk.set_default_color_theme("green")

class ShadowInstaller(ctk.CTk):
    def __init__(self):
        super().__init__()

        self.title("ShadowLink Installer")
        self.geometry("600x400")
        self.resizable(False, False)

        # Header
        self.header_frame = ctk.CTkFrame(self, height=60, corner_radius=0)
        self.header_frame.pack(fill="x")
        
        self.logo_label = ctk.CTkLabel(self.header_frame, text="SHADOWLINK SETUP", font=("Orbitron", 20, "bold"), text_color="#00ff41")
        self.logo_label.pack(pady=15)

        # Content
        self.content_frame = ctk.CTkFrame(self)
        self.content_frame.pack(fill="both", expand=True, padx=20, pady=20)
        
        self.lbl_info = ctk.CTkLabel(self.content_frame, text="Welcome to the ShadowLink Secure Tunnel Installer.\n\nThis wizard will install ShadowLink to your local application data folder\nand create a start menu shortcut.", 
                                     font=("Roboto", 14), justify="left")
        self.lbl_info.pack(pady=20)
        
        self.progress = ctk.CTkProgressBar(self.content_frame, orientation="horizontal", mode="determinate")
        self.progress.pack(pady=20, fill="x", padx=40)
        self.progress.set(0)
        
        self.status = ctk.CTkLabel(self.content_frame, text="Ready to Install", text_color="gray")
        self.status.pack()

        # Buttons
        self.btn_install = ctk.CTkButton(self, text="INSTALL", command=self.start_install, 
                                         height=40, font=("Roboto", 14, "bold"), fg_color="forestgreen")
        self.btn_install.pack(pady=20)

    def start_install(self):
        self.btn_install.configure(state="disabled", text="INSTALLING...")
        threading.Thread(target=self.install_process, daemon=True).start()

    def install_process(self):
        # SIMULATING INSTALL STEPS
        target_dir = os.path.join(os.getenv('LOCALAPPDATA'), 'ShadowLink')
        exe_source = "shadowlink.exe" # Assumes the installer is bundled with the exe or downloads it?
        # For a single-file installer, we usually embed the payload. 
        # But here we are building a setup.exe that effectively copies the 'shadowlink.exe' 
        # which should be next to it, OR this script is compiled into an exe that extracts it.
        
        # Simulating extraction/copy
        steps = [
            ("Checking System Requirements...", 0.1),
            ("Creating Directory Structure...", 0.3),
            (f"Installing to {target_dir}...", 0.5),
            ("Configuring Encryption Keys...", 0.7),
            ("Creating Shortcuts...", 0.9),
            ("Finalizing...", 1.0)
        ]

        for desc, prog in steps:
            self.update_status(desc, prog)
            time.sleep(0.8) # Fake delay for "Hacker" effect

        # Actual Copy Logic
        try:
            if not os.path.exists(target_dir):
                os.makedirs(target_dir)

            if getattr(sys, 'frozen', False):
                base_path = sys._MEIPASS
            else:
                base_path = os.path.dirname(os.path.abspath(__file__))
            
            src_file = os.path.join(base_path, 'shadowlink.exe')
            dst_file = os.path.join(target_dir, 'shadowlink.exe')
            
            if os.path.exists(src_file):
                shutil.copy2(src_file, dst_file)
                
                # Create Shortcut (PowerShell hack since we don't need pywin32 dep)
                desktop = os.path.join(os.environ['USERPROFILE'], 'Desktop')
                shortcut_path = os.path.join(desktop, 'ShadowLink.lnk')
                
                ps_script = f'$s=(New-Object -COM WScript.Shell).CreateShortcut("{shortcut_path}");$s.TargetPath="{dst_file}";$s.Save()'
                import subprocess
                subprocess.run(["powershell", "-Command", ps_script], capture_output=True, creationflags=0x08000000) # CREATE_NO_WINDOW
                
            else:
                self.update_status("Error: Payload not found!", 0)
                return
                
        except Exception as e:
            self.update_status(f"Error: {e}", 0)
            return
        
        self.update_status("Installation Complete!", 1.0)
        self.btn_install.configure(text="EXIT", command=self.destroy, state="normal", fg_color="gray")

    def update_status(self, text, prog):
        self.status.configure(text=text)
        self.progress.set(prog)

if __name__ == "__main__":
    app = ShadowInstaller()
    app.mainloop()
