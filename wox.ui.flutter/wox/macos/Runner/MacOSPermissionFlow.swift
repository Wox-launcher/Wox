import Cocoa
import FlutterMacOS

private let macOSPermissionFlowChannelName = "com.wox.macos_permission_flow"
private let systemSettingsBundleIdentifier = "com.apple.systempreferences"

private enum WoxMacOSPermissionType: String {
  case accessibility
  case fullDiskAccess

  var settingsAnchor: String {
    switch self {
    case .accessibility:
      return "Privacy_Accessibility"
    case .fullDiskAccess:
      return "Privacy_AllFiles"
    }
  }
}

final class MacOSPermissionFlowBridge: NSObject {
  private let channel: FlutterMethodChannel
  private weak var hostWindow: NSWindow?
  private lazy var flowController = MacOSPermissionFlowController(
    onClosed: { [weak self] in
      self?.restoreHostWindowAfterFlow()
      self?.hasActiveFlow = false
      self?.channel.invokeMethod("permissionFlowClosed", arguments: nil)
    },
    onRefreshRequested: { [weak self] in
      self?.channel.invokeMethod("permissionStatusRefreshRequested", arguments: nil)
    }
  )
  private var activationObserver: NSObjectProtocol?
  private var hasActiveFlow = false
  private var shouldRestoreHostWindow = false

  init(binaryMessenger: FlutterBinaryMessenger, hostWindow: NSWindow) {
    channel = FlutterMethodChannel(name: macOSPermissionFlowChannelName, binaryMessenger: binaryMessenger)
    self.hostWindow = hostWindow
    super.init()

    channel.setMethodCallHandler { [weak self] call, result in
      DispatchQueue.main.async {
        self?.handle(call: call, result: result)
      }
    }
    activationObserver = NotificationCenter.default.addObserver(
      forName: NSApplication.didBecomeActiveNotification,
      object: nil,
      queue: .main
    ) { [weak self] _ in
      if self?.hasActiveFlow == true {
        self?.channel.invokeMethod("applicationActivated", arguments: nil)
      }
    }
  }

  deinit {
    if let activationObserver {
      NotificationCenter.default.removeObserver(activationObserver)
    }
  }

  private func handle(call: FlutterMethodCall, result: @escaping FlutterResult) {
    guard call.method == "openPermissionFlow" else {
      result(FlutterMethodNotImplemented)
      return
    }
    guard
      let arguments = call.arguments as? [String: Any],
      let rawPermissionType = arguments["permissionType"] as? String,
      let permissionType = WoxMacOSPermissionType(rawValue: rawPermissionType),
      let corePID = arguments["corePid"] as? Int
    else {
      result(FlutterError(code: "INVALID_ARGUMENTS", message: "Invalid macOS permission flow arguments", details: nil))
      return
    }

    hideHostWindowForFlow()
    hasActiveFlow = true
    flowController.open(
      permissionType: permissionType,
      corePID: pid_t(corePID),
      title: arguments["title"] as? String ?? "Wox",
      rightInstruction: arguments["rightInstruction"] as? String ?? "Drag Wox into the permission list on the left.",
      bottomInstruction: arguments["bottomInstruction"] as? String ?? "Drag Wox into the permission list above.",
      manualInstruction: arguments["manualInstruction"] as? String ?? "Add Wox manually in System Settings."
    )
    result(nil)
  }

  // Hide only the Flutter host window so the native drag panel remains visible beside System Settings.
  private func hideHostWindowForFlow() {
    guard !hasActiveFlow, let hostWindow else { return }
    shouldRestoreHostWindow = hostWindow.isVisible
    if shouldRestoreHostWindow {
      hostWindow.orderOut(nil)
    }
  }

  // Restore onboarding only if this permission flow hid it.
  private func restoreHostWindowAfterFlow() {
    defer { shouldRestoreHostWindow = false }
    guard shouldRestoreHostWindow, let hostWindow else { return }
    hostWindow.orderFront(nil)
  }
}

