import Flutter
import UIKit
import Mobile

@main
@objc class AppDelegate: FlutterAppDelegate {
  var daemon: MobileNode?

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    let controller : FlutterViewController = window?.rootViewController as! FlutterViewController
    let channel = FlutterMethodChannel(name: "com.shadowlink/daemon", binaryMessenger: controller.binaryMessenger)
    
    channel.setMethodCallHandler({
      [weak self] (call: FlutterMethodCall, result: @escaping FlutterResult) -> Void in
      if call.method == "start" {
        let args = call.arguments as? [String: Any]
        let port = args?["port"] as? Int ?? 1080
        do {
            if self?.daemon == nil {
                try self?.daemon = Mobile.MobileStartEntryNode(port)
            }
            result(true)
        } catch let error {
            result(FlutterError(code: "DAEMON_ERROR", message: error.localizedDescription, details: nil))
        }
      } else if call.method == "stop" {
        do {
            try self?.daemon?.stop()
            self?.daemon = nil
            result(true)
        } catch let error {
            result(FlutterError(code: "DAEMON_ERROR", message: error.localizedDescription, details: nil))
        }
      } else {
        result(FlutterMethodNotImplemented)
      }
    })

    GeneratedPluginRegistrant.register(with: self)
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }
}
