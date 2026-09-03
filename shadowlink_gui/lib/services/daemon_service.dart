import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

import '../config/app_config.dart';

enum DaemonStatus { disconnected, connecting, connected, error }

/// DaemonService manages the ShadowLink background daemon.
///
/// On **desktop** (Windows / macOS / Linux), it spawns the bundled Go
/// binary as a child process and monitors its stdout for status signals.
///
/// On **mobile** (Android / iOS), it communicates with the Go engine
/// embedded via gomobile using a Flutter MethodChannel — no binary
/// spawning is needed or possible in a mobile sandbox.
class DaemonService {
  // ─── Singleton ──────────────────────────────────────────────────────────────
  static final DaemonService _instance = DaemonService._internal();
  factory DaemonService() => _instance;
  DaemonService._internal();

  // ─── State ──────────────────────────────────────────────────────────────────
  Process? _process;
  Timer? _connectionTimeout;
  bool _mobileStarted = false; // Guards duplicate MethodChannel calls on mobile

  final ValueNotifier<DaemonStatus> statusNotifier =
      ValueNotifier(DaemonStatus.disconnected);
  final ValueNotifier<String> logNotifier = ValueNotifier('');

  // Node role flags (only meaningful on desktop — mobile is always entry-only).
  bool isEntry = true;
  bool isRelay = false;
  bool isExit = false;

  // ─── Mobile MethodChannel (sourced from app_config) ─────────────────────────
  static const MethodChannel _mobileChannel = MethodChannel(kDaemonChannel);

  // ─── Public API ─────────────────────────────────────────────────────────────

  /// Starts the daemon. Dispatches to the correct implementation for the platform.
  Future<void> startDaemon() async {
    if (Platform.isAndroid || Platform.isIOS) {
      await _startMobileDaemon();
    } else {
      await _startDesktopDaemon();
    }
  }

  /// Stops the daemon. Dispatches to the correct implementation for the platform.
  Future<void> stopDaemon() async {
    _connectionTimeout?.cancel();
    if (Platform.isAndroid || Platform.isIOS) {
      await _stopMobileDaemon();
    } else {
      await _stopDesktopDaemon();
    }
  }

  // ─── Mobile implementation ──────────────────────────────────────────────────

  Future<void> _startMobileDaemon() async {
    if (_mobileStarted) return; // Guard: prevent duplicate channel calls

    statusNotifier.value = DaemonStatus.connecting;
    logNotifier.value = 'Starting mobile ShadowLink Daemon…\n';

    _connectionTimeout?.cancel();
    _connectionTimeout = Timer(kConnectionTimeout, () {
      if (statusNotifier.value == DaemonStatus.connecting) {
        logNotifier.value += '\nConnection timeout. Stopping daemon.';
        _stopMobileDaemon();
      }
    });

    try {
      logNotifier.value += 'Invoking native Go engine via MethodChannel…\n';
      // Port sourced from app_config.kDefaultSOCKSPort — not hardcoded.
      final bool started = await _mobileChannel
          .invokeMethod(kMethodStart, {kArgPort: kDefaultSOCKSPort});
      if (started) {
        _mobileStarted = true;
        _connectionTimeout?.cancel();
        logNotifier.value += 'Mobile Daemon started successfully.\n';
        statusNotifier.value = DaemonStatus.connected;
      }
    } catch (e) {
      _connectionTimeout?.cancel();
      statusNotifier.value = DaemonStatus.error;
      logNotifier.value += '\nFailed to start mobile daemon: $e';
    }
  }

  Future<void> _stopMobileDaemon() async {
    try {
      logNotifier.value += '\nStopping mobile daemon…';
      await _mobileChannel.invokeMethod(kMethodStop);
      _mobileStarted = false;
      statusNotifier.value = DaemonStatus.disconnected;
    } catch (e) {
      logNotifier.value += '\nFailed to stop mobile daemon: $e';
    }
  }

  // ─── Desktop implementation ─────────────────────────────────────────────────

