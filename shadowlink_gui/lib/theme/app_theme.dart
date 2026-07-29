import 'package:flutter/material.dart';

class AppTheme {
  static const Color background = Color(0xFF0D1117);
  static const Color surface = Color(0xFF161B22);
  static const Color primary = Color(0xFF00F0FF); // Neon Cyan
  static const Color textMain = Color(0xFFC9D1D9);
  static const Color textMuted = Color(0xFF8B949E);
  static const Color success = Color(0xFF2EA043);
  static const Color error = Color(0xFFF85149);

  static ThemeData get darkTheme {
    return ThemeData(
      brightness: Brightness.dark,
      scaffoldBackgroundColor: background,
      primaryColor: primary,
      colorScheme: const ColorScheme.dark(
        primary: primary,
        surface: surface,  // 'background' field is deprecated — surface covers both roles
      ),
      appBarTheme: const AppBarTheme(
        backgroundColor: background,
        elevation: 0,
        centerTitle: true,
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: primary,
          foregroundColor: background,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(8),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
        ),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) return primary;
          return textMuted;
        }),
        trackColor: WidgetStateProperty.resolveWith((states) {
          // withValues() replaces deprecated withOpacity() to avoid precision loss
          if (states.contains(WidgetState.selected)) {
            return primary.withValues(alpha: 0.5);
          }
          return surface;
        }),
      ),
    );
  }
}
