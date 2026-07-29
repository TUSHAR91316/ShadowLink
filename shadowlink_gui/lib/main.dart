import 'package:flutter/material.dart';
import 'theme/app_theme.dart';
import 'screens/eula_screen.dart';

void main() {
  runApp(const ShadowLinkApp());
}

class ShadowLinkApp extends StatelessWidget {
  const ShadowLinkApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ShadowLink dVPN',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.darkTheme,
      home: const EulaScreen(), // Starts at EULA, routes to Dashboard if accepted
    );
  }
}
