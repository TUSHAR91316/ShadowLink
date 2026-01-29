import winreg
import ctypes
import logging

class SystemProxyManager:
    """
    Manages Windows System Proxy settings via the Registry.
    Target Key: HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings
    """
    
    INTERNET_SETTINGS = winreg.OpenKey(winreg.HKEY_CURRENT_USER,
        r'Software\Microsoft\Windows\CurrentVersion\Internet Settings',
        0, winreg.KEY_ALL_ACCESS)

    @staticmethod
    def set_system_proxy(host, port, enabled=True):
        """
        Enables or disables the system-wide proxy.
        Note: This affects the current user's settings immediately.
        """
        try:
            key = SystemProxyManager.INTERNET_SETTINGS
            
            if enabled:
                proxy_server = f"socks={host}:{port}"
                # Enable Proxy
                winreg.SetValueEx(key, 'ProxyEnable', 0, winreg.REG_DWORD, 1)
                # Set Proxy Server
                winreg.SetValueEx(key, 'ProxyServer', 0, winreg.REG_SZ, proxy_server)
                # Optional: Bypass local
                winreg.SetValueEx(key, 'ProxyOverride', 0, winreg.REG_SZ, '<local>')
                logging.info(f"System Proxy ENABLED: {proxy_server}")
            else:
                # Disable Proxy
                winreg.SetValueEx(key, 'ProxyEnable', 0, winreg.REG_DWORD, 0)
                logging.info("System Proxy DISABLED")
                
            # Force system refresh
            SystemProxyManager._refresh_settings()
            return True
        except Exception as e:
            logging.error(f"Failed to set system proxy: {e}")
            return False

    @staticmethod
    def _refresh_settings():
        """
        Notifies the system that internet settings have changed.
        Equivalent to hitting liability 'OK' in LAN settings.
        """
        INTERNET_OPTION_SETTINGS_CHANGED = 39
        INTERNET_OPTION_REFRESH = 37
        
        internet_set_option = ctypes.windll.Wininet.InternetSetOptionW
        internet_set_option(0, INTERNET_OPTION_SETTINGS_CHANGED, 0, 0)
        internet_set_option(0, INTERNET_OPTION_REFRESH, 0, 0)
