import 'dart:io';

import 'package:flutter/material.dart';

import '../services/daemon_service.dart';
import '../theme/app_theme.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key}); // use_super_parameters fix

  @override
  // ignore: library_private_types_in_public_api
  _DashboardScreenState createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  final DaemonService _daemon = DaemonService();

  // ─── Role toggle helpers ─────────────────────────────────────────────────────

  /// B13 Fix: Entry node cannot be turned off unless at least one other role
  /// (relay or exit) is active. This prevents the user from leaving the daemon
  /// with no role, which the CLI silently treats as entry-only anyway.
  void _setEntry(bool val) {
    if (!val && !_daemon.isRelay && !_daemon.isExit) return;
    setState(() => _daemon.isEntry = val);
  }

  void _setRelay(bool val) => setState(() => _daemon.isRelay = val);
  void _setExit(bool val) => setState(() => _daemon.isExit = val);

  // ─── Build ───────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text(
          'SHADOWLINK',
          style: TextStyle(letterSpacing: 4, fontWeight: FontWeight.bold),
        ),
      ),
      body: ValueListenableBuilder<DaemonStatus>(
        valueListenable: _daemon.statusNotifier,
        builder: (context, status, _) {
          final isConnected = status == DaemonStatus.connected;
          final isConnecting = status == DaemonStatus.connecting;
          final isError = status == DaemonStatus.error;

          final Color statusColor = isConnected
              ? AppTheme.primary
              : isConnecting
                  ? Colors.orange
                  : isError
                      ? AppTheme.error
                      : AppTheme.textMuted;

          final String statusText = isConnected
              ? 'SECURED'
              : isConnecting
                  ? 'CONNECTING…'
                  : isError
                      ? 'ERROR'
                      : 'DISCONNECTED';

          return Padding(
            padding: const EdgeInsets.all(24.0),
            child: Column(
              children: [
                // ── Connect / Disconnect button ──────────────────────────────
                Expanded(
                  child: Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        GestureDetector(
                          onTap: () async {
                            if (isConnected || isConnecting) {
                              await _daemon.stopDaemon();
                            } else {
                              await _daemon.startDaemon();
                            }
                          },
                          child: Container(
                            width: 200,
                            height: 200,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: AppTheme.surface,
                              boxShadow: isConnected
                                  ? [
                                      BoxShadow(
                                        // withValues() replaces deprecated withOpacity()
                                        color: AppTheme.primary
                                            .withValues(alpha: 0.3),
                                        blurRadius: 30,
                                        spreadRadius: 10,
                                      )
                                    ]
                                  : null,
                              border: Border.all(
                                color: isConnected
                                    ? AppTheme.primary
                                    // withValues() replaces deprecated withOpacity()
                                    : AppTheme.textMuted.withValues(alpha: 0.2),
                                width: 2,
                              ),
                            ),
                            child: Center(
                              child: Icon(
                                isConnected
                                    ? Icons.shield
                                    : Icons.power_settings_new,
                                size: 80,
                                color: isConnected
                                    ? AppTheme.primary
                                    : AppTheme.textMuted,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 32),
                        Text(
                          statusText,
                          style: TextStyle(
                            color: statusColor,
                            fontSize: 20,
                            fontWeight: FontWeight.bold,
                            letterSpacing: 2,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),

                // ── Role toggles ─────────────────────────────────────────────
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: AppTheme.surface,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Column(
                    children: [
                      SwitchListTile(
                        title: const Text(
                          'Use as Client (Entry Node)',
                          style: TextStyle(color: AppTheme.textMain),
                        ),
                        subtitle: const Text(
                          'Route your traffic through the dVPN',
                          style: TextStyle(
                              color: AppTheme.textMuted, fontSize: 12),
                        ),
                        value: _daemon.isEntry,
                        // Lock to ON when no other role is active (B13 Fix).
                        onChanged: isConnected
                            ? null
                            : (!_daemon.isRelay && !_daemon.isExit)
                                ? null
                                : _setEntry,
                        // activeThumbColor replaces deprecated activeColor
                        activeThumbColor: AppTheme.primary,
                      ),

                      // Relay and Exit toggles are desktop-only.
                      if (!Platform.isAndroid && !Platform.isIOS) ...[
                        SwitchListTile(
                          title: const Text(
                            'Contribute Bandwidth (Relay Node)',
                            style: TextStyle(color: AppTheme.textMain),
                          ),
                          subtitle: const Text(
                            'Help route encrypted traffic for others',
                            style: TextStyle(
                                color: AppTheme.textMuted, fontSize: 12),
                          ),
                          value: _daemon.isRelay,
                          onChanged: isConnected ? null : _setRelay,
                          // activeThumbColor replaces deprecated activeColor
                          activeThumbColor: AppTheme.primary,
                        ),
                        SwitchListTile(
                          title: const Text(
                            'Exit Node Operator',
                            style: TextStyle(color: AppTheme.error),
                          ),
                          subtitle: const Text(
                            'WARNING: Public traffic exits your IP',
                            style: TextStyle(
                                color: AppTheme.error, fontSize: 12),
                          ),
                          value: _daemon.isExit,
                          onChanged: isConnected ? null : _setExit,
                          // activeThumbColor replaces deprecated activeColor
                          activeThumbColor: AppTheme.error,
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
