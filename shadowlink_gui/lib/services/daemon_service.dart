import 'dart:async';
import 'dart:convert';
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
  bool _sysProxyEnabled =
      false; // Whether the running daemon was started with --sysproxy
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
    _connectionTimeout = Timer(kConnectionTimeout, () async {
      if (statusNotifier.value == DaemonStatus.connecting) {
        _appendLog('\nConnection timeout. Stopping daemon.');
        await _stopDesktopDaemon();
        statusNotifier.value = DaemonStatus.error;
      }
    });

    try {
      final List<String> args = [];
      if (isEntry) args.add('--entry');
      if (isRelay) args.add('--relay');
      if (isExit) args.add('--exit');

      // Auto-configure system proxy on desktop platforms for entry nodes.
      _sysProxyEnabled = isEntry &&
          (Platform.isWindows || Platform.isMacOS || Platform.isLinux);
      if (_sysProxyEnabled) args.add('--sysproxy');

      // Explicitly pass --accept-eula so headless/daemon execution never hangs on stdin.
      args.add('--accept-eula');

      final binPath = _getBinaryPath();

      // Best-effort: write the EULA acceptance file. --accept-eula already keeps
      // the daemon from blocking on stdin, so an unwritable install directory
      // (e.g. Program Files) must not stop the daemon from starting.
      // File name sourced from app_config.kEulaFileName — not hardcoded.
      try {
        final eulaFile = File(await eulaFilePath());
        if (!await eulaFile.exists()) {
          await eulaFile.writeAsString('accepted');
        }
      } on FileSystemException catch (e) {
        _appendLog(
            'Note: could not write EULA file (${e.message}); continuing.\n');
      }

      final process = await Process.start(binPath, args);
      _process = process;

      // Go's log package writes to stderr, so readiness markers must be detected
      // on both streams. Decoding as UTF-8 lines keeps multi-byte characters and
      // markers intact when they straddle chunk boundaries.
      void onLine(String line) {
        // Ignore output from a stale process.
        if (!identical(_process, process)) return;
        _appendLog('$line\n');
        if (statusNotifier.value == DaemonStatus.connecting &&
            _readyMarkers.any(line.contains)) {
          _connectionTimeout?.cancel();
          statusNotifier.value = DaemonStatus.connected;
        }
      }

      process.stdout
          .transform(const Utf8Decoder(allowMalformed: true))
          .transform(const LineSplitter())
          .listen(onLine);
      process.stderr
          .transform(const Utf8Decoder(allowMalformed: true))
          .transform(const LineSplitter())
          .listen(onLine);

      process.exitCode.then((code) {
        // Ignore exits of a process that was already stopped or replaced; otherwise
        // a quick stop/start would clear the handle of the newly started daemon.
        if (!identical(_process, process)) return;
        _connectionTimeout?.cancel();
        _appendLog('\nDaemon exited with code $code');
        statusNotifier.value =
            code == 0 ? DaemonStatus.disconnected : DaemonStatus.error;
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
    final process = _process;
    if (process == null) return;
    _appendLog('\nStopping daemon…');
    _process = null;
    process.kill();
    statusNotifier.value = DaemonStatus.disconnected;

    // Failsafe: on Windows Process.kill() terminates immediately and skips Go defers.
    // Explicitly invoke the --reset-proxy flag to restore internet connectivity.
    // Uses the flag captured at start, since the role toggles may have changed since.
    if (_sysProxyEnabled) {
      _sysProxyEnabled = false;
      try {
        await Process.run(_getBinaryPath(), ['--reset-proxy']);
        _appendLog('\nSystem proxy restored successfully.');
      } catch (e) {
        _appendLog('\nFailed to restore proxy: $e');
      }
    }
  }

  /// Daemon log lines that signal the node is up and serving.
  static const List<String> _readyMarkers = [
    'SOCKS5 proxy listening', // Entry: local proxy bound
    'ShadowLink node ready', // Relay / exit: stream handler registered
  ];

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
      throw UnsupportedError(
          'Unsupported desktop platform: ${Platform.operatingSystem}');
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
  /// On mobile, uses the app documents directory (the sandbox). On desktop, uses
  /// the per-user application support directory: the folder next to the
  /// executable is read-only for installed builds (e.g. Program Files).
  static Future<String> eulaFilePath() async {
    final Directory dir;
    if (Platform.isAndroid || Platform.isIOS) {
      dir = await getApplicationDocumentsDirectory();
    } else {
      dir = await getApplicationSupportDirectory();
      await dir.create(recursive: true);
    }
    return p.join(dir.path, kEulaFileName);
  }
}
