import 'dart:io';
import 'package:flutter/material.dart';
import '../theme/app_theme.dart';
import '../services/daemon_service.dart';


class DashboardScreen extends StatefulWidget {
  const DashboardScreen({Key? key}) : super(key: key);

  @override
  _DashboardScreenState createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  final DaemonService _daemon = DaemonService();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text("SHADOWLINK", style: TextStyle(letterSpacing: 4, fontWeight: FontWeight.bold)),
      ),
      body: ValueListenableBuilder<DaemonStatus>(
        valueListenable: _daemon.statusNotifier,
        builder: (context, status, child) {
          bool isConnected = status == DaemonStatus.connected;
          bool isConnecting = status == DaemonStatus.connecting;

          Color statusColor = AppTheme.textMuted;
          String statusText = "DISCONNECTED";
          if (isConnected) {
            statusColor = AppTheme.primary;
            statusText = "SECURED";
          } else if (isConnecting) {
            statusColor = Colors.orange;
            statusText = "CONNECTING...";
          } else if (status == DaemonStatus.error) {
            statusColor = AppTheme.error;
            statusText = "ERROR";
          }

          return Padding(
            padding: const EdgeInsets.all(24.0),
            child: Column(
              children: [
                Expanded(
                  child: Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        GestureDetector(
                          // I-5 Fix: Use async onTap so stopDaemon is fully awaited
                          // before any subsequent tap can re-trigger a state change.
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
                              boxShadow: isConnected ? [
                                BoxShadow(
                                  color: AppTheme.primary.withOpacity(0.3),
                                  blurRadius: 30,
                                  spreadRadius: 10,
                                )
                              ] : null,
                              border: Border.all(
                                color: isConnected ? AppTheme.primary : AppTheme.textMuted.withOpacity(0.2),
                                width: 2,
                              )
                            ),
                            child: Center(
                              child: Icon(
                                isConnected ? Icons.shield : Icons.power_settings_new,
                                size: 80,
                                color: isConnected ? AppTheme.primary : AppTheme.textMuted,
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
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: AppTheme.surface,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Column(
                    children: [
                      SwitchListTile(
                        title: const Text("Use as Client (Entry Node)", style: TextStyle(color: AppTheme.textMain)),
                        subtitle: const Text("Route your traffic through the dVPN", style: TextStyle(color: AppTheme.textMuted, fontSize: 12)),
                        value: _daemon.isEntry,
                        onChanged: isConnected ? null : (val) => setState(() => _daemon.isEntry = val),
                        activeColor: AppTheme.primary,
                      ),
                      // Relay and Exit nodes are only supported on Desktop platforms.
                      // On Android/iOS, the Go engine runs in entry-node-only mode.
                      if (!Platform.isAndroid && !Platform.isIOS) ...([
                        SwitchListTile(
                          title: const Text("Contribute Bandwidth (Relay Node)", style: TextStyle(color: AppTheme.textMain)),
                          subtitle: const Text("Help route encrypted traffic for others", style: TextStyle(color: AppTheme.textMuted, fontSize: 12)),
                          value: _daemon.isRelay,
                          onChanged: isConnected ? null : (val) => setState(() => _daemon.isRelay = val),
                          activeColor: AppTheme.primary,
                        ),
                        SwitchListTile(
                          title: const Text("Exit Node Operator", style: TextStyle(color: AppTheme.error)),
                          subtitle: const Text("WARNING: Public traffic exits your IP", style: TextStyle(color: AppTheme.error, fontSize: 12)),
                          value: _daemon.isExit,
                          onChanged: isConnected ? null : (val) => setState(() => _daemon.isExit = val),
                          activeColor: AppTheme.error,
                        ),
                      ]),
                    ],
                  ),
                )
              ],
            ),
          );
        },
      ),
    );
  }
}
