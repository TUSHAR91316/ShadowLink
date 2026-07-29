/// Central configuration constants for the ShadowLink Flutter application.
///
/// All hardcoded values (ports, channel names, file paths, asset keys) belong
/// here so they can be changed in one place and stay in sync across the app.
library;

/// The Flutter MethodChannel name used to communicate with the native Go engine
/// on Android (Kotlin) and iOS (Swift). Must match the channel name registered
/// in MainActivity.kt and AppDelegate.swift exactly.
const String kDaemonChannel = 'com.shadowlink/daemon';

/// The default SOCKS5 proxy port passed to the native Go engine on mobile.
/// Must match [config.DefaultSOCKSPort] in the Go config package.
const int kDefaultSOCKSPort = 1080;

/// The name of the file written to disk when the user accepts the EULA.
/// Must match [config.EULAFileName] in the Go config package.
const String kEulaFileName = '.shadowlink_accepted';

/// The Flutter asset key for the bundled TERMS_AND_CONDITIONS.md file.
/// Must match the path listed in pubspec.yaml assets section.
const String kTermsAssetKey = '../TERMS_AND_CONDITIONS.md';

/// The MethodChannel method name to start the Go daemon.
const String kMethodStart = 'start';

/// The MethodChannel method name to stop the Go daemon.
const String kMethodStop = 'stop';

/// The MethodChannel argument key for the SOCKS5 port.
const String kArgPort = 'port';

/// Connection timeout before the daemon is considered failed.
const Duration kConnectionTimeout = Duration(seconds: 30);
