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
  // ignore: library_private_types_in_public_api
  _EulaScreenState createState() => _EulaScreenState();
}

class _EulaScreenState extends State<EulaScreen> {
  bool _accepted = false;
  String _eulaText = 'Loading terms…';

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
  /// Falls back to a short notice if the asset cannot be read.
  Future<void> _loadEulaText() async {
    try {
      // Asset key sourced from app_config — not a hardcoded string literal.
      final text = await rootBundle.loadString(kTermsAssetKey);
      if (mounted) setState(() => _eulaText = text);
    } catch (_) {
      if (mounted) {
        setState(() => _eulaText =
            'Could not load TERMS_AND_CONDITIONS.md.\n'
            'Please read the file in the repository root before proceeding.');
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
  /// File name sourced from [kEulaFileName] in app_config.
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
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'SHADOWLINK EULA',
                style: TextStyle(
                  color: AppTheme.error,
                  fontSize: 28,
                  fontWeight: FontWeight.bold,
                  letterSpacing: 2,
                ),
              ),
              const SizedBox(height: 16),
              const Text(
                'WARNING: By using this software, you legally bind yourself '
                'to the absolute terms of service.',
                style: TextStyle(color: AppTheme.textMain, fontSize: 16),
              ),
              const SizedBox(height: 24),
              Expanded(
                child: Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: AppTheme.surface,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: AppTheme.textMuted.withValues(alpha: 0.3),
                    ),
                  ),
                  child: SingleChildScrollView(
                    child: Text(
                      _eulaText,
                      style: const TextStyle(
                          color: AppTheme.textMuted, height: 1.5),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 24),
              Row(
                children: [
                  Checkbox(
                    value: _accepted,
                    activeColor: AppTheme.error,
                    onChanged: (val) => setState(() => _accepted = val ?? false),
                  ),
                  const Expanded(
                    child: Text(
                      'I accept 100% of the legal risk and absolute liability.',
                      style: TextStyle(color: AppTheme.textMain),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 24),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _accepted ? _acceptEula : null,
                  style: ElevatedButton.styleFrom(
                    backgroundColor:
                        _accepted ? AppTheme.error : AppTheme.surface,
                    foregroundColor: Colors.white,
                  ),
                  child: const Text('I ACCEPT'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
