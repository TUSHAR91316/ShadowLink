import 'dart:async';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:path/path.dart' as p;
import 'package:flutter/services.dart';

enum DaemonStatus { disconnected, connecting, connected, error }

class DaemonService {
  static final DaemonService _instance = DaemonService._internal();
  factory DaemonService() => _instance;
  DaemonService._internal();

  Process? _process;
  Timer? _connectionTimeout;

  final ValueNotifier<DaemonStatus> statusNotifier =
      ValueNotifier(DaemonStatus.disconnected);
  final ValueNotifier<String> logNotifier = ValueNotifier("");

  bool isEntry = true;
  bool isRelay = false;
  bool isExit = false;

  static const MethodChannel _mobileChannel = MethodChannel('com.shadowlink/daemon');

  /// Bug 7 Fix: Compute binary path relative to the Flutter executable,
  /// not the CWD. Works in both development and production builds.
  String _getBinaryPath() {
    final execDir = File(Platform.resolvedExecutable).parent.path;

    String bundledName;
    String devName;

    if (Platform.isWindows) {
      bundledName = 'shadowlink.exe';
      devName = 'shadowlink-windows-x64.exe';
    } else if (Platform.isMacOS) {
      bundledName = 'shadowlink';
      devName = 'shadowlink-macos-intel';
    } else if (Platform.isLinux) {
      bundledName = 'shadowlink';
      devName = 'shadowlink-linux-x64';
    } else {
      throw UnsupportedError('Unsupported Platform');
    }

    // Production: binary is placed next to the Flutter executable by the installer.
    final productionPath = p.join(execDir, bundledName);
    if (File(productionPath).existsSync()) return productionPath;

    // Development fallback: navigate up from the Flutter build output dir to
    // the repo root, then into the release folder.
    // Path: .../shadowlink_gui/build/windows/x64/runner/Debug/ -> ../../../../../
    final projectRoot = File(Platform.resolvedExecutable)
        .parent
        .parent
        .parent
        .parent
        .parent
        .parent
        .path;
    return p.join(projectRoot, 'release', devName);
  }

  /// Bug 7 Fix: EULA file also uses a stable absolute path.
  String _getEulaPath() {
    final execDir = File(Platform.resolvedExecutable).parent.path;
    return p.join(execDir, '.shadowlink_accepted');
  }

  Future<void> startDaemon() async {
    if (_process != null) return;

    statusNotifier.value = DaemonStatus.connecting;
    logNotifier.value = "Starting ShadowLink Daemon...\n";

    // I-4 Fix: Cancel any existing timeout and set a 30-second connection watchdog.
    _connectionTimeout?.cancel();
    _connectionTimeout = Timer(const Duration(seconds: 30), () {
      if (statusNotifier.value == DaemonStatus.connecting) {
        logNotifier.value += '\nConnection timeout after 30s. Stopping daemon.';
        stopDaemon();
      }
    });

    try {
      if (Platform.isAndroid || Platform.isIOS) {
        logNotifier.value += "Invoking mobile MethodChannel...\n";
        bool started = await _mobileChannel.invokeMethod('start', {"port": 1080});
        if (started) {
          logNotifier.value += "Mobile Daemon Started Successfully\n";
          _connectionTimeout?.cancel();
          statusNotifier.value = DaemonStatus.connected;
        }
        return;
      }

      List<String> args = [];
      if (isEntry) args.add('--entry');
      if (isRelay) args.add('--relay');
      if (isExit) args.add('--exit');

      // Auto-configure system proxy on Windows when acting as entry node.
      if (isEntry && Platform.isWindows) {
        args.add('--sysproxy');
      }

      final binPath = _getBinaryPath();

      // Write the EULA acceptance file so the CLI daemon does not hang on stdin.
      final eulaFile = File(_getEulaPath());
      if (!await eulaFile.exists()) {
        await eulaFile.writeAsString('accepted');
      }

      _process = await Process.start(binPath, args);

      _process!.stdout.listen((data) {
        final line = String.fromCharCodes(data);
        logNotifier.value += line;

        if (line.contains("Starting local SOCKS5 proxy") ||
            line.contains("Announce")) {
          _connectionTimeout?.cancel(); // Connected — cancel the timeout.
          statusNotifier.value = DaemonStatus.connected;
        }
      });

      _process!.stderr.listen((data) {
        logNotifier.value += 'ERROR: ${String.fromCharCodes(data)}';
      });

      _process!.exitCode.then((code) {
        _connectionTimeout?.cancel();
        logNotifier.value += "\nDaemon exited with code $code";
        statusNotifier.value = DaemonStatus.disconnected;
        _process = null;
      });
    } catch (e) {
      _connectionTimeout?.cancel();
      statusNotifier.value = DaemonStatus.error;
      logNotifier.value += "\nFailed to start daemon: $e";
      _process = null;
    }
  }

  Future<void> stopDaemon() async {
    _connectionTimeout?.cancel();

    if (Platform.isAndroid || Platform.isIOS) {
      try {
        logNotifier.value += "\nStopping mobile daemon...";
        await _mobileChannel.invokeMethod('stop');
        statusNotifier.value = DaemonStatus.disconnected;
      } catch (e) {
        logNotifier.value += "\nFailed to stop mobile daemon: $e";
      }
      return;
    }

    if (_process != null) {
      logNotifier.value += "\nStopping daemon...";
      _process!.kill();
      _process = null;
      statusNotifier.value = DaemonStatus.disconnected;

      // Failsafe: On Windows, Process.kill() is a hard SIGKILL and skips Go defers.
      // Explicitly invoke the --reset-proxy flag to restore internet connectivity.
      if (Platform.isWindows && isEntry) {
        try {
          await Process.run(_getBinaryPath(), ['--reset-proxy']);
          logNotifier.value += "\nSystem proxy restored successfully.";
        } catch (e) {
          logNotifier.value += "\nFailed to restore proxy: $e";
        }
      }
    }
  }
}
