import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

import '../config/app_config.dart';
import '../theme/app_theme.dart';
import 'dashboard_screen.dart';

class EulaScreen extends StatefulWidget {
  const EulaScreen({super.key});

  @override
  State<EulaScreen> createState() => _EulaScreenState();
}

class _EulaScreenState extends State<EulaScreen> {
  bool _accepted = false;
  String _eulaText = 'Loading protocol terms…';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      await Future.wait([
        _loadEulaText(),
        _checkExistingEula(),
      ]);
    });
  }

  /// Loads the EULA text from the bundled asset (kTermsAssetKey).
  Future<void> _loadEulaText() async {
    try {
      final text = await rootBundle.loadString(kTermsAssetKey);
      if (mounted) setState(() => _eulaText = text);
    } catch (_) {
      if (mounted) {
        setState(() => _eulaText =
            'SHADOWLINK ZERO-KNOWLEDGE PROTOCOL TERMS:\n\n'
            '1. NO CENTRAL LOGS: ShadowLink operates exclusively via peer-to-peer\n'
            '   cryptographic onion circuits. No telemetry or connection logs exist.\n\n'
            '2. OPERATOR RESPONSIBILITY: Running an exit node routes traffic from other peers\n'
            '   through your IP. You assume full legal responsibility for chosen node roles.\n\n'
            '3. DECENTRALIZED PROTOCOL: The software is open-source and provided as-is\n'
            '   without warranty of any kind.');
      }
    }
  }

  /// Skips the EULA screen if the user accepted in a prior session.
  Future<void> _checkExistingEula() async {
    final eulaPath = await _getEulaPath();
    if (await File(eulaPath).exists()) {
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const DashboardScreen()),
      );
    }
  }

  Future<void> _acceptEula() async {
    final eulaPath = await _getEulaPath();
    await File(eulaPath).writeAsString('accepted');
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const DashboardScreen()),
    );
  }

  /// Returns a writable path for the EULA acceptance file.
  Future<String> _getEulaPath() async {
    if (Platform.isAndroid || Platform.isIOS) {
      final dir = await getApplicationDocumentsDirectory();
      return p.join(dir.path, kEulaFileName);
    }
    final execDir = File(Platform.resolvedExecutable).parent.path;
    return p.join(execDir, kEulaFileName);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppTheme.background,
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 680),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Cyber Header
                  Row(
                    children: [
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(
                          color: AppTheme.surfaceHigh,
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(
                            color: AppTheme.primary.withValues(alpha: 0.4),
                          ),
                          boxShadow: [
                            BoxShadow(
                              color: AppTheme.primary.withValues(alpha: 0.2),
                              blurRadius: 16,
                            ),
                          ],
                        ),
                        child: const Icon(
                          Icons.shield,
                          color: AppTheme.primary,
                          size: 26,
                        ),
                      ),
                      const SizedBox(width: 14),
                      const Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'SHADOWLINK PROTOCOL',
                              style: TextStyle(
                                color: AppTheme.textMain,
                                fontSize: 18,
                                fontWeight: FontWeight.bold,
                                letterSpacing: 1.5,
                              ),
                            ),
                            Text(
                              'End User Terms & Decentralized Sovereign Agreement',
                              style: TextStyle(
                                color: AppTheme.textMuted,
                                fontSize: 12,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),

                  const SizedBox(height: 20),

                  // Warning Card
                  Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 14, vertical: 10),
                    decoration: BoxDecoration(
                      color: AppTheme.error.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(
                        color: AppTheme.error.withValues(alpha: 0.35),
                      ),
                    ),
                    child: const Row(
                      children: [
                        Icon(Icons.warning_amber_rounded,
                            color: AppTheme.error, size: 20),
                        SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            'P2P dVPN: Zero central authority. You assume full legal responsibility for personal traffic & node roles.',
                            style: TextStyle(
                              color: Color(0xFFFFB4AB),
                              fontSize: 12,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),

                  const SizedBox(height: 16),

                  // EULA Body Container (Frosted Glass Card)
                  Expanded(
                    child: Container(
                      padding: const EdgeInsets.all(18),
                      decoration: AppTheme.glassCard(radius: 14),
                      child: Scrollbar(
                        thumbVisibility: true,
                        child: SingleChildScrollView(
                          child: Text(
                            _eulaText,
                            style: const TextStyle(
                              color: AppTheme.textMuted,
                              fontFamily: 'monospace',
                              fontSize: 12,
                              height: 1.6,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),

                  const SizedBox(height: 16),

                  // Checkbox Acceptance
                  InkWell(
                    onTap: () => setState(() => _accepted = !_accepted),
                    borderRadius: BorderRadius.circular(8),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(vertical: 4),
                      child: Row(
                        children: [
                          Checkbox(
                            value: _accepted,
                            activeColor: AppTheme.primary,
                            checkColor: const Color(0xFF003824),
                            side: const BorderSide(color: AppTheme.borderSubtle),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(4),
                            ),
                            onChanged: (val) =>
                                setState(() => _accepted = val ?? false),
                          ),
                          const Expanded(
                            child: Text(
                              'I have read and agree to the Terms of Service & Sovereign EULA.',
                              style: TextStyle(
                                color: AppTheme.textMain,
                                fontSize: 13,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),

                  const SizedBox(height: 12),

                  // Action Button
                  SizedBox(
                    width: double.infinity,
                    height: 48,
                    child: ElevatedButton(
                      onPressed: _accepted ? _acceptEula : null,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: _accepted
                            ? AppTheme.primary
                            : AppTheme.surfaceHigh,
                        foregroundColor: _accepted
                            ? const Color(0xFF003824)
                            : AppTheme.textMuted,
                        disabledBackgroundColor: AppTheme.surfaceHigh,
                        disabledForegroundColor: AppTheme.textMuted,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                          side: BorderSide(
                            color: _accepted
                                ? AppTheme.primary
                                : AppTheme.borderSubtle,
                          ),
                        ),
                      ),
                      child: const Text(
                        'INITIALIZE SHADOWLINK CLIENT',
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.bold,
                          letterSpacing: 1.2,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
