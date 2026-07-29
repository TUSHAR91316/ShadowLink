import Flutter
import UIKit
import Mobile

@main
@objc class AppDelegate: FlutterAppDelegate {

    // MARK: - Constants
    // Must match kDaemonChannel, kMethodStart/Stop, kArgPort, kDefaultSOCKSPort
    // in lib/config/app_config.dart.
    private enum Channel {
        static let name        = "com.shadowlink/daemon"
        static let methodStart = "start"
        static let methodStop  = "stop"
        static let argPort     = "port"
        static let defaultPort: Int64 = 1080
    }

    // MARK: - State
    private var daemon: MobileNode?

    // MARK: - App Lifecycle

    override func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        let controller = window?.rootViewController as! FlutterViewController
        let channel = FlutterMethodChannel(
            name: Channel.name,
            binaryMessenger: controller.binaryMessenger
        )

        channel.setMethodCallHandler { [weak self] call, result in
            switch call.method {
            case Channel.methodStart:
                let args = call.arguments as? [String: Any]
                let port = (args?[Channel.argPort] as? Int).map { Int64($0) } ?? Channel.defaultPort
                do {
                    if self?.daemon == nil {
                        self?.daemon = try MobileStartEntryNode(port)
                    }
                    result(true)
                } catch {
                    result(FlutterError(
                        code: "DAEMON_ERROR",
                        message: error.localizedDescription,
                        details: nil
                    ))
                }

            case Channel.methodStop:
                do {
                    try self?.daemon?.stop()
                    self?.daemon = nil
                    result(true)
                } catch {
                    result(FlutterError(
                        code: "DAEMON_ERROR",
                        message: error.localizedDescription,
                        details: nil
                    ))
                }

            default:
                result(FlutterMethodNotImplemented)
            }
        }

        GeneratedPluginRegistrant.register(with: self)
        return super.application(application, didFinishLaunchingWithOptions: launchOptions)
    }
}
