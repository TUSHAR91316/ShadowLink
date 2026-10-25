import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';

import '../services/daemon_service.dart';
import '../theme/app_theme.dart';

/// Represents a dVPN node in the decentralized mesh network.
class DVPNNode {
  final String city;
  final String country;
  final String flag;
  final String code;
  final String ip;
  final int pingMs;
  final double load;

  const DVPNNode({
    required this.city,
    required this.country,
    required this.flag,
    required this.code,
    required this.ip,
    required this.pingMs,
    required this.load,
  });
}

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen>
    with SingleTickerProviderStateMixin {
  final DaemonService _daemon = DaemonService();

  // ─── Animation Controllers ──────────────────────────────────────────────────
  late final AnimationController _pulseController;
  late final Animation<double> _pulseAnimation;

  // ─── Live Telemetry Simulation State ────────────────────────────────────────
  Timer? _tickerTimer;
  int _sessionSeconds = 8078; // 02:14:38 initial sample uptime
  bool _killSwitchActive = true;
  bool _dnsLeakProtected = true;

  // Available exit nodes for server selector modal
  final List<DVPNNode> _availableNodes = const [
    DVPNNode(
      city: 'Zurich',
      country: 'Switzerland',
      flag: '🇨🇭',
      code: 'CH-09',
      ip: '185.220.101.42',
      pingMs: 14,
      load: 0.32,
    ),
    DVPNNode(
      city: 'Frankfurt',
      country: 'Germany',
      flag: '🇩🇪',
      code: 'DE-04',
      ip: '194.36.144.18',
      pingMs: 18,
      load: 0.48,
    ),
    DVPNNode(
      city: 'Reykjavik',
      country: 'Iceland',
      flag: '🇮🇸',
      code: 'IS-02',
      ip: '185.112.82.11',
      pingMs: 28,
      load: 0.22,
    ),
    DVPNNode(
      city: 'Amsterdam',
      country: 'Netherlands',
      flag: '🇳🇱',
      code: 'NL-15',
      ip: '198.51.100.77',
      pingMs: 21,
      load: 0.65,
    ),
    DVPNNode(
      city: 'Tokyo',
      country: 'Japan',
      flag: '🇯🇵',
      code: 'JP-08',
      ip: '203.0.113.194',
      pingMs: 110,
      load: 0.41,
    ),
  ];

  late DVPNNode _selectedNode;
  int _bottomNavIndex = 0;

  @override
  void initState() {
    super.initState();
    _selectedNode = _availableNodes[0];

    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 3),
    )..repeat();

    _pulseAnimation = CurvedAnimation(
      parent: _pulseController,
      curve: Curves.easeInOut,
    );

    _startUptimeTicker();
  }

  @override
  void dispose() {
    _pulseController.dispose();
    _tickerTimer?.cancel();
    super.dispose();
  }

  void _startUptimeTicker() {
    _tickerTimer?.cancel();
    _tickerTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted && _daemon.statusNotifier.value == DaemonStatus.connected) {
        setState(() {
          _sessionSeconds++;
        });
      }
    });
  }

  String _formatUptime(int totalSeconds) {
    final hours = (totalSeconds ~/ 3600).toString().padLeft(2, '0');
    final minutes = ((totalSeconds % 3600) ~/ 60).toString().padLeft(2, '0');
    final seconds = (totalSeconds % 60).toString().padLeft(2, '0');
    return '$hours:$minutes:$seconds';
  }

  // ─── Role toggle helpers ───────────────────────────────────────────────────
  void _setEntry(bool val) {
    if (!val && !_daemon.isRelay && !_daemon.isExit) return;
    setState(() => _daemon.isEntry = val);
  }

  void _setRelay(bool val) => setState(() => _daemon.isRelay = val);
  void _setExit(bool val) => setState(() => _daemon.isExit = val);

  // ─── Connect / Disconnect Action ───────────────────────────────────────────
  Future<void> _handleConnectToggle(bool isConnected, bool isConnecting) async {
    if (isConnected || isConnecting) {
      await _daemon.stopDaemon();
    } else {
      await _daemon.startDaemon();
    }
  }

  // ─── Server Selector Modal ─────────────────────────────────────────────────
  void _showServerSelector(BuildContext context) {
    showModalBottomSheet(
      context: context,
      backgroundColor: AppTheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (ctx) {
        return Padding(
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Expanded(
                    child: Text(
                      'Select Exit Egress Node',
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                        color: AppTheme.textMain,
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, color: AppTheme.textMuted),
                    onPressed: () => Navigator.pop(ctx),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Flexible(
                child: ListView.separated(
                  shrinkWrap: true,
                  itemCount: _availableNodes.length,
                  separatorBuilder: (_, __) => const Divider(),
                  itemBuilder: (_, index) {
                    final node = _availableNodes[index];
                    final isSelected = node.code == _selectedNode.code;
                    return Material(
                      color: Colors.transparent,
                      child: ListTile(
                        contentPadding: const EdgeInsets.symmetric(horizontal: 8),
                        leading: Container(
                          width: 40,
                          height: 40,
                          decoration: BoxDecoration(
                            color: AppTheme.surfaceHigh,
                            borderRadius: BorderRadius.circular(10),
                            border: Border.all(
                              color: isSelected
                                  ? AppTheme.primary
                                  : AppTheme.borderSubtle,
                            ),
                          ),
                          child: Center(
                            child: Text(
                              node.flag,
                              style: const TextStyle(fontSize: 20),
                            ),
                          ),
                        ),
                        title: Text(
                          '${node.city}, ${node.country}',
                          style: TextStyle(
                            color: isSelected
                                ? AppTheme.primaryBright
                                : AppTheme.textMain,
                            fontWeight:
                                isSelected ? FontWeight.bold : FontWeight.w500,
                          ),
                        ),
                        subtitle: Text(
                          '${node.code} · ${node.ip}',
                          style: const TextStyle(
                            color: AppTheme.textMuted,
                            fontSize: 12,
                          ),
                        ),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(
                              Icons.signal_cellular_alt,
                              size: 16,
                              color: isSelected
                                  ? AppTheme.primary
                                  : AppTheme.textMuted,
                            ),
                            const SizedBox(width: 4),
                            Text(
                              '${node.pingMs}ms',
                              style: TextStyle(
                                color: isSelected
                                    ? AppTheme.primary
                                    : AppTheme.textMuted,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ],
                        ),
                        onTap: () {
                          setState(() => _selectedNode = node);
                          Navigator.pop(ctx);
                        },
                      ),
                    );
                  },
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  // ─── Main Build & Responsive Dispatcher ────────────────────────────────────
  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<DaemonStatus>(
      valueListenable: _daemon.statusNotifier,
      builder: (context, status, _) {
        final isConnected = status == DaemonStatus.connected;
        final isConnecting = status == DaemonStatus.connecting;
        final isError = status == DaemonStatus.error;

        return Scaffold(
          backgroundColor: AppTheme.background,
          body: LayoutBuilder(
            builder: (context, constraints) {
              if (constraints.maxWidth > 850) {
                return _buildDesktopLayout(
                    isConnected, isConnecting, isError);
              }
              return _buildMobileLayout(isConnected, isConnecting, isError);
            },
          ),
          bottomNavigationBar: LayoutBuilder(
            builder: (context, constraints) {
              if (constraints.maxWidth <= 850) {
                return _buildMobileBottomNav();
              }
              return const SizedBox.shrink();
            },
          ),
        );
      },
    );
  }

  // ═════════════════════════════════════════════════════════════════════════════
  // DESKTOP LAYOUT (3-Column Proton/ExpressVPN Inspired Client Window)
  // ═════════════════════════════════════════════════════════════════════════════
  Widget _buildDesktopLayout(
      bool isConnected, bool isConnecting, bool isError) {
    return Row(
      children: [
        // ── Column 1: Left Navigation Rail (260px) ───────────────────────────
        SizedBox(
          width: 260,
          child: Container(
            color: AppTheme.surfaceLow,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Top Brand Header
                Padding(
                  padding: const EdgeInsets.all(20),
                  child: Row(
                    children: [
                      Container(
                        width: 36,
                        height: 36,
                        decoration: BoxDecoration(
                          color: AppTheme.surfaceHigh,
                          borderRadius: BorderRadius.circular(10),
                          border: Border.all(
                            color: AppTheme.primary.withValues(alpha: 0.4),
                          ),
                          boxShadow: [
                            BoxShadow(
                              color: AppTheme.primary.withValues(alpha: 0.2),
                              blurRadius: 10,
                            ),
                          ],
                        ),
                        child: const Icon(
                          Icons.shield,
                          color: AppTheme.primary,
                          size: 22,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text(
                              'SHADOWLINK',
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontWeight: FontWeight.bold,
                                fontSize: 16,
                                letterSpacing: 1.5,
                                color: AppTheme.textMain,
                              ),
                            ),
                            Wrap(
                              crossAxisAlignment: WrapCrossAlignment.center,
                              spacing: 6,
                              children: [
                                Container(
                                  padding: const EdgeInsets.symmetric(
                                      horizontal: 6, vertical: 1),
                                  decoration: BoxDecoration(
                                    color: AppTheme.secondary.withValues(alpha: 0.15),
                                    borderRadius: BorderRadius.circular(4),
                                    border: Border.all(
                                      color: AppTheme.secondary.withValues(alpha: 0.3),
                                    ),
                                  ),
                                  child: const Text(
                                    'dVPN v2.0',
                                    style: TextStyle(
                                      fontSize: 9,
                                      color: AppTheme.secondary,
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                ),
                                const Text(
                                  'sovereign',
                                  style: TextStyle(
                                    fontSize: 10,
                                    color: AppTheme.textMuted,
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),

                // Quick Connect Action Card
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Container(
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: AppTheme.surfaceContainer,
                      borderRadius: BorderRadius.circular(14),
                      border: Border.all(
                        color: isConnected
                            ? AppTheme.primary.withValues(alpha: 0.4)
                            : AppTheme.borderSubtle,
                      ),
                      boxShadow: isConnected
                          ? [
                              BoxShadow(
                                color: AppTheme.primary.withValues(alpha: 0.15),
                                blurRadius: 16,
                              )
                            ]
                          : null,
                    ),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Container(
                                    width: 6,
                                    height: 6,
                                    decoration: BoxDecoration(
                                      shape: BoxShape.circle,
                                      color: isConnected
                                          ? AppTheme.primary
                                          : isConnecting
                                              ? AppTheme.warning
                                              : AppTheme.textMuted,
                                    ),
                                  ),
                                  const SizedBox(width: 6),
                                  Text(
                                    isConnected
                                        ? 'PROTECTED'
                                        : isConnecting
                                            ? 'CONNECTING'
                                            : 'DISCONNECTED',
                                    style: TextStyle(
                                      fontSize: 11,
                                      fontWeight: FontWeight.bold,
                                      letterSpacing: 0.8,
                                      color: isConnected
                                          ? AppTheme.primary
                                          : AppTheme.textMuted,
                                    ),
                                  ),
                                ],
                              ),
                              const SizedBox(height: 4),
                              Text(
                                '${_selectedNode.city} ${_selectedNode.code}',
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontWeight: FontWeight.bold,
                                  fontSize: 13,
                                  color: AppTheme.textMain,
                                ),
                              ),
                            ],
                          ),
                        ),
                        GestureDetector(
                          onTap: () =>
                              _handleConnectToggle(isConnected, isConnecting),
                          child: Container(
                            width: 38,
                            height: 38,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: isConnected
                                  ? AppTheme.primary.withValues(alpha: 0.2)
                                  : AppTheme.surfaceHigh,
                              border: Border.all(
                                color: isConnected
                                    ? AppTheme.primary
                                    : AppTheme.borderSubtle,
                              ),
                            ),
                            child: Icon(
                              Icons.power_settings_new,
                              size: 20,
                              color: isConnected
                                  ? AppTheme.primary
                                  : AppTheme.textMuted,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),

                const SizedBox(height: 16),
                const Divider(),

                // Desktop Navigation Menu
                Expanded(
                  child: ListView(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    children: [
                      _buildNavItem(
                        icon: Icons.hub,
                        title: 'Dashboard',
                        isSelected: true,
                        badgeText: null,
                      ),
                      _buildNavItem(
                        icon: Icons.public,
                        title: 'Server Nodes & Map',
                        isSelected: false,
                        onTap: () => _showServerSelector(context),
                      ),
                      _buildNavItem(
                        icon: Icons.alt_route,
                        title: 'Multi-Hop Circuit',
                        isSelected: false,
                        badgeText: '3 Hops',
                      ),
                      _buildNavItem(
                        icon: Icons.network_check,
                        title: 'Live Telemetry',
                        isSelected: false,
                      ),
                      _buildNavItem(
                        icon: Icons.token,
                        title: 'Node Operator',
                        isSelected: false,
                        badgeText: 'Earn',
                      ),
                      _buildNavItem(
                        icon: Icons.settings,
                        title: 'Settings',
                        isSelected: false,
                      ),
                    ],
                  ),
                ),

                // Bottom Identity & Kill Switch
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    children: [
                      Container(
                        padding: const EdgeInsets.all(10),
                        decoration: BoxDecoration(
                          color: AppTheme.surfaceContainer,
                          borderRadius: BorderRadius.circular(10),
                          border: Border.all(color: AppTheme.borderSubtle),
                        ),
                        child: Row(
                          children: [
                            Container(
                              width: 24,
                              height: 24,
                              decoration: BoxDecoration(
                                color: AppTheme.primary.withValues(alpha: 0.2),
                                borderRadius: BorderRadius.circular(6),
                              ),
                              child: const Center(
                                child: Text(
                                  '0x',
                                  style: TextStyle(
                                    fontSize: 10,
                                    fontWeight: FontWeight.bold,
                                    color: AppTheme.primary,
                                  ),
                                ),
                              ),
                            ),
                            const SizedBox(width: 8),
                            const Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    'Sovereign Identity',
                                    style: TextStyle(
                                      fontSize: 10,
                                      color: AppTheme.textMuted,
                                    ),
                                  ),
                                  Text(
                                    '0x8f4c...4e19',
                                    style: TextStyle(
                                      fontSize: 11,
                                      fontFamily: 'monospace',
                                      color: AppTheme.textMain,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            const Icon(
                              Icons.content_copy,
                              size: 14,
                              color: AppTheme.textMuted,
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 10, vertical: 8),
                        decoration: BoxDecoration(
                          color: AppTheme.surfaceContainer,
                          borderRadius: BorderRadius.circular(10),
                          border: Border.all(
                            color: _killSwitchActive
                                ? AppTheme.primary.withValues(alpha: 0.4)
                                : AppTheme.borderSubtle,
                          ),
                        ),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Row(
                              children: [
                                Icon(
                                  Icons.lock,
                                  size: 14,
                                  color: _killSwitchActive
                                      ? AppTheme.primary
                                      : AppTheme.textMuted,
                                ),
                                const SizedBox(width: 6),
                                const Text(
                                  'KILL SWITCH',
                                  style: TextStyle(
                                    fontSize: 11,
                                    fontWeight: FontWeight.bold,
                                    color: AppTheme.textMain,
                                  ),
                                ),
                              ],
                            ),
                            Text(
                              _killSwitchActive ? 'ACTIVE' : 'OFF',
                              style: TextStyle(
                                fontSize: 10,
                                fontWeight: FontWeight.bold,
                                color: _killSwitchActive
                                    ? AppTheme.primary
                                    : AppTheme.textMuted,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),

        const VerticalDivider(width: 1),

        // ── Column 2: Center Cyber Stage (Flexible) ───────────────────────────
        Expanded(
          flex: 5,
          child: Column(
            children: [
              // Top Telemetry Header
              Container(
                height: 64,
                padding: const EdgeInsets.symmetric(horizontal: 24),
                decoration: const BoxDecoration(
                  color: AppTheme.surface,
                  border: Border(bottom: BorderSide(color: AppTheme.borderSubtle)),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 10, vertical: 6),
                          decoration: BoxDecoration(
                            color: isConnected
                                ? AppTheme.primary.withValues(alpha: 0.15)
                                : isConnecting
                                    ? AppTheme.warning.withValues(alpha: 0.15)
                                    : AppTheme.surfaceHigh,
                            borderRadius: BorderRadius.circular(20),
                            border: Border.all(
                              color: isConnected
                                  ? AppTheme.primary.withValues(alpha: 0.4)
                                  : AppTheme.borderSubtle,
                            ),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Container(
                                width: 8,
                                height: 8,
                                decoration: BoxDecoration(
                                  shape: BoxShape.circle,
                                  color: isConnected
                                      ? AppTheme.primary
                                      : isConnecting
                                          ? AppTheme.warning
                                          : AppTheme.textMuted,
                                ),
                              ),
                              const SizedBox(width: 8),
                              Text(
                                isConnected
                                    ? 'CONNECTED · SECURE'
                                    : isConnecting
                                        ? 'NEGOTIATING ROUTE…'
                                        : 'NETWORK IDLE',
                                style: TextStyle(
                                  color: isConnected
                                      ? AppTheme.primary
                                      : isConnecting
                                          ? AppTheme.warning
                                          : AppTheme.textMuted,
                                  fontSize: 11,
                                  fontWeight: FontWeight.bold,
                                  letterSpacing: 0.8,
                                ),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(width: 10),
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 8, vertical: 4),
                          decoration: BoxDecoration(
                            color: AppTheme.surfaceContainer,
                            borderRadius: BorderRadius.circular(8),
                            border: Border.all(color: AppTheme.borderSubtle),
                          ),
                          child: const Text(
                            'IP: MASKED',
                            style: TextStyle(
                              fontSize: 11,
                              color: AppTheme.secondary,
                              fontFamily: 'monospace',
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(width: 8),
                    Flexible(
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Flexible(
                            child: Text(
                              'Virtual: ${_selectedNode.ip}',
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                fontSize: 12,
                                fontFamily: 'monospace',
                                color: AppTheme.textMain,
                              ),
                            ),
                          ),
                          const SizedBox(width: 6),
                          Text(_selectedNode.flag),
                        ],
                      ),
                    ),
                  ],
                ),
              ),

              // Interactive Cyber Grid Map Canvas
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Stack(
                    children: [
                      // World Network Grid Backdrop Card
                      Container(
                        width: double.infinity,
                        height: double.infinity,
                        decoration: AppTheme.glassCard(radius: 20),
                        child: CustomPaint(
                          painter: _CyberMapPainter(
                            isConnected: isConnected,
                            pulseProgress: _pulseAnimation.value,
                          ),
                        ),
                      ),

                      // Floating Top Controls (Search & Filter)
                      Positioned(
                        top: 16,
                        left: 16,
                        right: 16,
                        child: Wrap(
                          alignment: WrapAlignment.spaceBetween,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          spacing: 10,
                          runSpacing: 8,
                          children: [
                            InkWell(
                              onTap: () => _showServerSelector(context),
                              borderRadius: BorderRadius.circular(12),
                              child: Container(
                                padding: const EdgeInsets.symmetric(
                                    horizontal: 14, vertical: 10),
                                decoration: BoxDecoration(
                                  color: AppTheme.surfaceLow.withValues(alpha: 0.85),
                                  borderRadius: BorderRadius.circular(12),
                                  border: Border.all(color: AppTheme.borderSubtle),
                                ),
                                child: Row(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    const Icon(
                                      Icons.search,
                                      size: 16,
                                      color: AppTheme.textMuted,
                                    ),
                                    const SizedBox(width: 8),
                                    Text(
                                      'Node: ${_selectedNode.city} (${_selectedNode.code})',
                                      style: const TextStyle(
                                        fontSize: 12,
                                        color: AppTheme.textMain,
                                        fontWeight: FontWeight.w500,
                                      ),
                                    ),
                                    const SizedBox(width: 8),
                                    const Icon(
                                      Icons.arrow_drop_down,
                                      size: 16,
                                      color: AppTheme.textMuted,
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            Wrap(
                              spacing: 6,
                              runSpacing: 6,
                              children: [
                                _buildFilterChip('P2P Mesh', true),
                                _buildFilterChip('Onion Circuits', true),
                                _buildFilterChip('Tor Bridges', false),
                              ],
                            ),
                          ],
                        ),
                      ),

                      // Central Tactical Hero Quick Connect
                      Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            GestureDetector(
                              onTap: () => _handleConnectToggle(
                                  isConnected, isConnecting),
                              child: AnimatedBuilder(
                                animation: _pulseAnimation,
                                builder: (context, child) {
                                  return Container(
                                    width: 170,
                                    height: 170,
                                    decoration: BoxDecoration(
                                      shape: BoxShape.circle,
                                      color: AppTheme.surfaceContainer,
                                      border: Border.all(
                                        color: isConnected
                                            ? AppTheme.primary
                                            : isConnecting
                                                ? AppTheme.warning
                                                : AppTheme.borderSubtle,
                                        width: 2.5,
                                      ),
                                      boxShadow: isConnected
                                          ? [
                                              BoxShadow(
                                                color: AppTheme.primary
                                                    .withValues(alpha: 0.35),
                                                blurRadius: 36 *
                                                    _pulseAnimation.value,
                                                spreadRadius: 8 *
                                                    _pulseAnimation.value,
                                              )
                                            ]
                                          : null,
                                    ),
                                    child: Center(
                                      child: Column(
                                        mainAxisSize: MainAxisSize.min,
                                        children: [
                                          Icon(
                                            isConnected
                                                ? Icons.shield
                                                : Icons.power_settings_new,
                                            size: 58,
                                            color: isConnected
                                                ? AppTheme.primary
                                                : isConnecting
                                                    ? AppTheme.warning
                                                    : AppTheme.textMuted,
                                          ),
                                          const SizedBox(height: 6),
                                          Text(
                                            isConnected
                                                ? 'DISCONNECT'
                                                : isConnecting
                                                    ? 'WAITING…'
                                                    : 'CONNECT',
                                            style: TextStyle(
                                              fontSize: 11,
                                              fontWeight: FontWeight.bold,
                                              letterSpacing: 2,
                                              color: isConnected
                                                  ? AppTheme.primary
                                                  : isConnecting
                                                      ? AppTheme.warning
                                                      : AppTheme.textMuted,
                                            ),
                                          ),
                                        ],
                                      ),
                                    ),
                                  );
                                },
                              ),
                            ),
                            const SizedBox(height: 16),
                            Container(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 16, vertical: 8),
                              decoration: BoxDecoration(
                                color: AppTheme.surfaceHigh.withValues(alpha: 0.9),
                                borderRadius: BorderRadius.circular(20),
                                border: Border.all(
                                  color: isConnected
                                      ? AppTheme.primary.withValues(alpha: 0.4)
                                      : AppTheme.borderSubtle,
                                ),
                              ),
                              child: Text(
                                isConnected
                                    ? 'Exit Node: ${_selectedNode.city}, ${_selectedNode.country} (${_selectedNode.code})'
                                    : 'Select a route and press connect',
                                style: const TextStyle(
                                  fontSize: 12,
                                  color: AppTheme.textMain,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),

              // Bottom Quick Controls Bar
              Padding(
                padding: const EdgeInsets.fromLTRB(24, 0, 24, 24),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                  decoration: AppTheme.glassCard(radius: 16),
                  child: Wrap(
                    alignment: WrapAlignment.spaceBetween,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    spacing: 12,
                    runSpacing: 10,
                    children: [
                      Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          ElevatedButton.icon(
                            style: ElevatedButton.styleFrom(
                              backgroundColor: isConnected
                                  ? AppTheme.error.withValues(alpha: 0.2)
                                  : AppTheme.primary,
                              foregroundColor: isConnected
                                  ? AppTheme.error
                                  : const Color(0xFF003824),
                              side: BorderSide(
                                color: isConnected
                                    ? AppTheme.error.withValues(alpha: 0.4)
                                    : Colors.transparent,
                              ),
                            ),
                            onPressed: () => _handleConnectToggle(
                                isConnected, isConnecting),
                            icon: Icon(
                              Icons.power_settings_new,
                              size: 16,
                              color: isConnected
                                  ? AppTheme.error
                                  : const Color(0xFF003824),
                            ),
                            label: Text(isConnected ? 'Disconnect' : 'Connect'),
                          ),
                          const SizedBox(width: 12),
                          OutlinedButton.icon(
                            style: OutlinedButton.styleFrom(
                              foregroundColor: AppTheme.textMain,
                              side: const BorderSide(color: AppTheme.borderSubtle),
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 16, vertical: 14),
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(12),
                              ),
                            ),
                            onPressed: () => _showServerSelector(context),
                            icon: const Icon(
                              Icons.travel_explore,
                              size: 16,
                              color: AppTheme.secondary,
                            ),
                            label: const Text('Change Server Node'),
                          ),
                        ],
                      ),
                      Wrap(
                        spacing: 10,
                        runSpacing: 6,
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 12, vertical: 8),
                            decoration: BoxDecoration(
                              color: AppTheme.surfaceContainer,
                              borderRadius: BorderRadius.circular(10),
                              border: Border.all(color: AppTheme.borderSubtle),
                            ),
                            child: const Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(
                                  Icons.verified_user,
                                  size: 14,
                                  color: AppTheme.primary,
                                ),
                                SizedBox(width: 6),
                                Text(
                                  'ShadowLink Onion v2 (X25519 + ChaCha20)',
                                  style: TextStyle(
                                    fontSize: 11,
                                    color: AppTheme.textMain,
                                    fontWeight: FontWeight.w500,
                                  ),
                                ),
                              ],
                            ),
                          ),
                          Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 12, vertical: 8),
                            decoration: BoxDecoration(
                              color: AppTheme.surfaceContainer,
                              borderRadius: BorderRadius.circular(10),
                              border: Border.all(color: AppTheme.borderSubtle),
                            ),
                            child: const Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(
                                  Icons.lan,
                                  size: 14,
                                  color: AppTheme.secondary,
                                ),
                                SizedBox(width: 6),
                                Text(
                                  'SOCKS5: 127.0.0.1:1080',
                                  style: TextStyle(
                                    fontSize: 11,
                                    color: AppTheme.secondary,
                                    fontFamily: 'monospace',
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),

        const VerticalDivider(width: 1),

        // ── Column 3: Right Inspector (340px) ─────────────────────────────────
        SizedBox(
          width: 340,
          child: Container(
            color: AppTheme.surfaceLow,
            child: ListView(
              padding: const EdgeInsets.all(20),
              children: [
                _buildCircuitTopologyCard(isConnected),
                const SizedBox(height: 16),
                _buildLiveTelemetryGrid(isConnected),
                const SizedBox(height: 16),
                _buildNodeOperatorCard(isConnected),
              ],
            ),
          ),
        ),
      ],
    );
  }

  // ═════════════════════════════════════════════════════════════════════════════
  // MOBILE LAYOUT (Compact Portrait Proton/ExpressVPN Style)
  // ═════════════════════════════════════════════════════════════════════════════
  Widget _buildMobileLayout(
      bool isConnected, bool isConnecting, bool isError) {
    return SafeArea(
      child: ListView(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
        children: [
          // Mobile Top Bar
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Row(
                  children: [
                    Container(
                      width: 34,
                      height: 34,
                      decoration: BoxDecoration(
                        color: AppTheme.surfaceHigh,
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(
                          color: AppTheme.primary.withValues(alpha: 0.3),
                        ),
                      ),
                      child: const Icon(
                        Icons.shield,
                        color: AppTheme.primary,
                        size: 20,
                      ),
                    ),
                    const SizedBox(width: 10),
                    const Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'ShadowLink dVPN',
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.bold,
                              color: AppTheme.textMain,
                            ),
                          ),
                          Text(
                            'Decentralized Mesh',
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 10,
                              color: AppTheme.primary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: AppTheme.surfaceHigh,
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: AppTheme.borderSubtle),
                    ),
                    child: Row(
                      children: [
                        Container(
                          width: 6,
                          height: 6,
                          decoration: const BoxDecoration(
                            shape: BoxShape.circle,
                            color: AppTheme.primary,
                          ),
                        ),
                        const SizedBox(width: 6),
                        Text(
                          '${_selectedNode.pingMs}ms',
                          style: const TextStyle(
                            fontSize: 11,
                            color: AppTheme.textMuted,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.settings, color: AppTheme.textMuted),
                    onPressed: () {},
                  ),
                ],
              ),
            ],
          ),

          const SizedBox(height: 24),

          // Central Tactile Connect Button with Radar Rings
          Center(
            child: Stack(
              alignment: Alignment.center,
              children: [
                if (isConnected)
                  AnimatedBuilder(
                    animation: _pulseAnimation,
                    builder: (context, child) {
                      return Container(
                        width: 190 * _pulseAnimation.value,
                        height: 190 * _pulseAnimation.value,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: AppTheme.primary.withValues(
                              alpha: (1.0 - _pulseAnimation.value) * 0.5,
                            ),
                            width: 1.5,
                          ),
                        ),
                      );
                    },
                  ),
                GestureDetector(
                  onTap: () =>
                      _handleConnectToggle(isConnected, isConnecting),
                  child: Container(
                    width: 160,
                    height: 160,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: AppTheme.surfaceContainer,
                      border: Border.all(
                        color: isConnected
                            ? AppTheme.primary
                            : isConnecting
                                ? AppTheme.warning
                                : AppTheme.borderSubtle,
                        width: 2.5,
                      ),
                      boxShadow: isConnected
                          ? [
                              BoxShadow(
                                color: AppTheme.primary.withValues(alpha: 0.3),
                                blurRadius: 28,
                                spreadRadius: 4,
                              )
                            ]
                          : null,
                    ),
                    child: Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            isConnected
                                ? Icons.shield
                                : Icons.power_settings_new,
                            size: 54,
                            color: isConnected
                                ? AppTheme.primary
                                : isConnecting
                                    ? AppTheme.warning
                                    : AppTheme.textMuted,
                          ),
                          const SizedBox(height: 6),
                          Text(
                            isConnected
                                ? 'DISCONNECT'
                                : isConnecting
                                    ? 'CONNECTING…'
                                    : 'CONNECT',
                            style: TextStyle(
                              fontSize: 11,
                              fontWeight: FontWeight.bold,
                              letterSpacing: 1.8,
                              color: isConnected
                                  ? AppTheme.primary
                                  : isConnecting
                                      ? AppTheme.warning
                                      : AppTheme.textMuted,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),

          const SizedBox(height: 16),

          // Connection State Pill
          Center(
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
              decoration: BoxDecoration(
                color: isConnected
                    ? AppTheme.primary.withValues(alpha: 0.15)
                    : isConnecting
                        ? AppTheme.warning.withValues(alpha: 0.15)
                        : AppTheme.surfaceHigh,
                borderRadius: BorderRadius.circular(20),
                border: Border.all(
                  color: isConnected
                      ? AppTheme.primary.withValues(alpha: 0.4)
                      : AppTheme.borderSubtle,
                ),
              ),
              child: Text(
                isConnected
                    ? 'CONNECTED · SECURE'
                    : isConnecting
                        ? 'CONNECTING TO ONION CIRCUIT…'
                        : 'DISCONNECTED',
                style: TextStyle(
                  color: isConnected
                      ? AppTheme.primary
                      : isConnecting
                          ? AppTheme.warning
                          : AppTheme.textMuted,
                  fontSize: 11,
                  fontWeight: FontWeight.bold,
                  letterSpacing: 1,
                ),
              ),
            ),
          ),

          const SizedBox(height: 20),

          // Virtual Location Card
          InkWell(
            onTap: () => _showServerSelector(context),
            borderRadius: BorderRadius.circular(14),
            child: Container(
              padding: const EdgeInsets.all(14),
              decoration: AppTheme.glassCard(radius: 14),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Row(
                      children: [
                        Container(
                          width: 38,
                          height: 38,
                          decoration: BoxDecoration(
                            color: AppTheme.surfaceHigh,
                            borderRadius: BorderRadius.circular(8),
                          ),
                          child: Center(
                            child: Text(
                              _selectedNode.flag,
                              style: const TextStyle(fontSize: 20),
                            ),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '${_selectedNode.city}, ${_selectedNode.country}',
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontWeight: FontWeight.bold,
                                  fontSize: 14,
                                  color: AppTheme.textMain,
                                ),
                              ),
                              Text(
                                '${_selectedNode.ip} · ${_selectedNode.pingMs}ms',
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontSize: 11,
                                  color: AppTheme.secondary,
                                  fontFamily: 'monospace',
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),
                  TextButton(
                    onPressed: () => _showServerSelector(context),
                    style: TextButton.styleFrom(
                      foregroundColor: AppTheme.primary,
                    ),
                    child: const Text('Switch ⇄'),
                  ),
                ],
              ),
            ),
          ),

          const SizedBox(height: 16),

          // 3-Hop Onion Circuit Topology Card
          _buildCircuitTopologyCard(isConnected),

          const SizedBox(height: 16),

          // Live Telemetry Speed Grid
          _buildLiveTelemetryGrid(isConnected),

          const SizedBox(height: 16),

          // Cryptographic Defenses Card
          _buildSecurityDefensesCard(),

          const SizedBox(height: 16),

          // Mobile Role Toggles Card
          _buildRoleTogglesCard(isConnected),

          const SizedBox(height: 20),
        ],
      ),
    );
  }

  // ═════════════════════════════════════════════════════════════════════════════
  // REUSABLE DASHBOARD CARDS & COMPONENTS
  // ═════════════════════════════════════════════════════════════════════════════

  Widget _buildCircuitTopologyCard(bool isConnected) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.glassCard(radius: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Expanded(
                child: Row(
                  children: [
                    Icon(Icons.alt_route, size: 16, color: AppTheme.secondary),
                    SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        'Multi-Hop Onion Route',
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.bold,
                          letterSpacing: 0.5,
                          color: AppTheme.textMain,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: AppTheme.surfaceHigh,
                  borderRadius: BorderRadius.circular(4),
                  border: Border.all(
                    color: AppTheme.secondary.withValues(alpha: 0.3),
                  ),
                ),
                child: const Text(
                  'ChaCha20-Poly1305',
                  style: TextStyle(
                    fontSize: 9,
                    color: AppTheme.secondary,
                    fontFamily: 'monospace',
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              _buildHopNode(
                icon: Icons.smartphone,
                label: 'Your Device',
                sublabel: 'IP: Masked',
                color: isConnected ? AppTheme.primary : AppTheme.textMuted,
              ),
              Expanded(
                child: Container(
                  height: 2,
                  color: isConnected
                      ? AppTheme.primary.withValues(alpha: 0.6)
                      : AppTheme.borderSubtle,
                ),
              ),
              _buildHopNode(
                icon: Icons.dns,
                label: 'Relay Node',
                sublabel: 'Frankfurt DE',
                color: isConnected ? AppTheme.secondary : AppTheme.textMuted,
              ),
              Expanded(
                child: Container(
                  height: 2,
                  color: isConnected
                      ? AppTheme.secondary.withValues(alpha: 0.6)
                      : AppTheme.borderSubtle,
                ),
              ),
              _buildHopNode(
                icon: Icons.vpn_lock,
                label: 'Exit Egress',
                sublabel: '${_selectedNode.city} ${_selectedNode.code}',
                color: isConnected ? AppTheme.primary : AppTheme.textMuted,
              ),
            ],
          ),
          const SizedBox(height: 12),
          const Divider(),
          const SizedBox(height: 6),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Expanded(
                child: Text(
                  'Zero-Knowledge Layered Tunnel',
                  style: TextStyle(fontSize: 10, color: AppTheme.textMuted),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: 8),
              const Text(
                '3 Encrypted Hops Active',
                style: TextStyle(
                  fontSize: 10,
                  color: AppTheme.primary,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildHopNode({
    required IconData icon,
    required String label,
    required String sublabel,
    required Color color,
  }) {
    return SizedBox(
      width: 76,
      child: Column(
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(
              color: AppTheme.surfaceHigh,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: color.withValues(alpha: 0.5)),
            ),
            child: Icon(icon, size: 16, color: color),
          ),
          const SizedBox(height: 4),
          Text(
            label,
            textAlign: TextAlign.center,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.w600,
              color: AppTheme.textMain,
            ),
          ),
          Text(
            sublabel,
            textAlign: TextAlign.center,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 8, color: color),
          ),
        ],
      ),
    );
  }

  Widget _buildLiveTelemetryGrid(bool isConnected) {
    return Column(
      children: [
        Row(
          children: [
            Expanded(
              child: _buildTelemetryMetric(
                icon: Icons.arrow_downward,
                label: 'Download',
                value: isConnected ? '84.2' : '0.0',
                unit: 'Mbps',
                color: AppTheme.secondary,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _buildTelemetryMetric(
                icon: Icons.arrow_upward,
                label: 'Upload',
                value: isConnected ? '38.6' : '0.0',
                unit: 'Mbps',
                color: AppTheme.primary,
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: _buildTelemetryMetric(
                icon: Icons.timer,
                label: 'Session',
                value: isConnected ? _formatUptime(_sessionSeconds) : '00:00:00',
                unit: 'Live',
                color: AppTheme.textMain,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _buildTelemetryMetric(
                icon: Icons.data_usage,
                label: 'Transferred',
                value: isConnected ? '1.4' : '0.0',
                unit: 'GB',
                color: AppTheme.textMain,
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildTelemetryMetric({
    required IconData icon,
    required String label,
    required String value,
    required String unit,
    required Color color,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: AppTheme.glassCard(radius: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 13, color: color),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  label,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 10,
                    color: AppTheme.textMuted,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Wrap(
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                value,
                style: const TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.bold,
                  fontFamily: 'monospace',
                  color: AppTheme.textMain,
                ),
              ),
              const SizedBox(width: 4),
              Text(
                unit,
                style: const TextStyle(
                  fontSize: 10,
                  color: AppTheme.textMuted,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildSecurityDefensesCard() {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: AppTheme.glassCard(radius: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Active Cryptographic Defenses',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.bold,
              color: AppTheme.textMuted,
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: _buildDefenseBadge(
                  icon: Icons.verified_user,
                  title: 'Kill Switch',
                  status: _killSwitchActive ? 'Active' : 'Off',
                  color: _killSwitchActive ? AppTheme.primary : AppTheme.textMuted,
                  onTap: () => setState(
                      () => _killSwitchActive = !_killSwitchActive),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _buildDefenseBadge(
                  icon: Icons.lock,
                  title: 'DNS Shield',
                  status: _dnsLeakProtected ? 'Protected' : 'Off',
                  color: _dnsLeakProtected ? AppTheme.secondary : AppTheme.textMuted,
                  onTap: () => setState(
                      () => _dnsLeakProtected = !_dnsLeakProtected),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _buildDefenseBadge(
                  icon: Icons.security,
                  title: 'Protocol',
                  status: 'Onion v2',
                  color: AppTheme.primaryBright,
                  onTap: null,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildDefenseBadge({
    required IconData icon,
    required String title,
    required String status,
    required Color color,
    required VoidCallback? onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 6),
        decoration: BoxDecoration(
          color: AppTheme.surfaceHigh,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: color.withValues(alpha: 0.3)),
        ),
        child: Column(
          children: [
            Icon(icon, size: 18, color: color),
            const SizedBox(height: 4),
            Text(
              title,
              style: const TextStyle(
                fontSize: 10,
                fontWeight: FontWeight.w600,
                color: AppTheme.textMain,
              ),
            ),
            Text(
              status,
              style: TextStyle(
                fontSize: 9,
                color: color,
                fontWeight: FontWeight.bold,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildNodeOperatorCard(bool isConnected) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.glassCard(radius: 16),
      child: Material(
        type: MaterialType.transparency,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Expanded(
                  child: Row(
                    children: [
                      Icon(Icons.dns, size: 16, color: AppTheme.primary),
                      SizedBox(width: 6),
                      Expanded(
                        child: Text(
                          'Node Operator Controls',
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 13,
                            fontWeight: FontWeight.bold,
                            color: AppTheme.textMain,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: AppTheme.secondary.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: const Text(
                    'DePIN',
                    style: TextStyle(
                      fontSize: 9,
                      color: AppTheme.secondary,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: const Text(
                'Contribute Bandwidth (Relay)',
                style: TextStyle(fontSize: 13, color: AppTheme.textMain),
              ),
              subtitle: const Text(
                'Earn +4.2 SHDW/hr routing for others',
                style: TextStyle(fontSize: 11, color: AppTheme.primary),
              ),
              value: _daemon.isRelay,
              onChanged: isConnected ? null : _setRelay,
            ),
            if (!Platform.isAndroid && !Platform.isIOS)
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text(
                  'Exit Node Operator',
                  style: TextStyle(fontSize: 13, color: AppTheme.error),
                ),
                subtitle: const Text(
                  'WARNING: Internet traffic exits your IP',
                  style: TextStyle(fontSize: 11, color: AppTheme.textMuted),
                ),
                value: _daemon.isExit,
                onChanged: isConnected ? null : _setExit,
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildRoleTogglesCard(bool isConnected) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: AppTheme.glassCard(radius: 14),
      child: Material(
        type: MaterialType.transparency,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Node Mode & Routing Roles',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.bold,
                color: AppTheme.textMuted,
              ),
            ),
            const SizedBox(height: 6),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: const Text(
                'Client Mode (Entry Node)',
                style: TextStyle(fontSize: 13, color: AppTheme.textMain),
              ),
              subtitle: const Text(
                'Routes your local traffic through onion circuit',
                style: TextStyle(fontSize: 11, color: AppTheme.textMuted),
              ),
              value: _daemon.isEntry,
              onChanged: isConnected
                  ? null
                  : (!_daemon.isRelay && !_daemon.isExit)
                      ? null
                      : _setEntry,
            ),
            if (!Platform.isAndroid && !Platform.isIOS) ...[
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text(
                  'Relay Mode (Contribute)',
                  style: TextStyle(fontSize: 13, color: AppTheme.textMain),
                ),
                subtitle: const Text(
                  'Help encrypt and route traffic in P2P mesh',
                  style: TextStyle(fontSize: 11, color: AppTheme.textMuted),
                ),
                value: _daemon.isRelay,
                onChanged: isConnected ? null : _setRelay,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildNavItem({
    required IconData icon,
    required String title,
    required bool isSelected,
    String? badgeText,
    VoidCallback? onTap,
  }) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Material(
        color: Colors.transparent,
        child: ListTile(
          onTap: onTap,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          tileColor: isSelected ? AppTheme.surfaceHigh : Colors.transparent,
          leading: Icon(
            icon,
            color: isSelected ? AppTheme.primary : AppTheme.textMuted,
            size: 20,
          ),
          title: Text(
            title,
            style: TextStyle(
              fontSize: 13,
              fontWeight: isSelected ? FontWeight.bold : FontWeight.w500,
              color: isSelected ? AppTheme.primary : AppTheme.textMain,
            ),
          ),
          trailing: badgeText != null
              ? Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: AppTheme.secondary.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    badgeText,
                    style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.bold,
                      color: AppTheme.secondary,
                    ),
                  ),
                )
              : null,
        ),
      ),
    );
  }

  Widget _buildFilterChip(String label, bool isSelected) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: isSelected
            ? AppTheme.primary.withValues(alpha: 0.15)
            : AppTheme.surfaceContainer,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isSelected
              ? AppTheme.primary.withValues(alpha: 0.4)
              : AppTheme.borderSubtle,
        ),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          color: isSelected ? AppTheme.primary : AppTheme.textMuted,
          fontWeight: isSelected ? FontWeight.bold : FontWeight.w500,
        ),
      ),
    );
  }

  Widget _buildMobileBottomNav() {
    return Container(
      decoration: const BoxDecoration(
        color: AppTheme.surfaceLow,
        border: Border(top: BorderSide(color: AppTheme.borderSubtle)),
      ),
      child: BottomNavigationBar(
        currentIndex: _bottomNavIndex,
        onTap: (index) {
          setState(() => _bottomNavIndex = index);
          if (index == 1) {
            _showServerSelector(context);
          }
        },
        backgroundColor: Colors.transparent,
        elevation: 0,
        type: BottomNavigationBarType.fixed,
        selectedItemColor: AppTheme.primary,
        unselectedItemColor: AppTheme.textMuted,
        selectedFontSize: 11,
        unselectedFontSize: 11,
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.shield),
            label: 'Shield',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.hub),
            label: 'Nodes',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.insights),
            label: 'Traffic',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.tune),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}

// ═════════════════════════════════════════════════════════════════════════════
// CUSTOM CYBER MAP PAINTER (Interactive Mesh Map Canvas)
// ═════════════════════════════════════════════════════════════════════════════
class _CyberMapPainter extends CustomPainter {
  final bool isConnected;
  final double pulseProgress;

  _CyberMapPainter({
    required this.isConnected,
    required this.pulseProgress,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final gridPaint = Paint()
      ..color = Colors.white.withValues(alpha: 0.035)
      ..strokeWidth = 1.0;

    // Draw cyber grid lines
    const step = 32.0;
    for (double x = 0; x < size.width; x += step) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), gridPaint);
    }
    for (double y = 0; y < size.height; y += step) {
      canvas.drawLine(Offset(0, y), Offset(size.width, y), gridPaint);
    }

    if (isConnected) {
      // 3 Multi-hop node positions
      final clientNode = Offset(size.width * 0.28, size.height * 0.45);
      final relayNode = Offset(size.width * 0.48, size.height * 0.62);
      final exitNode = Offset(size.width * 0.72, size.height * 0.38);

      final linePaint = Paint()
        ..color = AppTheme.secondary.withValues(alpha: 0.5)
        ..strokeWidth = 2.0
        ..style = PaintingStyle.stroke;

      final path = Path()
        ..moveTo(clientNode.dx, clientNode.dy)
        ..quadraticBezierTo(
          (clientNode.dx + relayNode.dx) / 2,
          relayNode.dy + 20,
          relayNode.dx,
          relayNode.dy,
        )
        ..quadraticBezierTo(
          (relayNode.dx + exitNode.dx) / 2,
          exitNode.dy - 20,
          exitNode.dx,
          exitNode.dy,
        );

      canvas.drawPath(path, linePaint);

      // Draw node beacons
      _drawNodeBeacon(canvas, clientNode, AppTheme.secondary, 'Client (0ms)');
      _drawNodeBeacon(canvas, relayNode, AppTheme.primary, 'Frankfurt DE (18ms)');
      _drawNodeBeacon(canvas, exitNode, AppTheme.primary, 'Zurich CH (14ms)');
    }
  }

  void _drawNodeBeacon(
      Canvas canvas, Offset offset, Color color, String label) {
    // Outer pulse ring
    final pulsePaint = Paint()
      ..color = color.withValues(alpha: (1.0 - pulseProgress) * 0.4)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    canvas.drawCircle(offset, 12 * pulseProgress, pulsePaint);

    // Inner core
    final corePaint = Paint()..color = color;
    canvas.drawCircle(offset, 5, corePaint);

    final borderPaint = Paint()
      ..color = Colors.white
      ..strokeWidth = 1.5
      ..style = PaintingStyle.stroke;
    canvas.drawCircle(offset, 5, borderPaint);
  }

  @override
  bool shouldRepaint(covariant _CyberMapPainter oldDelegate) {
    return oldDelegate.isConnected != isConnected ||
        oldDelegate.pulseProgress != pulseProgress;
  }
}
