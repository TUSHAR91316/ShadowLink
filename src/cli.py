import threading
import sys
import time
from api import ShadowAPI

def event_callback(event_type, data):
    if event_type == "log":
        print(f"[LOG] {data.get('message')}")
    elif event_type == "status":
        print(f"\n*** Status changed to: {data.get('state')} ***\n> ", end="")
    elif event_type == "stats":
        pass # Optional: Print stats, but might clutter the CLI when typing
    else:
        print(f"[{event_type.upper()}] {data}")

def main():
    print("ShadowLink CLI started. Type 'help' for commands.")
    api = ShadowAPI(event_callback=event_callback)
    
    # Run the API stats processing loop in background
    bg_thread = threading.Thread(target=api.run, kwargs={"use_stdin": False}, daemon=True)
    bg_thread.start()
    
    # Small sleep to allow initial 'ready' status log to print before prompt
    time.sleep(0.5)

    while True:
        try:
            cmd = input("> ").strip().lower()
            if cmd == "start":
                api.start_services(strict=False, sysproxy_on=False)
            elif cmd == "start strict":
                api.start_services(strict=True, sysproxy_on=False)
            elif cmd == "start sysproxy":
                api.start_services(strict=False, sysproxy_on=True)
            elif cmd == "stop":
                api.stop_services()
            elif cmd in ("quit", "exit"):
                api.stop_services()
                break
            elif cmd == "help":
                print("Commands:\n  start          - Start standard ShadowLink tunnel\n  start strict   - Start with Kill Switch enabled\n  start sysproxy - Start with Windows System Proxy routing\n  stop           - Stop services\n  quit / exit    - Exit application")
            elif cmd == "":
                pass
            else:
                print("Unknown command. Type 'help' for available commands.")
        except (KeyboardInterrupt, EOFError):
            api.stop_services()
            break
            
    print("Exiting ShadowLink CLI...")
    sys.exit(0)

if __name__ == "__main__":
    main()
