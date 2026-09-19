package com.shadowlink.shadowlink_gui

import androidx.annotation.NonNull
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import mobile.Mobile
import mobile.MobileNode

class MainActivity : FlutterActivity() {

    companion object {
        // Sourced here as constants so they are never hardcoded in logic below.
        // Must match kDaemonChannel and kDefaultSOCKSPort in lib/config/app_config.dart.
        private const val CHANNEL = "com.shadowlink/daemon"
        private const val DEFAULT_SOCKS_PORT = 1080L

        // MethodChannel method names — must match kMethodStart/kMethodStop in app_config.dart.
        private const val METHOD_START = "start"
        private const val METHOD_STOP  = "stop"

        // MethodChannel argument keys — must match kArgPort in app_config.dart.
        private const val ARG_PORT = "port"
    }

    private var daemon: MobileNode? = null

    override fun configureFlutterEngine(@NonNull flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    METHOD_START -> {
                        val port = call.argument<Int>(ARG_PORT)?.toLong() ?: DEFAULT_SOCKS_PORT
                        try {
                            if (daemon == null) {
                                daemon = Mobile.startEntryNode(port)
                            }
                            result.success(true)
                        } catch (e: Exception) {
                            result.error("DAEMON_ERROR", e.message, null)
                        }
                    }
                    METHOD_STOP -> {
                        try {
                            daemon?.stop()
                            daemon = null
                            result.success(true)
                        } catch (e: Exception) {
                            result.error("DAEMON_ERROR", e.message, null)
                        }
                    }
                    else -> result.notImplemented()
                }
            }
    }
}