private final class PermissionGuidePanel: NSPanel {
  override var canBecomeKey: Bool { false }
  override var canBecomeMain: Bool { false }
}

private enum PermissionGuidePlacement {
  case right
  case bottom
}

private struct PermissionGuideLayout {
  let origin: NSPoint
  let placement: PermissionGuidePlacement
}

private final class MacOSPermissionFlowController: NSObject, NSWindowDelegate {
  private let onClosed: () -> Void
  private let onRefreshRequested: () -> Void
  private var panel: PermissionGuidePanel?
  private weak var instructionLabel: NSTextField?
  private var tracker: Timer?
  private var systemSettingsWasRunning = false
  private var panelHasTargetFrame = false
  private var lastRefreshRequest = Date.distantPast
  private var rightInstruction = ""
  private var bottomInstruction = ""

  init(onClosed: @escaping () -> Void, onRefreshRequested: @escaping () -> Void) {
    self.onClosed = onClosed
    self.onRefreshRequested = onRefreshRequested
    super.init()
  }

  // Open replaces an existing guide so only one permission panel can be active at a time.
  func open(
    permissionType: WoxMacOSPermissionType,
    corePID: pid_t,
    title: String,
    rightInstruction: String,
    bottomInstruction: String,
    manualInstruction: String
  ) {
    close(notify: false)

    self.rightInstruction = rightInstruction
    self.bottomInstruction = bottomInstruction
    let appURL = resolveAuthorizationApplication(corePID: corePID)
    let panel = makePanel(title: title, instruction: bottomInstruction, manualInstruction: manualInstruction, appURL: appURL)
    self.panel = panel
    systemSettingsWasRunning = false
    panelHasTargetFrame = false
    lastRefreshRequest = .distantPast

    openSystemSettings(anchor: permissionType.settingsAnchor)
    startTrackingSystemSettings()
  }

  func windowWillClose(_ notification: Notification) {
    guard notification.object as AnyObject? === panel else { return }
    close(notify: true)
  }

