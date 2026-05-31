import Cocoa
import FlutterMacOS

private let resultDragChannelName = "com.wox.result_drag"

final class ResultDragBridge: NSObject, NSDraggingSource {
  private let channel: FlutterMethodChannel
  private weak var sourceView: NSView?
  private weak var sourceWindow: NSWindow?
  private var pendingResult: FlutterResult?

  init(binaryMessenger: FlutterBinaryMessenger, sourceView: NSView) {
    channel = FlutterMethodChannel(name: resultDragChannelName, binaryMessenger: binaryMessenger)
    self.sourceView = sourceView
    super.init()

    channel.setMethodCallHandler { [weak self] call, result in
      DispatchQueue.main.async {
        self?.handle(call: call, result: result)
      }
    }
  }

  private func handle(call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "startFileDrag":
      startFileDrag(arguments: call.arguments, result: result)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func startFileDrag(arguments: Any?, result: @escaping FlutterResult) {
    guard pendingResult == nil else {
      result(statusPayload("error"))
      return
    }

    guard let sourceView,
      let window = sourceView.window,
      let event = window.currentEvent ?? NSApp.currentEvent,
      let args = arguments as? [String: Any],
      let rawFiles = args["files"] as? [String]
    else {
      result(statusPayload("error"))
      return
    }

    let files = rawFiles.filter { FileManager.default.fileExists(atPath: $0) }
    guard !files.isEmpty else {
      result(statusPayload("error"))
      return
    }

    let dragPoint = sourceView.convert(event.locationInWindow, from: nil)
    let items = files.enumerated().map { index, path in
      makeDraggingItem(path: path, index: index, dragPoint: dragPoint)
    }

    pendingResult = result
    let session = sourceView.beginDraggingSession(with: items, event: event, source: self)
    session.draggingFormation = .pile
    session.animatesToStartingPositionsOnCancelOrFail = true
    sourceWindow = window
  }

  private func makeDraggingItem(path: String, index: Int, dragPoint: NSPoint) -> NSDraggingItem {
    let fileURL = NSURL(fileURLWithPath: path)
    let item = NSDraggingItem(pasteboardWriter: fileURL)
    let icon = (NSWorkspace.shared.icon(forFile: path).copy() as? NSImage) ?? NSImage()
    let iconSize = NSSize(width: 40, height: 40)
    let offset = CGFloat(min(index, 4)) * 4

    icon.size = iconSize
    item.setDraggingFrame(
      NSRect(
        x: dragPoint.x - iconSize.width / 2 + offset,
        y: dragPoint.y - iconSize.height / 2 - offset,
        width: iconSize.width,
        height: iconSize.height
      ),
      contents: icon
    )
    return item
  }

  func draggingSession(_ session: NSDraggingSession, sourceOperationMaskFor context: NSDraggingContext) -> NSDragOperation {
    return .copy
  }

  func draggingSession(_ session: NSDraggingSession, endedAt screenPoint: NSPoint, operation: NSDragOperation) {
    let releasedInSourceWindow = sourceWindow?.frame.contains(screenPoint) ?? false
    let status: String
    if releasedInSourceWindow {
      status = "cancel_in_source"
    } else if operation.contains(.copy) {
      status = "success"
    } else {
      status = "cancel"
    }

    if status != "cancel_in_source" {
      // Hide exactly when drag ends for external targets while keeping Wox
      // visible when the pointer is released back inside the launcher.
      sourceWindow?.orderOut(nil)
    }

    pendingResult?(statusPayload(status))
    pendingResult = nil
    sourceWindow = nil
  }

  private func statusPayload(_ status: String) -> [String: String] {
    return ["status": status]
  }
}
