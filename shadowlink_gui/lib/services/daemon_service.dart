import 'dart:io';
import 'dart:async';
import 'package:flutter/foundation.dart';

enum DaemonStatus { disconnected, connecting, connected, error }

class DaemonService {
  static final DaemonService _instance = DaemonService._internal();
  factory DaemonService() => _instance;
  DaemonService._internal();

  Process? _process;
  final ValueNotifier<DaemonStatus> statusNotifier = ValueNotifier(DaemonStatus.disconnected);
  final ValueNotifier<String> logNotifier = ValueNotifier("");

  bool isEntry = true;
  bool isRelay = false;
  bool isExit = false;

  String _getBinaryPath() {
    // For development, we point to the release directory in the parent folder.
    // In production, this would be bundled via flutter assets or installation scripts.
    if (Platform.isWindows) {
      return '../release/shadowlink-windows-x64.exe';
    } else if (Platform.isMacOS) {
      // Simplification: assuming intel for testing. M-series would be shadowlink-macos-apple-silicon
      return '../release/shadowlink-macos-intel'; 
    } else if (Platform.isLinux) {
      return '../release/shadowlink-linux-x64';
    }
    throw UnsupportedError('Unsupported Platform');
  }

  Future<void> startDaemon() async {
    if (_process != null) return;
    
    statusNotifier.value = DaemonStatus.connecting;
    logNotifier.value = "Starting ShadowLink Daemon...\n";

    try {
      List<String> args = [];
      if (isEntry) args.add('--entry');
      if (isRelay) args.add('--relay');
      if (isExit) args.add('--exit');
      
      // We also auto-configure sysproxy if on Windows and entry is enabled
      if (isEntry && Platform.isWindows) {
        args.add('--sysproxy');
      }

      String binPath = _getBinaryPath();
      
      // Before starting, ensure EULA is "accepted" for the CLI so it doesn't hang waiting for stdin
      final eulaFile = File('../.shadowlink_accepted');
      if (!await eulaFile.exists()) {
        await eulaFile.writeAsString('accepted');
      }

      _process = await Process.start(binPath, args);

      _process!.stdout.listen((data) {
        final line = String.fromCharCodes(data);
        logNotifier.value += line;
        if (line.contains("Starting local SOCKS5 proxy") || line.contains("Announce")) {
           statusNotifier.value = DaemonStatus.connected;
        }
      });

      _process!.stderr.listen((data) {
        logNotifier.value += 'ERROR: ${String.fromCharCodes(data)}';
      });

      _process!.exitCode.then((code) {
        logNotifier.value += "\nDaemon exited with code $code";
        statusNotifier.value = DaemonStatus.disconnected;
        _process = null;
      });

    } catch (e) {
      statusNotifier.value = DaemonStatus.error;
      logNotifier.value += "\nFailed to start daemon: $e";
      _process = null;
    }
  }

  void stopDaemon() {
    if (_process != null) {
      logNotifier.value += "\nStopping daemon...";
      _process!.kill();
      _process = null;
      statusNotifier.value = DaemonStatus.disconnected;
    }
  }
}