  private func makePanel(title: String, instruction: String, manualInstruction: String, appURL: URL?) -> PermissionGuidePanel {
    let size = NSSize(width: 330, height: appURL == nil ? 158 : 184)
    let panel = PermissionGuidePanel(
      contentRect: NSRect(origin: .zero, size: size),
      styleMask: [.borderless, .nonactivatingPanel],
      backing: .buffered,
      defer: false
    )
    panel.delegate = self
    panel.isReleasedWhenClosed = false
    panel.level = .floating
    panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
    panel.backgroundColor = .clear
    panel.isOpaque = false
    panel.hasShadow = true
    panel.hidesOnDeactivate = false

    let effect = NSVisualEffectView(frame: NSRect(origin: .zero, size: size))
    effect.material = .popover
    effect.blendingMode = .behindWindow
    effect.state = .active
    effect.wantsLayer = true
    effect.layer?.cornerRadius = 14
    effect.layer?.masksToBounds = true
    panel.contentView = effect

    let titleLabel = makeLabel(text: title, font: .systemFont(ofSize: 15, weight: .semibold), color: .labelColor)
    let bodyLabel = makeLabel(
      text: appURL == nil ? manualInstruction : instruction,
      font: .systemFont(ofSize: 13, weight: appURL == nil ? .regular : .semibold),
      color: appURL == nil ? .secondaryLabelColor : .systemOrange
    )
    bodyLabel.maximumNumberOfLines = 3
    bodyLabel.lineBreakMode = .byWordWrapping
    bodyLabel.alignment = .center
    instructionLabel = appURL == nil ? nil : bodyLabel

    let closeButton = NSButton(title: "×", target: self, action: #selector(closeButtonPressed))
    closeButton.isBordered = false
    closeButton.font = .systemFont(ofSize: 18, weight: .medium)
    closeButton.contentTintColor = .secondaryLabelColor
    closeButton.translatesAutoresizingMaskIntoConstraints = false

    effect.addSubview(titleLabel)
    effect.addSubview(bodyLabel)
    effect.addSubview(closeButton)

    var constraints = [
      titleLabel.topAnchor.constraint(equalTo: effect.topAnchor, constant: 18),
      titleLabel.centerXAnchor.constraint(equalTo: effect.centerXAnchor),
      bodyLabel.leadingAnchor.constraint(equalTo: effect.leadingAnchor, constant: 24),
      bodyLabel.trailingAnchor.constraint(equalTo: effect.trailingAnchor, constant: -24),
      bodyLabel.topAnchor.constraint(equalTo: titleLabel.bottomAnchor, constant: 8),
      closeButton.topAnchor.constraint(equalTo: effect.topAnchor, constant: 8),
      closeButton.trailingAnchor.constraint(equalTo: effect.trailingAnchor, constant: -10),
      closeButton.widthAnchor.constraint(equalToConstant: 24),
      closeButton.heightAnchor.constraint(equalToConstant: 24),
    ]

    if let appURL {
      let dragView = PermissionApplicationDragView(appURL: appURL)
      dragView.translatesAutoresizingMaskIntoConstraints = false
      dragView.onDraggingChanged = { [weak panel] isDragging in
        panel?.ignoresMouseEvents = isDragging
        if isDragging {
          panel?.orderBack(nil)
        } else {
          panel?.orderFrontRegardless()
        }
      }
      effect.addSubview(dragView)
      constraints.append(contentsOf: [
        dragView.topAnchor.constraint(equalTo: bodyLabel.bottomAnchor, constant: 12),
        dragView.centerXAnchor.constraint(equalTo: effect.centerXAnchor),
        dragView.widthAnchor.constraint(equalToConstant: 64),
        dragView.heightAnchor.constraint(equalToConstant: 64),
      ])
    }
    NSLayoutConstraint.activate(constraints)
    return panel
  }

  private func makeLabel(text: String, font: NSFont, color: NSColor) -> NSTextField {
    let label = NSTextField(labelWithString: text)
    label.font = font
    label.textColor = color
    label.translatesAutoresizingMaskIntoConstraints = false
    return label
  }

  @objc private func closeButtonPressed() {
    close(notify: true)
  }

  // Resolve from the Go core process so the drag item is the outer Wox.app instead of the nested Flutter host.
  private func resolveAuthorizationApplication(corePID: pid_t) -> URL? {
    guard
      let bundleURL = NSRunningApplication(processIdentifier: corePID)?.bundleURL,
      bundleURL.pathExtension.lowercased() == "app",
      bundleURL.lastPathComponent.lowercased() != "wox-ui.app",
      FileManager.default.fileExists(atPath: bundleURL.path)
    else {
      return nil
    }
    return bundleURL
  }

  private func openSystemSettings(anchor: String) {
    guard let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?\(anchor)") else { return }
    NSWorkspace.shared.open(url)
  }

  private func startTrackingSystemSettings() {
    tracker?.invalidate()
    tracker = Timer.scheduledTimer(withTimeInterval: 0.15, repeats: true) { [weak self] _ in
      self?.trackSystemSettings()
    }
    trackSystemSettings()
  }

  private func trackSystemSettings() {
    let settingsApps = NSRunningApplication.runningApplications(withBundleIdentifier: systemSettingsBundleIdentifier)
    if settingsApps.isEmpty {
      if systemSettingsWasRunning {
        close(notify: true)
      }
      return
    }
    systemSettingsWasRunning = true
    requestPermissionStatusRefreshIfNeeded()

    guard let settingsFrame = systemSettingsWindowFrame(), let panel else { return }
    let layout = panelLayout(nextTo: settingsFrame, panelSize: panel.frame.size)
    instructionLabel?.stringValue = layout.placement == .right ? rightInstruction : bottomInstruction
    if !panelHasTargetFrame {
      panelHasTargetFrame = true
      panel.setFrameOrigin(layout.origin)
      panel.orderFrontRegardless()
      return
    }
    NSAnimationContext.runAnimationGroup { context in
      context.duration = 0.12
      panel.animator().setFrameOrigin(layout.origin)
    }
  }

  // CGWindowList tracks System Settings without introducing another Accessibility permission dependency.
  private func systemSettingsWindowFrame() -> NSRect? {
    guard let windowInfo = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else { return nil }
    let currentPID = ProcessInfo.processInfo.processIdentifier
    let candidates = windowInfo.compactMap { info -> (frame: NSRect, hasTitle: Bool)? in
      guard
        let ownerPID = (info[kCGWindowOwnerPID as String] as? NSNumber)?.int32Value,
        ownerPID != currentPID,
        NSRunningApplication(processIdentifier: ownerPID)?.bundleIdentifier == systemSettingsBundleIdentifier,
        (info[kCGWindowLayer as String] as? NSNumber)?.intValue == 0,
        ((info[kCGWindowAlpha as String] as? NSNumber)?.doubleValue ?? 1) > 0,
        let bounds = info[kCGWindowBounds as String] as? NSDictionary,
        let quartzFrame = CGRect(dictionaryRepresentation: bounds as CFDictionary),
        quartzFrame.width > 200,
        quartzFrame.height > 200
      else {
        return nil
      }
      let title = (info[kCGWindowName as String] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
      return (appKitFrame(fromGlobalTopLeftFrame: quartzFrame), !title.isEmpty)
    }
    return candidates.max { lhs, rhs in
      if lhs.hasTitle != rhs.hasTitle {
        return !lhs.hasTitle
      }
      return lhs.frame.width * lhs.frame.height < rhs.frame.width * rhs.frame.height
    }?.frame
  }

  // Convert the window-server frame against its actual display so vertically offset monitors remain aligned.
  private func appKitFrame(fromGlobalTopLeftFrame frame: CGRect) -> CGRect {
    let screens = NSScreen.screens.compactMap { screen -> (appKitFrame: CGRect, quartzFrame: CGRect)? in
      guard let number = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber else { return nil }
      return (screen.frame, CGDisplayBounds(CGDirectDisplayID(number.uint32Value)))
    }
    guard
      let matchedScreen = screens.filter({ $0.quartzFrame.intersects(frame) }).max(by: {
        $0.quartzFrame.intersection(frame).width * $0.quartzFrame.intersection(frame).height
          < $1.quartzFrame.intersection(frame).width * $1.quartzFrame.intersection(frame).height
      })
    else {
      return frame
    }

    let localX = frame.minX - matchedScreen.quartzFrame.minX
    let localY = frame.minY - matchedScreen.quartzFrame.minY
    return CGRect(
      x: matchedScreen.appKitFrame.minX + localX,
      y: matchedScreen.appKitFrame.maxY - localY - frame.height,
      width: frame.width,
      height: frame.height
    )
  }

  private func requestPermissionStatusRefreshIfNeeded() {
    guard Date().timeIntervalSince(lastRefreshRequest) >= 0.8 else { return }
    lastRefreshRequest = Date()
    onRefreshRequested()
  }

  // Keep the attachment axis outside System Settings; only the perpendicular axis may be clamped to the screen.
  private func panelLayout(nextTo settingsFrame: NSRect, panelSize: NSSize) -> PermissionGuideLayout {
    let screen =
      NSScreen.screens.filter({ $0.frame.intersects(settingsFrame) }).max(by: {
        $0.frame.intersection(settingsFrame).width * $0.frame.intersection(settingsFrame).height
          < $1.frame.intersection(settingsFrame).width * $1.frame.intersection(settingsFrame).height
      }) ?? NSScreen.main
    let visibleFrame = screen?.visibleFrame ?? settingsFrame
    let attachmentGap: CGFloat = 0
    let rightSpace = visibleFrame.maxX - settingsFrame.maxX
    let bottomSpace = settingsFrame.minY - visibleFrame.minY

    if rightSpace >= panelSize.width + attachmentGap {
      let y = min(max(settingsFrame.maxY - panelSize.height, visibleFrame.minY), visibleFrame.maxY - panelSize.height)
      return PermissionGuideLayout(origin: NSPoint(x: settingsFrame.maxX + attachmentGap, y: y), placement: .right)
    }
    if bottomSpace >= panelSize.height + attachmentGap {
      let x = min(max(settingsFrame.maxX - panelSize.width, visibleFrame.minX), visibleFrame.maxX - panelSize.width)
      return PermissionGuideLayout(
        origin: NSPoint(x: x, y: settingsFrame.minY - panelSize.height - attachmentGap),
        placement: .bottom
      )
    }

    // When neither edge fully fits, staying attached outside the better edge is preferable to
    // covering the permission list and obscuring the destination the user must drag into.
    if rightSpace / panelSize.width >= bottomSpace / panelSize.height {
      let y = min(max(settingsFrame.maxY - panelSize.height, visibleFrame.minY), visibleFrame.maxY - panelSize.height)
      return PermissionGuideLayout(origin: NSPoint(x: settingsFrame.maxX + attachmentGap, y: y), placement: .right)
    }
    let x = min(max(settingsFrame.maxX - panelSize.width, visibleFrame.minX), visibleFrame.maxX - panelSize.width)
    return PermissionGuideLayout(
      origin: NSPoint(x: x, y: settingsFrame.minY - panelSize.height - attachmentGap),
      placement: .bottom
    )
  }

  private func close(notify: Bool) {
    tracker?.invalidate()
    tracker = nil
    panelHasTargetFrame = false
    guard let currentPanel = panel else { return }
    panel = nil
    instructionLabel = nil
    currentPanel.delegate = nil
    currentPanel.orderOut(nil)
    if notify {
      onClosed()
    }
  }
}

private final class PermissionApplicationDragView: NSView, NSDraggingSource {
  let appURL: URL
  var onDraggingChanged: ((Bool) -> Void)?

  init(appURL: URL) {
    self.appURL = appURL
    super.init(frame: .zero)
    wantsLayer = true
    layer?.cornerRadius = 12
    layer?.backgroundColor = NSColor.controlBackgroundColor.withAlphaComponent(0.8).cgColor

    let iconView = NSImageView(image: NSWorkspace.shared.icon(forFile: appURL.path))
    iconView.imageScaling = .scaleProportionallyUpOrDown
    iconView.translatesAutoresizingMaskIntoConstraints = false
    addSubview(iconView)
    NSLayoutConstraint.activate([
      iconView.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 8),
      iconView.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -8),
      iconView.topAnchor.constraint(equalTo: topAnchor, constant: 8),
      iconView.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -8),
    ])
  }

  required init?(coder: NSCoder) {
    return nil
  }

  override func mouseDown(with event: NSEvent) {
    let item = NSDraggingItem(pasteboardWriter: appURL as NSURL)
    let icon = NSWorkspace.shared.icon(forFile: appURL.path)
    item.setDraggingFrame(bounds, contents: icon)
    onDraggingChanged?(true)
    let session = beginDraggingSession(with: [item], event: event, source: self)
    session.animatesToStartingPositionsOnCancelOrFail = true
  }

  func draggingSession(_ session: NSDraggingSession, sourceOperationMaskFor context: NSDraggingContext) -> NSDragOperation {
    return .copy
  }

  func draggingSession(_ session: NSDraggingSession, endedAt screenPoint: NSPoint, operation: NSDragOperation) {
    onDraggingChanged?(false)
  }
}