  Future<void> _startDesktopDaemon() async {
    if (_process != null) return; // Guard: already running

    statusNotifier.value = DaemonStatus.connecting;
    logNotifier.value = 'Starting ShadowLink Daemon…\n';

    _connectionTimeout?.cancel();
    _connectionTimeout = Timer(kConnectionTimeout, () {
      if (statusNotifier.value == DaemonStatus.connecting) {
        logNotifier.value += '\nConnection timeout. Stopping daemon.';
        _stopDesktopDaemon();
      }
    });

    try {
      final List<String> args = [];
      if (isEntry) args.add('--entry');
      if (isRelay) args.add('--relay');
      if (isExit) args.add('--exit');

      // Auto-configure system proxy on Windows for entry nodes.
      if (isEntry && Platform.isWindows) args.add('--sysproxy');

      final binPath = _getBinaryPath();

      // Write the EULA acceptance file so the CLI daemon does not hang on stdin.
      // File name sourced from app_config.kEulaFileName — not hardcoded.
      final eulaFile = File(await _getEulaPath());
      if (!await eulaFile.exists()) {
        await eulaFile.writeAsString('accepted');
      }

      _process = await Process.start(binPath, args);

      _process!.stdout.listen((data) {
        final line = String.fromCharCodes(data);
        _appendLog(line);
        // Detect connection success from daemon stdout.
        if (line.contains('Starting local SOCKS5 proxy') ||
            line.contains('Announce')) {
          _connectionTimeout?.cancel();
          statusNotifier.value = DaemonStatus.connected;
        }
      });

      _process!.stderr.listen((data) {
        _appendLog('ERROR: ${String.fromCharCodes(data)}');
      });

      _process!.exitCode.then((code) {
        _connectionTimeout?.cancel();
        _appendLog('\nDaemon exited with code $code');
        statusNotifier.value = DaemonStatus.disconnected;
        _process = null;
      });
    } catch (e) {
      _connectionTimeout?.cancel();
      statusNotifier.value = DaemonStatus.error;
      _appendLog('\nFailed to start daemon: $e');
      _process = null;
    }
  }

  /// Appends text to logNotifier while enforcing a 20,000 character sliding window
  /// to prevent unbounded memory growth over prolonged uptime.
  void _appendLog(String text) {
    const int maxLogLength = 20000;
    final current = logNotifier.value + text;
    if (current.length > maxLogLength) {
      logNotifier.value = current.substring(current.length - maxLogLength);
    } else {
      logNotifier.value = current;
    }
  }

  Future<void> _stopDesktopDaemon() async {
    if (_process == null) return;
    logNotifier.value += '\nStopping daemon…';
    _process!.kill();
    _process = null;
    statusNotifier.value = DaemonStatus.disconnected;

    // Failsafe: On Windows, Process.kill() is a hard SIGKILL and skips Go defers.
    // Explicitly invoke the --reset-proxy flag to restore internet connectivity.
    if (Platform.isWindows && isEntry) {
      try {
        await Process.run(_getBinaryPath(), ['--reset-proxy']);
        logNotifier.value += '\nSystem proxy restored successfully.';
      } catch (e) {
        logNotifier.value += '\nFailed to restore proxy: $e';
      }
    }
  }

  // ─── Path helpers (desktop-only) ─────────────────────────────────────────────

  /// Returns the path to the Go binary.
  ///
  /// This method is only called from desktop code paths. It will throw
  /// [UnsupportedError] if invoked on an unsupported platform — but that is
  /// impossible because both callers are gated behind `!Platform.isAndroid &&
  /// !Platform.isIOS`.
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
      throw UnsupportedError('Unsupported desktop platform: ${Platform.operatingSystem}');
    }

    // Production: binary is placed next to the Flutter executable by the installer.
    final productionPath = p.join(execDir, bundledName);
    if (File(productionPath).existsSync()) return productionPath;

    // Development fallback: navigate up from the Flutter build output to the repo root.
    // e.g. .../shadowlink_gui/build/windows/x64/runner/Debug/ → ../../../../../../
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

  /// Returns a writable path for the EULA acceptance sentinel file.
  ///
  /// Uses [kEulaFileName] from app_config — not a hardcoded string.
  /// On mobile, uses the app documents directory (Platform.resolvedExecutable
  /// is not writable in a mobile sandbox).
  Future<String> _getEulaPath() async {
    if (Platform.isAndroid || Platform.isIOS) {
      final dir = await getApplicationDocumentsDirectory();
      return p.join(dir.path, kEulaFileName);
    }
    final execDir = File(Platform.resolvedExecutable).parent.path;
    return p.join(execDir, kEulaFileName);
  }
}
