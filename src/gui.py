import sys
import threading
from PyQt6.QtWidgets import (QApplication, QMainWindow, QWidget, QVBoxLayout, 
                             QHBoxLayout, QPushButton, QLabel, QTextEdit, 
                             QCheckBox, QGroupBox)
from PyQt6.QtCore import pyqtSignal, QObject, pyqtSlot
from PyQt6.QtGui import QFont

from api import ShadowAPI

class APIWorkerSignals(QObject):
    log_signal = pyqtSignal(str)
    status_signal = pyqtSignal(str)
    stats_signal = pyqtSignal(dict)

class ShadowLinkGUI(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("ShadowLink Native GUI")
        self.resize(550, 450)

        self.signals = APIWorkerSignals()
        self.signals.log_signal.connect(self.on_log)
        self.signals.status_signal.connect(self.on_status)
        self.signals.stats_signal.connect(self.on_stats)

        self.api = ShadowAPI(event_callback=self.api_callback)

        self.init_ui()

        # Start background API logic
        self.api_thread = threading.Thread(target=self.api.run, kwargs={"use_stdin": False}, daemon=True)
        self.api_thread.start()

    def init_ui(self):
        central_widget = QWidget()
        self.setCentralWidget(central_widget)
        main_layout = QVBoxLayout(central_widget)

        # Status Label
        self.status_label = QLabel("Status: Unknown")
        font = QFont()
        font.setPointSize(16)
        font.setBold(True)
        self.status_label.setFont(font)
        self.status_label.setStyleSheet("color: gray;")
        main_layout.addWidget(self.status_label)

        # Stats Group
        stats_group = QGroupBox("Live Connection Stats")
        stats_layout = QVBoxLayout()
        self.tx_label = QLabel("TX: 0 B/s")
        self.rx_label = QLabel("RX: 0 B/s")
        font_stats = QFont("Consolas", 11)
        self.tx_label.setFont(font_stats)
        self.rx_label.setFont(font_stats)
        stats_layout.addWidget(self.tx_label)
        stats_layout.addWidget(self.rx_label)
        stats_group.setLayout(stats_layout)
        main_layout.addWidget(stats_group)

        # Controls Group
        controls_group = QGroupBox("Controls")
        controls_layout = QHBoxLayout()
        
        self.strict_cb = QCheckBox("Strict Mode (Kill Switch)")
        self.sysproxy_cb = QCheckBox("System-Wide Proxy")
        
        self.start_btn = QPushButton("Start Tunnel")
        self.start_btn.clicked.connect(self.start_api)
        self.start_btn.setStyleSheet("background-color: #2e8b57; color: white; font-weight: bold; padding: 6px;")
        
        self.stop_btn = QPushButton("Stop Tunnel")
        self.stop_btn.clicked.connect(self.stop_api)
        self.stop_btn.setEnabled(False)
        self.stop_btn.setStyleSheet("background-color: #cd5c5c; color: white; font-weight: bold; padding: 6px;")

        controls_layout.addWidget(self.strict_cb)
        controls_layout.addWidget(self.sysproxy_cb)
        controls_layout.addWidget(self.start_btn)
        controls_layout.addWidget(self.stop_btn)
        controls_group.setLayout(controls_layout)
        main_layout.addWidget(controls_group)

        # Log Window
        log_group = QGroupBox("System Logs")
        log_layout = QVBoxLayout()
        self.log_output = QTextEdit()
        self.log_output.setReadOnly(True)
        self.log_output.setFont(QFont("Consolas", 9))
        log_layout.addWidget(self.log_output)
        log_group.setLayout(log_layout)
        main_layout.addWidget(log_group)

    def api_callback(self, event_type, data):
        if event_type == "log":
            self.signals.log_signal.emit(data.get("message", ""))
        elif event_type == "status":
            self.signals.status_signal.emit(data.get("state", "unknown"))
        elif event_type == "stats":
            self.signals.stats_signal.emit(data)

    @pyqtSlot(str)
    def on_log(self, msg):
        self.log_output.append(f"> {msg}")

    @pyqtSlot(str)
    def on_status(self, state):
        self.status_label.setText(f"Status: {state.capitalize()}")
        if state == "running":
            self.status_label.setStyleSheet("color: #2e8b57;") # SeaGreen
            self.start_btn.setEnabled(False)
            self.stop_btn.setEnabled(True)
            self.strict_cb.setEnabled(False)
            self.sysproxy_cb.setEnabled(False)
        elif state == "stopped" or state == "ready":
            self.status_label.setStyleSheet("color: gray;")
            self.start_btn.setEnabled(True)
            self.stop_btn.setEnabled(False)
            self.strict_cb.setEnabled(True)
            self.sysproxy_cb.setEnabled(True)
            self.tx_label.setText("TX: 0 B/s")
            self.rx_label.setText("RX: 0 B/s")
        elif state == "starting" or state == "stopping":
            self.status_label.setStyleSheet("color: #daa520;") # GoldenRod
            self.start_btn.setEnabled(False)
            self.stop_btn.setEnabled(False)

    @pyqtSlot(dict)
    def on_stats(self, stats):
        tx = stats.get("bytes_sent", 0)
        rx = stats.get("bytes_received", 0)
        self.tx_label.setText(f"TX: {self.format_bytes(tx)}/s")
        self.rx_label.setText(f"RX: {self.format_bytes(rx)}/s")

    def format_bytes(self, size):
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size < 1024.0:
                return f"{size:.2f} {unit}"
            size /= 1024.0
        return f"{size:.2f} PB"

    def start_api(self):
        strict = self.strict_cb.isChecked()
        sysproxy = self.sysproxy_cb.isChecked()
        self.api.start_services(strict=strict, sysproxy_on=sysproxy)

    def stop_api(self):
        self.api.stop_services()

    def closeEvent(self, event):
        self.api.stop_services()
        event.accept()

def main():
    app = QApplication(sys.argv)
    app.setStyle("Fusion") # Cleaner native-like cross-platform look
    gui = ShadowLinkGUI()
    gui.show()
    sys.exit(app.exec())

if __name__ == "__main__":
    main()
