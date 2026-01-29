import os
import logging

class Config:
    # Ports
    SERVER_PORT = 8443
    CLIENT_PORT = 1080
    
    # Handshake Protocol Constants
    HANDSHAKE_SIZE = 4096 # Allow enough buffer for PEM keys
    
    # Paths
    BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    CONFIG_DIR = os.path.join(BASE_DIR, 'config')
    
    # Settings (defaults)
    STRICT_MODE = False
    ISP_IP_MARKER = None # Store user's ISP IP here to check against
    
    @staticmethod
    def get_log_level():
        return logging.INFO
