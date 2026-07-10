import 'dart:io';
import 'package:flutter/material.dart';
import '../theme/app_theme.dart';
import 'dashboard_screen.dart';

class EulaScreen extends StatefulWidget {
  const EulaScreen({Key? key}) : super(key: key);

  @override
  _EulaScreenState createState() => _EulaScreenState();
}

class _EulaScreenState extends State<EulaScreen> {
  bool _accepted = false;

  @override
  void initState() {
    super.initState();
    _checkExistingEula();
  }

  Future<void> _checkExistingEula() async {
    final file = File('../.shadowlink_accepted');
    if (await file.exists()) {
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const DashboardScreen()),
      );
    }
  }

  Future<void> _acceptEula() async {
    final file = File('../.shadowlink_accepted');
    await file.writeAsString('accepted');
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const DashboardScreen()),
    );
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
                "SHADOWLINK EULA",
                style: TextStyle(
                  color: AppTheme.error,
                  fontSize: 28,
                  fontWeight: FontWeight.bold,
                  letterSpacing: 2,
                ),
              ),
              const SizedBox(height: 16),
              const Text(
                "WARNING: By using this software, you legally bind yourself to the absolute terms of service.",
                style: TextStyle(color: AppTheme.textMain, fontSize: 16),
              ),
              const SizedBox(height: 24),
              Expanded(
                child: Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: AppTheme.surface,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: AppTheme.textMuted.withOpacity(0.3)),
                  ),
                  child: const SingleChildScrollView(
                    child: Text(
                      "1. NO INFRASTRUCTURE: ShadowLink is an open-source protocol. The developers operate NO network servers and have NO control over peer-to-peer traffic.\n\n"
                      "2. ZERO LIABILITY: The developers assume ABSOLUTELY ZERO LIABILITY for any damages, legal repercussions, or network traffic.\n\n"
                      "3. COMPLIANCE: You assume 100% of the legal risk. You agree NOT to use this software to violate any laws.\n\n"
                      "4. EXIT NODES: Running an Exit Node exposes your IP to third-party traffic. You do so ENTIRELY at your own risk.\n\n"
                      "5. NO DUTY OF CARE: The developers owe you no duty of care, equitable duty, or other legal obligation worldwide.\n\n"
                      "By clicking Accept, you acknowledge that you have read the full TERMS_AND_CONDITIONS.md file in the root directory.",
                      style: TextStyle(color: AppTheme.textMuted, height: 1.5),
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
                    onChanged: (val) {
                      setState(() {
                        _accepted = val ?? false;
                      });
                    },
                  ),
                  const Expanded(
                    child: Text(
                      "I accept 100% of the legal risk and absolute liability.",
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
                    backgroundColor: _accepted ? AppTheme.error : AppTheme.surface,
                    foregroundColor: Colors.white,
                  ),
                  child: const Text("I ACCEPT"),
                ),
              )
            ],
          ),
        ),
      ),
    );
  }
}
