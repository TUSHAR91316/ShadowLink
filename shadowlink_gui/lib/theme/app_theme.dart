import 'package:flutter/material.dart';

/// AppTheme implements the "Obsidian Cyber Grid" design system derived from Google Stitch,
/// inspired by Proton VPN, ExpressVPN, and modern decentralized cyber-security suites.
class AppTheme {
  // ─── Canvas & Surface Substrates ───────────────────────────────────────────
  static const Color background = Color(0xFF090D12);       // Deep Obsidian Canvas
  static const Color surface = Color(0xFF101419);          // Sub-surface base
  static const Color surfaceLow = Color(0xFF181C21);       // Subtle panel base
  static const Color surfaceContainer = Color(0xFF1C2025); // Elevated card level
  static const Color surfaceHigh = Color(0xFF262A30);      // High-contrast module
  static const Color cardGlass = Color(0xFF111827);        // Frosted acrylic cards

  // ─── Cyber Accents & Defense States ─────────────────────────────────────────
  static const Color primary = Color(0xFF10B981);          // Active Node Emerald
  static const Color primaryBright = Color(0xFF4EDEA3);    // Specular Emerald
  static const Color secondary = Color(0xFF06B6D4);        // Telemetry Cyan
  static const Color secondaryBright = Color(0xFF4CD7F6);  // Light Cyan Highlight
  static const Color error = Color(0xFFEF4444);            // Killswitch Crimson
  static const Color warning = Color(0xFFF59E0B);          // Precautionary Amber

  // ─── Typography & Neutrals ──────────────────────────────────────────────────
  static const Color textMain = Color(0xFFE0E2EA);         // High-contrast primary text
  static const Color textMuted = Color(0xFF86948A);        // Muted labels & telemetry
  static const Color borderSubtle = Color(0x1FFFFFFF);     // 12% white boundary
  static const Color borderEmerald = Color(0x4D10B981);    // 30% emerald neon aura
  static const Color borderCyan = Color(0x4D06B6D4);       // 30% cyan neon aura

  // ─── Common BoxDecorations ──────────────────────────────────────────────────
  static BoxDecoration glassCard({
    Color? borderColor,
    double radius = 16,
    List<BoxShadow>? shadows,
  }) {
    return BoxDecoration(
      color: cardGlass.withValues(alpha: 0.82),
      borderRadius: BorderRadius.circular(radius),
      border: Border.all(
        color: borderColor ?? borderSubtle,
        width: 1.2,
      ),
      boxShadow: shadows ??
          [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.35),
              blurRadius: 18,
              offset: const Offset(0, 6),
            ),
          ],
    );
  }

  static BoxDecoration glowCard({
    required Color glowColor,
    double radius = 16,
  }) {
    return BoxDecoration(
      color: surfaceContainer,
      borderRadius: BorderRadius.circular(radius),
      border: Border.all(color: glowColor.withValues(alpha: 0.45), width: 1.2),
      boxShadow: [
        BoxShadow(
          color: glowColor.withValues(alpha: 0.18),
          blurRadius: 24,
          spreadRadius: 2,
        ),
      ],
    );
  }

  // ─── ThemeData ──────────────────────────────────────────────────────────────
  static ThemeData get darkTheme {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: background,
      primaryColor: primary,
      colorScheme: const ColorScheme.dark(
        primary: primary,
        secondary: secondary,
        surface: surface,
        error: error,
        onPrimary: Color(0xFF003824),
        onSecondary: Color(0xFF003640),
        onSurface: textMain,
        onError: Colors.white,
      ),
      appBarTheme: const AppBarTheme(
        backgroundColor: background,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        centerTitle: false,
        titleTextStyle: TextStyle(
          color: textMain,
          fontSize: 18,
          fontWeight: FontWeight.w700,
          letterSpacing: 1.2,
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: primary,
          foregroundColor: const Color(0xFF003824),
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
          textStyle: const TextStyle(
            fontWeight: FontWeight.w700,
            fontSize: 14,
            letterSpacing: 0.5,
          ),
        ),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) return primary;
          return textMuted;
        }),
        trackColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return primary.withValues(alpha: 0.35);
          }
          return surfaceHigh;
        }),
        trackOutlineColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) return primary;
          return borderSubtle;
        }),
      ),
      dividerTheme: const DividerThemeData(
        color: borderSubtle,
        thickness: 1,
        space: 1,
      ),
    );
  }
}
