import ApplicationServices
import Cocoa
import FlutterMacOS
import ScreenCaptureKit

// The screenshot workspace and saved window positions both use a top-left logical desktop space
// that spans every monitor. The previous macOS conversion mixed that global contract with
// per-screen top edges, which left fullscreen screenshot overlays misaligned as soon as displays
// had different vertical offsets. Keeping every top-left/AppKit conversion anchored to the virtual
// desktop top fixes the multi-display capture bug without introducing a second coordinate system.
private func virtualDesktopTopInAppKit() -> CGFloat {
  return NSScreen.screens.map { $0.frame.maxY }.max() ?? 0
}

private func quartzTopLeftToWorkspaceYOffset() -> CGFloat {
  return virtualDesktopTopInAppKit() - (NSScreen.main?.frame.maxY ?? virtualDesktopTopInAppKit())
}

private func appKitY(fromTopLeftY y: CGFloat, height: CGFloat = 0) -> CGFloat {
  return virtualDesktopTopInAppKit() - y - height
}

private func topLeftY(fromAppKitY y: CGFloat, height: CGFloat) -> CGFloat {
  return virtualDesktopTopInAppKit() - y - height
}

private func topLeftPoint(fromAppKit point: CGPoint) -> CGPoint {
  return CGPoint(x: point.x, y: topLeftY(fromAppKitY: point.y, height: 0))
}

private func quartzPoint(fromWorkspaceTopLeftPoint point: CGPoint) -> CGPoint {
  // Accessibility and CGWindow APIs report top-left coordinates in Quartz display space anchored
  // to the main display. The screenshot selector uses Wox's virtual-desktop top-left space, so the
  // Y offset keeps hover hit-testing aligned on vertical multi-monitor layouts.
  return CGPoint(x: point.x, y: point.y - quartzTopLeftToWorkspaceYOffset())
}

private func workspaceTopLeftRect(fromQuartzRect rect: NSRect) -> NSRect {
  // AX element frames and CGWindow bounds share the Quartz top-left coordinate space. Normalize them
  // once before validation so the returned selection matches the Flutter annotation contract.
  return NSRect(
    x: rect.origin.x,
    y: rect.origin.y + quartzTopLeftToWorkspaceYOffset(),
    width: rect.width,
    height: rect.height
  )
}

private func topLeftRect(fromAppKitRect rect: NSRect) -> NSRect {
  return NSRect(
    x: rect.origin.x,
    y: topLeftY(fromAppKitY: rect.origin.y, height: rect.height),
    width: rect.width,
    height: rect.height
  )
}

private func appKitRect(fromTopLeftRect rect: NSRect) -> NSRect {
  return NSRect(
    x: rect.origin.x,
    y: appKitY(fromTopLeftY: rect.origin.y, height: rect.height),
    width: rect.width,
    height: rect.height
  )
}

private func elapsedMilliseconds(since start: Date) -> Int {
  return Int((Date().timeIntervalSince(start) * 1000).rounded())
}

private func formatTimingRect(_ rect: NSRect) -> String {
  return "\(Int(rect.width.rounded()))x\(Int(rect.height.rounded()))@\(Int(rect.origin.x.rounded())),\(Int(rect.origin.y.rounded()))"
}

private let screenshotHoverMinimumSide: CGFloat = 12
private let screenshotHoverMinimumArea: CGFloat = 256
private let screenshotHoverDisplaySizedWidthRatio: CGFloat = 0.90
private let screenshotHoverDisplaySizedHeightRatio: CGFloat = 0.75

private func isTooSmallForScreenshotHover(_ rect: NSRect) -> Bool {
  // Bug fix: AX and CGWindow can report tiny text/image fragments or invisible slivers with valid
  // geometry. Requiring both minimum side length and area keeps hover preview focused on regions a
  // user can reasonably click and annotate.
  return rect.width < screenshotHoverMinimumSide || rect.height < screenshotHoverMinimumSide || rect.width * rect.height < screenshotHoverMinimumArea
}

private func isDisplaySizedScreenshotHoverCandidate(_ rect: NSRect, displayBounds: NSRect) -> Bool {
  return displayBounds.width > 0 &&
    displayBounds.height > 0 &&
    rect.width >= displayBounds.width * screenshotHoverDisplaySizedWidthRatio &&
    rect.height >= displayBounds.height * screenshotHoverDisplaySizedHeightRatio
}

private func screenshotWindowLevel() -> NSWindow.Level {
  // `.popUpMenu` was high enough for regular launcher panels but still sat underneath some
  // fullscreen and always-on-top utility windows, which made the capture UI appear to vanish right
  // after screenshot handoff. Match the system shielding/window-saver range so both the native
  // selector and the Flutter annotation workspace stay above the windows being captured.
  let shieldingLevel = NSWindow.Level(rawValue: Int(CGShieldingWindowLevel()))
  return max(.screenSaver, shieldingLevel)
}

private func scrollingCaptureControlsWindowLevel() -> NSWindow.Level {
  // The scrolling mask and the compact Flutter preview are separate windows. Keeping them at the
  // exact same level made the initial AppKit ordering depend on focus events, so the preview could
  // stay visually under the mask until the user clicked. Put controls one level above the passive
  // mask while keeping both in the screenshot overlay range.
  return NSWindow.Level(rawValue: screenshotWindowLevel().rawValue + 1)
}

private var screenshotDiagonalResizeCursorCache: [String: NSCursor] = [:]

private func screenshotDiagonalResizeCursor(kind: String) -> NSCursor {
  if let cachedCursor = screenshotDiagonalResizeCursorCache[kind] {
    return cachedCursor
  }

  let cursor = makeScreenshotDiagonalResizeCursor(kind: kind)
  screenshotDiagonalResizeCursorCache[kind] = cursor
  return cursor
}

private func makeScreenshotDiagonalResizeCursor(kind: String) -> NSCursor {
  let size: CGFloat = 24
  let margin: CGFloat = 4.5
  let image = NSImage(size: NSSize(width: size, height: size))

  image.lockFocus()
  defer { image.unlockFocus() }

  guard let context = NSGraphicsContext.current?.cgContext else {
    return NSCursor.arrow
  }

  let isNorthEastSouthWest = kind == "resizeUpRightDownLeft"
  let start = isNorthEastSouthWest ? CGPoint(x: size - margin, y: size - margin) : CGPoint(x: margin, y: size - margin)
  let end = isNorthEastSouthWest ? CGPoint(x: margin, y: margin) : CGPoint(x: size - margin, y: margin)

  func addArrowHead(tip: CGPoint, toward: CGPoint) {
    let dx = toward.x - tip.x
    let dy = toward.y - tip.y
    let length = max(1, sqrt(dx * dx + dy * dy))
    let unit = CGPoint(x: dx / length, y: dy / length)
    let perpendicular = CGPoint(x: -unit.y, y: unit.x)
    let base = CGPoint(x: tip.x + unit.x * 6, y: tip.y + unit.y * 6)
    let sideA = CGPoint(x: base.x + perpendicular.x * 4.5, y: base.y + perpendicular.y * 4.5)
    let sideB = CGPoint(x: base.x - perpendicular.x * 4.5, y: base.y - perpendicular.y * 4.5)

    context.move(to: tip)
    context.addLine(to: sideA)
    context.move(to: tip)
    context.addLine(to: sideB)
  }

  func strokeCursor(color: NSColor, lineWidth: CGFloat) {
    context.setStrokeColor(color.cgColor)
    context.setLineWidth(lineWidth)
    context.setLineCap(.round)
    context.setLineJoin(.round)
    context.beginPath()
    context.move(to: start)
    context.addLine(to: end)
    addArrowHead(tip: start, toward: end)
    addArrowHead(tip: end, toward: start)
    context.strokePath()
  }

  // Flutter's macOS engine maps unsupported cursor names back to the arrow cursor, including the
  // diagonal resize names used by screenshot corner handles. Drawing a tiny AppKit cursor here fixes
  // that platform gap without changing the standard cursor behavior on Windows and Linux.
  strokeCursor(color: NSColor.black.withAlphaComponent(0.82), lineWidth: 4.2)
  strokeCursor(color: NSColor.white, lineWidth: 2.1)
  return NSCursor(image: image, hotSpot: NSPoint(x: size / 2, y: size / 2))
}

private func applyScreenshotWindowPresentation(to window: NSWindow, level: NSWindow.Level) {
  window.collectionBehavior = screenshotCollectionBehavior()
  if let panel = window as? NSPanel {
    panel.hidesOnDeactivate = false
    panel.becomesKeyOnlyIfNeeded = false
    // AppKit resets an NSPanel back to a normal/floating level when `isFloatingPanel` changes. Put
    // panel flags before the final level assignment so scrolling preview controls are not pushed
    // behind the passive mask until a user click reorders the window.
    panel.isFloatingPanel = false
  }
  window.level = level
}

private func makeScreenshotWindowTransparent(_ window: NSWindow) {
  // Scrolling screenshot reuses the launcher Flutter window as a compact preview/toolbox panel.
  // NSWindow transparency alone was not enough after frame moves because AppKit/Flutter layers could
  // keep an opaque backing store, leaving a gray rectangle around otherwise transparent preview UI.
  window.isOpaque = false
  window.backgroundColor = .clear
  guard let contentView = window.contentView else {
    return
  }

  clearBackingLayers(in: contentView)
}

private func clearBackingLayers(in view: NSView) {
  view.wantsLayer = true
  view.layer?.isOpaque = false
  view.layer?.backgroundColor = NSColor.clear.cgColor
  for subview in view.subviews {
    clearBackingLayers(in: subview)
  }
}

private func screenshotCollectionBehavior() -> NSWindow.CollectionBehavior {
  // The native selector already relied on fullscreen auxiliary behavior to coexist with fullscreen
  // apps. Reusing that exact behavior for the prepared Flutter workspace avoids downgrading the
  // annotation stage into a regular panel that can be pushed behind fullscreen floating windows.
  var collectionBehavior: NSWindow.CollectionBehavior = [
    .canJoinAllSpaces,
    .fullScreenAuxiliary,
    .stationary,
    .ignoresCycle,
  ]
  if #available(macOS 13.0, *) {
    collectionBehavior.insert(.canJoinAllApplications)
  }
  return collectionBehavior
}

private func clampPoint(_ point: CGPoint, to bounds: NSRect) -> CGPoint {
  return CGPoint(
    x: min(max(point.x, bounds.minX), bounds.maxX),
    y: min(max(point.y, bounds.minY), bounds.maxY)
  )
}

private func rectFromPoints(_ start: CGPoint, _ end: CGPoint) -> NSRect {
  return NSRect(
    x: min(start.x, end.x),
    y: min(start.y, end.y),
    width: abs(end.x - start.x),
    height: abs(end.y - start.y)
  )
}

private func isManualScreenshotSelection(_ selection: NSRect) -> Bool {
  return selection.width >= 4 && selection.height >= 4
}

private func intersectionArea(_ lhs: NSRect, _ rhs: NSRect) -> CGFloat {
  let intersection = lhs.intersection(rhs)
  if intersection.isEmpty {
    return 0
  }

  return intersection.width * intersection.height
}

private func describeRect(_ rect: NSRect?) -> String {
  guard let rect else {
    return "null"
  }

  return String(
    format: "%.1f,%.1f %.1fx%.1f",
    rect.origin.x,
    rect.origin.y,
    rect.width,
    rect.height
  )
}

private struct CachedDisplayCapture {
  let displayId: String
  let logicalBounds: NSRect
  let visibleBounds: NSRect
  let scale: CGFloat
  let rotation: Int
  let image: CGImage
}

private enum DisplaySnapshotImagePayloadMode {
  case none
  case base64
  case filePath

  var includesImagePayload: Bool {
    switch self {
    case .none:
      return false
    case .base64, .filePath:
      return true
    }
  }

  var logName: String {
    switch self {
    case .none:
      return "none"
    case .base64:
      return "base64"
    case .filePath:
      return "file"
    }
  }
}

private struct NativeSelectionOverlayResult {
  let selection: NSRect?
  let editorVisibleBounds: NSRect?
}

private final class ScreenshotOverlayView: NSView {
  // Keep the native selection color aligned with the Flutter annotation frame. The older purple
  // border made the handoff look like a state change when selection completed.
  private static let selectionBorderColor = NSColor(red: 41 / 255, green: 255 / 255, blue: 114 / 255, alpha: 1)
  private static let overlayColor = NSColor(calibratedWhite: 0, alpha: 0.46)
  private static let labelBackgroundColor = NSColor(calibratedWhite: 0.09, alpha: 0.9)
  private static let selectionShadowColor = NSColor(calibratedWhite: 0, alpha: 0.2)

  let capture: CachedDisplayCapture

  var globalSelection: NSRect? {
    didSet {
      needsDisplay = true
    }
  }

  var shouldDrawSizeLabel = false {
    didSet {
      needsDisplay = true
    }
  }

  override var isFlipped: Bool {
    return true
  }

  init(capture: CachedDisplayCapture) {
    self.capture = capture
    super.init(frame: NSRect(origin: .zero, size: capture.logicalBounds.size))
    wantsLayer = true
    layer?.backgroundColor = NSColor.clear.cgColor
  }

  @available(*, unavailable)
  required init?(coder: NSCoder) {
    fatalError("init(coder:) has not been implemented")
  }

  override func acceptsFirstMouse(for event: NSEvent?) -> Bool {
    return true
  }

  override func resetCursorRects() {
    discardCursorRects()
    addCursorRect(bounds, cursor: .crosshair)
  }

  override func draw(_ dirtyRect: NSRect) {
    super.draw(dirtyRect)

    guard let context = NSGraphicsContext.current?.cgContext else {
      return
    }

    context.saveGState()
    context.interpolationQuality = .high
    // `CGContext.draw` still uses a bottom-left image coordinate system even when the NSView is
    // flipped. Drawing without an explicit Y flip makes every monitor preview appear upside down.
    context.translateBy(x: 0, y: bounds.height)
    context.scaleBy(x: 1, y: -1)
    context.draw(capture.image, in: NSRect(origin: .zero, size: bounds.size))
    context.scaleBy(x: 1, y: -1)
    context.translateBy(x: 0, y: -bounds.height)

    let overlayPath = NSBezierPath(rect: bounds)
    if let localSelection = localSelectionRect() {
      overlayPath.appendRect(localSelection)
      overlayPath.windingRule = .evenOdd
    }
    ScreenshotOverlayView.overlayColor.setFill()
    overlayPath.fill()

    if let localSelection = localSelectionRect() {
      drawSelectionBorder(in: localSelection)
      if shouldDrawSizeLabel, let globalSelection = globalSelection {
        drawSelectionSizeLabel(for: globalSelection)
      }
    }

    context.restoreGState()
  }

  private func localSelectionRect() -> NSRect? {
    guard let globalSelection = globalSelection, !globalSelection.isEmpty else {
      return nil
    }

    let intersection = capture.logicalBounds.intersection(globalSelection)
    if intersection.isEmpty {
      return nil
    }

    return NSRect(
      x: intersection.minX - capture.logicalBounds.minX,
      y: intersection.minY - capture.logicalBounds.minY,
      width: intersection.width,
      height: intersection.height
    )
  }

  private func drawSelectionBorder(in localSelection: NSRect) {
    let borderRect = localSelection.insetBy(dx: 1, dy: 1)
    let shadow = NSShadow()
    shadow.shadowBlurRadius = 18
    shadow.shadowOffset = NSSize(width: 0, height: 0)
    shadow.shadowColor = ScreenshotOverlayView.selectionShadowColor
    shadow.set()

    let borderPath = NSBezierPath(rect: borderRect)
    borderPath.lineWidth = 2
    ScreenshotOverlayView.selectionBorderColor.setStroke()
    borderPath.stroke()
  }

  private func drawSelectionSizeLabel(for globalSelection: NSRect) {
    let label = "\(Int(globalSelection.width.rounded())) x \(Int(globalSelection.height.rounded()))"
    let attributes: [NSAttributedString.Key: Any] = [
      .font: NSFont.systemFont(ofSize: 14, weight: .bold),
      .foregroundColor: NSColor.white,
    ]
    let labelSize = label.size(withAttributes: attributes)
    let labelWidth = labelSize.width + 16
    let labelHeight = labelSize.height + 8
    let preferredAboveY = globalSelection.minY - labelHeight - 10
    let labelY =
      preferredAboveY >= capture.logicalBounds.minY + 8
      ? preferredAboveY
      : min(globalSelection.maxY + 10, capture.logicalBounds.maxY - labelHeight - 8)
    let labelX = min(
      max(globalSelection.minX + 12, capture.logicalBounds.minX + 8),
      capture.logicalBounds.maxX - labelWidth - 8
    )
    let localRect = NSRect(
      x: labelX - capture.logicalBounds.minX,
      y: labelY - capture.logicalBounds.minY,
      width: labelWidth,
      height: labelHeight
    )

    let backgroundPath = NSBezierPath(roundedRect: localRect, xRadius: 10, yRadius: 10)
    ScreenshotOverlayView.labelBackgroundColor.setFill()
    backgroundPath.fill()
    label.draw(
      at: CGPoint(x: localRect.minX + 8, y: localRect.minY + 4),
      withAttributes: attributes
    )
  }
}

private final class ScreenshotOverlayWindow: NSWindow {
  let overlayView: ScreenshotOverlayView

  override var canBecomeKey: Bool {
    return true
  }

  override var canBecomeMain: Bool {
    return true
  }

  init(capture: CachedDisplayCapture) {
    overlayView = ScreenshotOverlayView(capture: capture)
    super.init(
      contentRect: appKitRect(fromTopLeftRect: capture.logicalBounds),
      styleMask: .borderless,
      backing: .buffered,
      defer: false
    )

    isOpaque = false
    backgroundColor = .clear
    hasShadow = false
    // The overlay session owns these windows explicitly. Releasing them as a side effect of close()
    // makes lifetime depend on AppKit event timing and was the most likely source of the drag-time crash.
    isReleasedWhenClosed = false
    level = screenshotWindowLevel()
    ignoresMouseEvents = false
    acceptsMouseMovedEvents = true
    titleVisibility = .hidden
    titlebarAppearsTransparent = true
    animationBehavior = .none
    collectionBehavior = screenshotCollectionBehavior()

    contentView = overlayView
  }
}

private final class ScreenshotOverlaySession {
  private let workspaceBounds: NSRect
  private let captures: [CachedDisplayCapture]
  private let windows: [ScreenshotOverlayWindow]
  private let onPreferredDisplayChanged: (CachedDisplayCapture) -> Void
  private let onComplete: (NativeSelectionOverlayResult) -> Void
  private var localEventMonitor: Any?
  private var dragStart: CGPoint?
  private var hoverSelection: NSRect?
  private var pendingHoverSelection: NSRect?
  private var isCompleting = false
  private var overlaysDismissed = false
  private var lastPreferredDisplayId: String?
  private let currentProcessId = pid_t(ProcessInfo.processInfo.processIdentifier)
  private let systemWideAccessibilityElement = AXUIElementCreateSystemWide()

  init(
    workspaceBounds: NSRect,
    captures: [CachedDisplayCapture],
    onPreferredDisplayChanged: @escaping (CachedDisplayCapture) -> Void,
    onComplete: @escaping (NativeSelectionOverlayResult) -> Void
  ) {
    self.workspaceBounds = workspaceBounds
    self.captures = captures
    self.windows = captures.map(ScreenshotOverlayWindow.init)
    self.onPreferredDisplayChanged = onPreferredDisplayChanged
    self.onComplete = onComplete
  }

  func begin() {
    installEventMonitor()
    // Build the first hover preview before the selector windows are ordered front. AX and
    // CGWindow hit-testing can then see the user's original desktop target instead of our overlay.
    updateHoverSelection(at: topLeftPoint(fromAppKit: NSEvent.mouseLocation))
    for window in windows {
      window.orderFrontRegardless()
    }
    windows.first?.makeKeyAndOrderFront(nil)
    NSApp.activate(ignoringOtherApps: true)
  }

  func cancel() {
    complete(with: NativeSelectionOverlayResult(selection: nil, editorVisibleBounds: nil))
  }

  func dismissOverlays() {
    if overlaysDismissed {
      return
    }

    overlaysDismissed = true
    let windowsToDismiss = windows
    DispatchQueue.main.async {
      for window in windowsToDismiss {
        window.orderOut(nil)
        window.contentView = nil
        window.close()
      }
    }
  }

  private func installEventMonitor() {
    localEventMonitor = NSEvent.addLocalMonitorForEvents(matching: [.mouseMoved, .leftMouseDown, .leftMouseDragged, .leftMouseUp, .keyDown]) { [weak self] event in
      return self?.handle(event: event) ?? event
    }
  }

  private func handle(event: NSEvent) -> NSEvent? {
    if isCompleting {
      return nil
    }

    switch event.type {
    case .keyDown:
      if event.keyCode == 53 {
        cancel()
        return nil
      }

      return nil

    case .mouseMoved:
      guard dragStart == nil else {
        return nil
      }

      // Hover previews are resolved only before a real drag starts. Once dragging begins the
      // selector keeps the existing manual rectangle path so the new feature cannot alter drag
      // selection semantics.
      updateHoverSelection(at: topLeftPoint(fromAppKit: NSEvent.mouseLocation))
      return nil

    case .leftMouseDown:
      let point = clampPoint(topLeftPoint(fromAppKit: NSEvent.mouseLocation), to: workspaceBounds)
      pendingHoverSelection = hoverSelection
      dragStart = point
      self.hoverSelection = nil
      let selection = rectFromPoints(point, point)
      // Bug fix: hover click completion is deferred until mouse-up. Mouse-down records the hover
      // candidate but still starts the normal drag path, so users can press on a preview and drag
      // a manual rectangle instead of being forced into the hovered control.
      updateSelection(selection)
      updatePreferredDisplayHint(for: selection)
      return nil

    case .leftMouseDragged:
      guard let dragStart else {
        return nil
      }

      hoverSelection = nil
      let point = clampPoint(topLeftPoint(fromAppKit: NSEvent.mouseLocation), to: workspaceBounds)
      let selection = rectFromPoints(dragStart, point)
      // Mouse-up used to be the first moment Flutter learned which monitor would host annotation,
      // so the visible handoff still had to prepare the new backdrop. Emitting drag-time hints here
      // gives Flutter the whole drag gesture to prewarm the hidden editor on the likely target screen.
      updateSelection(selection)
      updatePreferredDisplayHint(for: selection)
      return nil

    case .leftMouseUp:
      guard let dragStart else {
        return nil
      }

      let point = clampPoint(topLeftPoint(fromAppKit: NSEvent.mouseLocation), to: workspaceBounds)
      let selection = rectFromPoints(dragStart, point)
      if let pendingHoverSelection, !isManualScreenshotSelection(selection) {
        // Bug fix: a plain click on a hover preview still enters annotation, but only after mouse-up
        // proves that the user did not draw a manual rectangle from the same starting point.
        self.pendingHoverSelection = nil
        completeSelection(pendingHoverSelection)
      } else {
        self.pendingHoverSelection = nil
        completeSelection(selection)
      }
      return nil

    default:
      return event
    }
  }

  private func updateHoverSelection(at topLeftPoint: CGPoint) {
    let point = clampPoint(topLeftPoint, to: workspaceBounds)
    let selection = accessibilityHoverSelection(at: point) ?? windowHoverSelection(at: point)

    if hoverSelection == selection {
      return
    }

    hoverSelection = selection
    // Hover selection intentionally reuses the existing selection drawing path instead of changing
    // the captured backdrop or rebuilding any region. That keeps mouse-move updates cheap while
    // still making the control-level target visible before the user clicks.
    updateSelection(selection)
    if let selection {
      updatePreferredDisplayHint(for: selection)
    }
  }

  private func updateSelection(_ selection: NSRect?) {
    let labelDisplayId = selection.flatMap(selectionLabelDisplayId(for:))
    for window in windows {
      window.overlayView.globalSelection = selection
      window.overlayView.shouldDrawSizeLabel = window.overlayView.capture.displayId == labelDisplayId
    }
  }

  private func updatePreferredDisplayHint(for selection: NSRect) {
    guard let preferredCapture = preferredCapture(for: selection) else {
      return
    }

    if preferredCapture.displayId == lastPreferredDisplayId {
      return
    }

    lastPreferredDisplayId = preferredCapture.displayId
    onPreferredDisplayChanged(preferredCapture)
  }

  private func completeSelection(_ selection: NSRect) {
    if selection.width < 1 || selection.height < 1 {
      cancel()
      return
    }

    let editorVisibleBounds = preferredEditorVisibleBounds(for: selection)
    complete(with: NativeSelectionOverlayResult(selection: selection, editorVisibleBounds: editorVisibleBounds))
  }

  private func complete(with result: NativeSelectionOverlayResult) {
    if isCompleting {
      return
    }

    isCompleting = true
    dragStart = nil
    pendingHoverSelection = nil
    if let localEventMonitor {
      NSEvent.removeMonitor(localEventMonitor)
      self.localEventMonitor = nil
    }

    // The drag phase ends here, but successful handoff keeps the overlay windows alive so Flutter
    // can reuse the exact same captured backdrop underneath its annotation controls. Cancel still
    // closes immediately because there is no follow-up editor that needs the native background.
    if result.selection == nil {
      dismissOverlays()
    }

    let onComplete = self.onComplete
    DispatchQueue.main.async {
      onComplete(result)
    }
  }

  private func selectionLabelDisplayId(for selection: NSRect) -> String? {
    let anchorPoint = CGPoint(x: selection.minX + 12, y: selection.minY + 12)
    if let containingCapture = captures.first(where: { $0.logicalBounds.contains(anchorPoint) }) {
      return containingCapture.displayId
    }

    if let originCapture = captures.first(where: { $0.logicalBounds.contains(selection.origin) }) {
      return originCapture.displayId
    }

    return preferredCapture(for: selection)?.displayId
  }

  private func accessibilityHoverSelection(at point: CGPoint) -> NSRect? {
    let hitTestPoint = quartzPoint(fromWorkspaceTopLeftPoint: point)
    var hitElement: AXUIElement?
    let error = AXUIElementCopyElementAtPosition(systemWideAccessibilityElement, Float(hitTestPoint.x), Float(hitTestPoint.y), &hitElement)
    guard error == .success, let hitElement else {
      return nil
    }

    var currentElement: AXUIElement? = hitElement
    var fallbackSelection: NSRect?

    for _ in 0..<8 {
      guard let element = currentElement else {
        break
      }

      let role = accessibilityRole(for: element)
      let subrole = accessibilitySubrole(for: element)
      if isSelectableAccessibilityElement(element, role: role, subrole: subrole),
        let rect = accessibilityRect(for: element),
        let selection = validatedHoverSelection(rect, at: point)
      {
        if isPreferredAccessibilityRole(role) {
          return selection
        }

        fallbackSelection = fallbackSelection ?? selection
      }

      currentElement = accessibilityParent(for: element)
    }

    // AX hit-testing often starts on text or image leaves in web content. Walking upward lets the
    // preview land on a meaningful row, button, or input, while this fallback still gives a usable
    // rect when an app exposes no better parent.
    return fallbackSelection
  }

  private func accessibilityRect(for element: AXUIElement) -> NSRect? {
    var positionValue: CFTypeRef?
    var sizeValue: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, kAXPositionAttribute as CFString, &positionValue) == .success,
      AXUIElementCopyAttributeValue(element, kAXSizeAttribute as CFString, &sizeValue) == .success,
      let positionValue,
      let sizeValue,
      CFGetTypeID(positionValue) == AXValueGetTypeID(),
      CFGetTypeID(sizeValue) == AXValueGetTypeID()
    else {
      return nil
    }

    let positionAXValue = positionValue as! AXValue
    let sizeAXValue = sizeValue as! AXValue
    var position = CGPoint.zero
    var size = CGSize.zero
    guard AXValueGetType(positionAXValue) == .cgPoint,
      AXValueGetType(sizeAXValue) == .cgSize,
      AXValueGetValue(positionAXValue, .cgPoint, &position),
      AXValueGetValue(sizeAXValue, .cgSize, &size)
    else {
      return nil
    }

    return workspaceTopLeftRect(fromQuartzRect: NSRect(origin: position, size: size))
  }

  private func accessibilityParent(for element: AXUIElement) -> AXUIElement? {
    var parentValue: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, kAXParentAttribute as CFString, &parentValue) == .success,
      let parentValue,
      CFGetTypeID(parentValue) == AXUIElementGetTypeID()
    else {
      return nil
    }

    return (parentValue as! AXUIElement)
  }

  private func accessibilityRole(for element: AXUIElement) -> String? {
    var roleValue: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, kAXRoleAttribute as CFString, &roleValue) == .success else {
      return nil
    }

    return roleValue as? String
  }

  private func accessibilitySubrole(for element: AXUIElement) -> String? {
    var subroleValue: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, kAXSubroleAttribute as CFString, &subroleValue) == .success else {
      return nil
    }

    return subroleValue as? String
  }

  private func isHiddenAccessibilityElement(_ element: AXUIElement) -> Bool {
    var hiddenValue: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, kAXHiddenAttribute as CFString, &hiddenValue) == .success else {
      return false
    }

    return (hiddenValue as? Bool) ?? false
  }

  private func hasAccessibilityWindowAssociation(_ element: AXUIElement, role: String?) -> Bool {
    if role == kAXWindowRole as String {
      return true
    }

    var windowValue: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, kAXWindowAttribute as CFString, &windowValue) == .success,
      let windowValue
    else {
      return false
    }

    return CFGetTypeID(windowValue) == AXUIElementGetTypeID()
  }

  private func isSelectableAccessibilityElement(_ element: AXUIElement, role: String?, subrole: String?) -> Bool {
    if isCurrentProcessAccessibilityElement(element) || isHiddenAccessibilityElement(element) {
      return false
    }

    let ignoredRoles = [
      "AXApplication",
      "AXSystemWide",
      "AXDesktop",
      "AXTitleBar",
      "AXToolbar",
      "AXMenu",
      "AXMenuBar",
      "AXMenuBarItem",
      "AXMenuItem",
      "AXDockItem",
      "AXStatusItem",
    ]
    let ignoredSubroles = [
      "AXDesktop",
      "AXCloseButton",
      "AXMinimizeButton",
      "AXZoomButton",
      "AXFullScreenButton",
      "AXToolbarButton",
    ]
    if let role, ignoredRoles.contains(role) {
      return false
    }
    if let subrole, ignoredSubroles.contains(subrole) {
      return false
    }

    // Bug fix: AX can hit Finder/Desktop, wallpaper, or titlebar chrome nodes that expose geometry
    // but are not useful screenshot targets. Requiring a window association keeps content controls
    // selectable while the role/subrole deny-list skips window buttons and menu/toolbar chrome.
    return hasAccessibilityWindowAssociation(element, role: role)
  }

  private func isPreferredAccessibilityRole(_ role: String?) -> Bool {
    guard let role else {
      return true
    }

    // Text and image leaves usually describe the visual content under the cursor, not the clickable
    // control or row the user wants to capture. Prefer their parent when AX exposes one.
    let leafRoles = [
      kAXStaticTextRole as String,
      kAXImageRole as String,
    ]
    return !leafRoles.contains(role)
  }

  private func isCurrentProcessAccessibilityElement(_ element: AXUIElement) -> Bool {
    var pid: pid_t = 0
    return AXUIElementGetPid(element, &pid) == .success && pid == currentProcessId
  }

  private func windowHoverSelection(at point: CGPoint) -> NSRect? {
    guard let windowInfos = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else {
      return nil
    }

    let hitTestPoint = quartzPoint(fromWorkspaceTopLeftPoint: point)
    for windowInfo in windowInfos {
      let ownerPid = (windowInfo[kCGWindowOwnerPID as String] as? NSNumber)?.int32Value ?? 0
      if ownerPid == currentProcessId {
        continue
      }

      let layer = (windowInfo[kCGWindowLayer as String] as? NSNumber)?.intValue ?? 0
      if layer != 0 {
        continue
      }

      let alpha = (windowInfo[kCGWindowAlpha as String] as? NSNumber)?.doubleValue ?? 1
      if alpha <= 0 {
        continue
      }

      guard let boundsDictionary = windowInfo[kCGWindowBounds as String] as? NSDictionary,
        let rect = CGRect(dictionaryRepresentation: boundsDictionary as CFDictionary)
      else {
        continue
      }

      let windowRect = NSRect(x: rect.origin.x, y: rect.origin.y, width: rect.width, height: rect.height)
      if !windowRect.contains(hitTestPoint) {
        continue
      }

      let workspaceWindowRect = workspaceTopLeftRect(fromQuartzRect: windowRect)
      if isDesktopLikeWindowInfo(windowInfo, workspaceRect: workspaceWindowRect, at: point) {
        continue
      }

      // The window-level fallback keeps the feature useful when AX is unavailable because the user
      // has not granted accessibility permission, or when an app exposes unreliable control bounds.
      return validatedHoverSelection(workspaceWindowRect, at: point, allowDisplaySizedCandidate: true)
    }

    return nil
  }

  private func isDesktopLikeWindowInfo(_ windowInfo: [String: Any], workspaceRect: NSRect, at point: CGPoint) -> Bool {
    // Bug fix: CGWindow fallback can still list wallpaper, Dock, or Finder desktop surfaces as
    // layer-0 windows. Geometry alone cannot distinguish those from real full-screen apps, so use
    // owner/name hints only for known desktop surfaces while keeping normal app windows selectable.
    let ownerName = windowInfo[kCGWindowOwnerName as String] as? String ?? ""
    let windowName = windowInfo[kCGWindowName as String] as? String ?? ""
    if ownerName == "Window Server" || ownerName == "Dock" {
      return true
    }

    guard let containingCapture = captures.first(where: { $0.logicalBounds.contains(point) }) else {
      return true
    }

    let selection = workspaceRect.intersection(containingCapture.logicalBounds).intersection(workspaceBounds)
    let looksDisplaySized = !selection.isEmpty && isDisplaySizedScreenshotHoverCandidate(selection, displayBounds: containingCapture.logicalBounds)
    if looksDisplaySized && ownerName == "Finder" && (windowName.isEmpty || windowName.localizedCaseInsensitiveContains("desktop")) {
      return true
    }
    if looksDisplaySized && (windowName.localizedCaseInsensitiveContains("desktop") || windowName.localizedCaseInsensitiveContains("wallpaper")) {
      return true
    }

    return false
  }

  private func validatedHoverSelection(_ rect: NSRect, at point: CGPoint, allowDisplaySizedCandidate: Bool = false) -> NSRect? {
    let values = [rect.minX, rect.minY, rect.width, rect.height]
    if values.contains(where: { !$0.isFinite }) || isTooSmallForScreenshotHover(rect) {
      return nil
    }

    guard let containingCapture = captures.first(where: { $0.logicalBounds.contains(point) }) else {
      return nil
    }

    if rect.width > containingCapture.logicalBounds.width + 2 || rect.height > containingCapture.logicalBounds.height + 2 {
      return nil
    }

    let selection = rect.intersection(containingCapture.logicalBounds).intersection(workspaceBounds)
    if selection.isEmpty || isTooSmallForScreenshotHover(selection) {
      return nil
    }
    if !allowDisplaySizedCandidate && isDisplaySizedScreenshotHoverCandidate(selection, displayBounds: containingCapture.logicalBounds) {
      // Bug fix: AX may report desktop/wallpaper or full-display containers as ordinary elements
      // even though the pointer is over empty background. Reject display-sized AX candidates so the
      // selector only previews real controls, with CGWindow fallback handling actual app windows.
      return nil
    }

    return selection
  }

  private func preferredEditorVisibleBounds(for selection: NSRect) -> NSRect? {
    return preferredCapture(for: selection)?.visibleBounds
  }

  private func preferredCapture(for selection: NSRect) -> CachedDisplayCapture? {
    let selectionCenter = CGPoint(x: selection.midX, y: selection.midY)
    if let centeredCapture = captures.first(where: { $0.logicalBounds.contains(selectionCenter) }) {
      return centeredCapture
    }

    return captures.max(by: { intersectionArea($0.logicalBounds, selection) < intersectionArea($1.logicalBounds, selection) })
  }
}

private final class ScrollingCaptureOverlayView: NSView {
  private let selectionRect: NSRect
  private let overlayColor = NSColor(calibratedWhite: 0, alpha: 0.46)
  private let borderColor = NSColor(red: 41 / 255, green: 1, blue: 114 / 255, alpha: 1)

  init(frame: NSRect, selectionRect: NSRect) {
    self.selectionRect = selectionRect
    super.init(frame: frame)
  }

  required init?(coder: NSCoder) {
    return nil
  }

  override var isOpaque: Bool { false }

  override func draw(_ dirtyRect: NSRect) {
    super.draw(dirtyRect)

    // Drawing the four dimmed bands in one mouse-transparent window avoids compositor seams that
    // appeared when the same visual mask was split across four adjacent NSWindows. The center still
    // receives native mouse and text interaction because the whole overlay window ignores events.
    overlayColor.setFill()
    NSBezierPath(rect: NSRect(x: bounds.minX, y: bounds.minY, width: bounds.width, height: max(0, selectionRect.minY - bounds.minY))).fill()
    NSBezierPath(rect: NSRect(x: bounds.minX, y: selectionRect.maxY, width: bounds.width, height: max(0, bounds.maxY - selectionRect.maxY))).fill()
    NSBezierPath(rect: NSRect(x: bounds.minX, y: selectionRect.minY, width: max(0, selectionRect.minX - bounds.minX), height: selectionRect.height)).fill()
    NSBezierPath(rect: NSRect(x: selectionRect.maxX, y: selectionRect.minY, width: max(0, bounds.maxX - selectionRect.maxX), height: selectionRect.height)).fill()

    let borderPath = NSBezierPath(rect: selectionRect.insetBy(dx: 1, dy: 1))
    borderPath.lineWidth = 2
    borderColor.setStroke()
    borderPath.stroke()
  }
}

private final class ScrollingCaptureOverlaySession {
  private let selection: NSRect
  private let overlayWindow: NSWindow
  private let controlsWindow: NSWindow
  private let controlsBounds: NSRect
  private let onScroll: (Double, Double) -> Void
  private let onTimingLog: (String) -> Void
  private var scrollMonitor: Any?

  init(workspaceBounds: NSRect, selection: NSRect, controlsWindow: NSWindow, controlsBounds: NSRect, onScroll: @escaping (Double, Double) -> Void, onTimingLog: @escaping (String) -> Void) {
    self.selection = selection
    self.controlsWindow = controlsWindow
    self.controlsBounds = controlsBounds
    self.onScroll = onScroll
    self.onTimingLog = onTimingLog

    let localSelection = NSRect(x: selection.minX - workspaceBounds.minX, y: workspaceBounds.maxY - selection.maxY, width: selection.width, height: selection.height)
    overlayWindow = NSWindow(contentRect: appKitRect(fromTopLeftRect: workspaceBounds), styleMask: .borderless, backing: .buffered, defer: false)
    overlayWindow.isOpaque = false
    overlayWindow.backgroundColor = .clear
    overlayWindow.hasShadow = false
    overlayWindow.isReleasedWhenClosed = false
    overlayWindow.level = screenshotWindowLevel()
    overlayWindow.ignoresMouseEvents = true
    overlayWindow.acceptsMouseMovedEvents = false
    overlayWindow.titleVisibility = .hidden
    overlayWindow.titlebarAppearsTransparent = true
    overlayWindow.animationBehavior = .none
    overlayWindow.collectionBehavior = screenshotCollectionBehavior()
    overlayWindow.contentView = ScrollingCaptureOverlayView(frame: NSRect(x: 0, y: 0, width: workspaceBounds.width, height: workspaceBounds.height), selectionRect: localSelection)
  }

  func begin() {
    let beginStart = Date()
    overlayWindow.orderFrontRegardless()
    moveControlsWindow()
    installScrollMonitor()
    onTimingLog("event=native_overlay_session_begin_done selection=\(formatTimingRect(selection)) controls=\(formatTimingRect(controlsBounds)) elapsedMs=\(elapsedMilliseconds(since: beginStart))")
  }

  func dismiss() {
    if let scrollMonitor {
      NSEvent.removeMonitor(scrollMonitor)
      self.scrollMonitor = nil
    }

    overlayWindow.orderOut(nil)
    overlayWindow.contentView = nil
    overlayWindow.close()
  }

  private func moveControlsWindow() {
    let moveStart = Date()
    controlsWindow.setFrame(appKitRect(fromTopLeftRect: controlsBounds), display: true)
    makeScreenshotWindowTransparent(controlsWindow)
    controlsWindow.contentView?.needsLayout = true
    controlsWindow.contentView?.layoutSubtreeIfNeeded()
    controlsWindow.displayIfNeeded()
    applyScreenshotWindowPresentation(to: controlsWindow, level: scrollingCaptureControlsWindowLevel())
    controlsWindow.orderFrontRegardless()
    controlsWindow.makeKeyAndOrderFront(nil)
    // Ordering can cause AppKit to reconsider NSPanel presentation, so reassert the explicit
    // scrolling-preview level after fronting the window as well as before it.
    controlsWindow.level = scrollingCaptureControlsWindowLevel()
    NSApp.activate(ignoringOtherApps: true)
    onTimingLog("event=native_controls_window_shown controls=\(formatTimingRect(controlsBounds)) elapsedMs=\(elapsedMilliseconds(since: moveStart))")
  }

  private func installScrollMonitor() {
    scrollMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.scrollWheel]) { [weak self] event in
      guard let self else {
        return
      }

      let point = topLeftPoint(fromAppKit: NSEvent.mouseLocation)
      if self.selection.contains(point) {
        let rawDeltaY = event.scrollingDeltaY
        // Bug fix: native scroll events must carry direction so Dart can prepend when the user
        // scrolls upward. AppKit's raw delta uses the opposite sign from Flutter's PointerScrollEvent
        // convention used by the synthetic scroll path, so normalize it here and log both values.
        let normalizedDeltaY = -rawDeltaY
        self.onTimingLog("event=native_scroll_monitor_hit selection=\(formatTimingRect(self.selection)) rawDeltaY=\(rawDeltaY) deltaY=\(normalizedDeltaY)")
        self.onScroll(normalizedDeltaY, rawDeltaY)
      }
    }
  }
}

@main
class AppDelegate: FlutterAppDelegate {
  // The screenshot capture helpers run inside Swift `throws` / `async throws` contexts. The
  // previous implementation threw `FlutterError` directly, but the macOS Flutter SDK exposes it
  // as an Objective-C channel payload rather than a Swift `Error`, which now fails compilation.
  // Keep a Swift-native error for internal control flow and convert it back at the channel
  // boundary so Dart still receives the same error codes and messages as before.
  private struct DisplayCaptureError: LocalizedError {
    let code: String
    let message: String
    let details: Any?

    var errorDescription: String? {
      return message
    }

    func asFlutterError() -> FlutterError {
      return FlutterError(code: code, message: message, details: details)
    }
  }

  private struct ScreenshotPresentationState {
    let collectionBehavior: NSWindow.CollectionBehavior
    let level: NSWindow.Level
    let animationBehavior: NSWindow.AnimationBehavior
    let isOpaque: Bool
    let backgroundColor: NSColor?
    let hasShadow: Bool
    let styleMask: NSWindow.StyleMask
    let ignoresMouseEvents: Bool
    let acceptsMouseMovedEvents: Bool
    let titleVisibility: NSWindow.TitleVisibility
    let titlebarAppearsTransparent: Bool
    let closeButtonHidden: Bool
    let miniaturizeButtonHidden: Bool
    let zoomButtonHidden: Bool
    let panelHidesOnDeactivate: Bool?
    let panelBecomesKeyOnlyIfNeeded: Bool?
    let panelIsFloating: Bool?
    let hadAcrylicEffect: Bool
  }

  // Store the previous active application
  private var previousActiveApp: NSRunningApplication?
  // Only restore the previous app when Wox has stayed focused since the last show/focus.
  private var shouldRestorePreviousAppOnHide = false
  // Flutter method channel for window events
  private var windowEventChannel: FlutterMethodChannel?
  // Screenshot prewarm events use their own channel so drag-time hints do not interfere with the
  // main window manager handler that already owns blur and debug traffic.
  private var screenshotEventChannel: FlutterMethodChannel?
  private var resultDragBridge: ResultDragBridge?
  // Current appearance (light/dark)
  private var currentAppearance: String = "light"
  private var screenshotPresentationState: ScreenshotPresentationState?
  private var isCapturePresentationActive = false
  private var captureWorkspaceBounds = NSRect.zero
  private var captureWorkspaceScale = 1.0
  // Native multi-display selection reuses the already captured CGImages so the selector can draw
  // one full-resolution background per monitor without sending large image payloads back to Swift.
  private var cachedDisplayCaptures: [CachedDisplayCapture] = []
  private var activeOverlaySession: ScreenshotOverlaySession?
  private var activeScrollingCaptureOverlaySession: ScrollingCaptureOverlaySession?
  private var activeScrollingCaptureTraceId = ""
  private var nativeOverlayDismissTimeoutWorkItem: DispatchWorkItem?

  // Carries both a resolved file path and whether macOS' image wallpaper extension owns the current Space.
  private struct WallpaperStoreResolution {
    let imagePath: String?
    let usesImageExtension: Bool
  }

  private func describeFrontmostApplication() -> String {
    guard let frontApp = NSWorkspace.shared.frontmostApplication else {
      return "Unknown (bundleID: Unknown)"
    }

    return "\(frontApp.localizedName ?? "Unknown") (bundleID: \(frontApp.bundleIdentifier ?? "Unknown"))"
  }

  private func savePreviousActiveAppIfNeeded() {
    if let frontApp = NSWorkspace.shared.frontmostApplication,
      frontApp != NSRunningApplication.current,
      !frontApp.isTerminated
    {
      log(
        "Saving previous active app: \(frontApp.localizedName ?? "Unknown") (bundleID: \(frontApp.bundleIdentifier ?? "Unknown"))"
      )
      previousActiveApp = frontApp
      shouldRestorePreviousAppOnHide = true
    } else {
      log("No new previous app to save, keeping existing restore state")
    }
  }

  private func log(_ message: String, traceId: String? = nil) {
    DispatchQueue.main.async { [weak self] in
      if let traceId, !traceId.isEmpty {
        self?.windowEventChannel?.invokeMethod("log", arguments: ["traceId": traceId, "message": message])
      } else {
        self?.windowEventChannel?.invokeMethod("log", arguments: message)
      }
    }
  }

  // Resolve the active desktop wallpaper without System Events automation permission.
  private func desktopWallpaperPath() -> String? {
    let storeResolution = desktopWallpaperPathFromStore()
    if let path = storeResolution.imagePath {
      return path
    }

    if storeResolution.usesImageExtension, let path = latestWallpaperAgentImageCachePath() {
      return path
    }

    var screens = NSScreen.screens
    if let mainScreen = NSScreen.main {
      screens.insert(mainScreen, at: 0)
    }
    var seenScreens = Set<ObjectIdentifier>()

    for screen in screens {
      let screenId = ObjectIdentifier(screen)
      if seenScreens.contains(screenId) {
        continue
      }
      seenScreens.insert(screenId)

      guard
        let url = NSWorkspace.shared.desktopImageURL(for: screen),
        url.isFileURL
      else {
        continue
      }

      let path = url.path
      if FileManager.default.fileExists(atPath: path) {
        return path
      }
    }

    return nil
  }

  // Resolve static image wallpaper choices from macOS' current Space wallpaper store.
  private func desktopWallpaperPathFromStore() -> WallpaperStoreResolution {
    let storeURL = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support/com.apple.wallpaper/Store/Index.plist")
    guard
      let data = try? Data(contentsOf: storeURL),
      let root = try? PropertyListSerialization.propertyList(from: data, options: [], format: nil) as? [String: Any]
    else {
      return WallpaperStoreResolution(imagePath: nil, usesImageExtension: false)
    }

    var usesImageExtension = false
    for spaceID in currentDesktopSpaceIdentifiers() {
      let resolution = desktopWallpaperPath(fromSpaceID: spaceID, root: root)
      usesImageExtension = usesImageExtension || resolution.usesImageExtension
      if let imagePath = resolution.imagePath {
        return WallpaperStoreResolution(imagePath: imagePath, usesImageExtension: usesImageExtension)
      }
    }

    if let displays = root["Displays"] as? [String: Any] {
      for display in displays.values {
        guard let display = display as? [String: Any] else {
          continue
        }
        let resolution = wallpaperStoreResolution(fromDesktop: display["Desktop"] as? [String: Any])
        usesImageExtension = usesImageExtension || resolution.usesImageExtension
        if let imagePath = resolution.imagePath {
          return WallpaperStoreResolution(imagePath: imagePath, usesImageExtension: usesImageExtension)
        }
      }
    }

    return WallpaperStoreResolution(imagePath: nil, usesImageExtension: usesImageExtension)
  }

  // Read the active macOS Space IDs so per-Space wallpapers override global defaults.
  private func currentDesktopSpaceIdentifiers() -> [String] {
    let spacesURL = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Preferences/com.apple.spaces.plist")
    guard
      let data = try? Data(contentsOf: spacesURL),
      let root = try? PropertyListSerialization.propertyList(from: data, options: [], format: nil) as? [String: Any],
      let configuration = root["SpacesDisplayConfiguration"] as? [String: Any],
      let managementData = configuration["Management Data"] as? [String: Any],
      let monitors = managementData["Monitors"] as? [[String: Any]]
    else {
      return [""]
    }

    var spaceIDs: [String] = []
    for monitor in monitors {
      guard
        let currentSpace = monitor["Current Space"] as? [String: Any],
        let uuid = currentSpace["uuid"] as? String
      else {
        continue
      }
      if !spaceIDs.contains(uuid) {
        spaceIDs.append(uuid)
      }
    }

    if !spaceIDs.contains("") {
      spaceIDs.append("")
    }
    return spaceIDs
  }

  // Resolve a wallpaper choice from a specific Space entry in macOS' wallpaper store.
  private func desktopWallpaperPath(fromSpaceID spaceID: String, root: [String: Any]) -> WallpaperStoreResolution {
    guard
      let spaces = root["Spaces"] as? [String: Any],
      let space = spaces[spaceID] as? [String: Any]
    else {
      return WallpaperStoreResolution(imagePath: nil, usesImageExtension: false)
    }

    var usesImageExtension = false
    if let displays = space["Displays"] as? [String: Any] {
      for display in displays.values {
        guard let display = display as? [String: Any] else {
          continue
        }
        let resolution = wallpaperStoreResolution(fromDesktop: display["Desktop"] as? [String: Any])
        usesImageExtension = usesImageExtension || resolution.usesImageExtension
        if let imagePath = resolution.imagePath {
          return WallpaperStoreResolution(imagePath: imagePath, usesImageExtension: usesImageExtension)
        }
      }
    }

    if let defaultSpace = space["Default"] as? [String: Any] {
      let resolution = wallpaperStoreResolution(fromDesktop: defaultSpace["Desktop"] as? [String: Any])
      usesImageExtension = usesImageExtension || resolution.usesImageExtension
      if let imagePath = resolution.imagePath {
        return WallpaperStoreResolution(imagePath: imagePath, usesImageExtension: usesImageExtension)
      }
    }

    return WallpaperStoreResolution(imagePath: nil, usesImageExtension: usesImageExtension)
  }

  // Decode one Desktop content entry and return either its source image or the provider kind.
  private func wallpaperStoreResolution(fromDesktop desktop: [String: Any]?) -> WallpaperStoreResolution {
    guard
      let content = desktop?["Content"] as? [String: Any],
      let choices = content["Choices"] as? [[String: Any]]
    else {
      return WallpaperStoreResolution(imagePath: nil, usesImageExtension: false)
    }

    var usesImageExtension = false
    for choice in choices {
      if choice["Provider"] as? String == "com.apple.wallpaper.extension.image" {
        usesImageExtension = true
      }

      if let imagePath = firstExistingImagePath(in: choice["Files"]) {
        return WallpaperStoreResolution(imagePath: imagePath, usesImageExtension: usesImageExtension)
      }

      guard
        let configurationData = choice["Configuration"] as? Data,
        let configuration = try? PropertyListSerialization.propertyList(from: configurationData, options: [], format: nil),
        let imagePath = firstExistingImagePath(in: configuration)
      else {
        continue
      }
      return WallpaperStoreResolution(imagePath: imagePath, usesImageExtension: usesImageExtension)
    }

    return WallpaperStoreResolution(imagePath: nil, usesImageExtension: usesImageExtension)
  }

  // Use WallpaperAgent's rendered image cache for photo shuffle choices that do not expose a source path.
  private func latestWallpaperAgentImageCachePath() -> String? {
    let cacheURL = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Containers/com.apple.wallpaper.agent/Data/Library/Caches/com.apple.wallpaper.caches/extension-com.apple.wallpaper.extension.image")
    guard let files = try? FileManager.default.contentsOfDirectory(at: cacheURL, includingPropertiesForKeys: [.contentModificationDateKey, .isRegularFileKey], options: [.skipsHiddenFiles]) else {
      return nil
    }

    let imageFiles = files.compactMap { url -> (url: URL, modificationDate: Date)? in
      guard
        isSupportedWallpaperImagePath(url.path),
        (try? url.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile) == true,
        let modificationDate = try? url.resourceValues(forKeys: [.contentModificationDateKey]).contentModificationDate
      else {
        return nil
      }
      return (url, modificationDate)
    }

    return imageFiles.max { $0.modificationDate < $1.modificationDate }?.url.path
  }

  // Walk decoded wallpaper configuration values because macOS stores file URLs under several provider-specific keys.
  private func firstExistingImagePath(in value: Any?) -> String? {
    if let path = value as? String {
      return existingImagePath(from: path)
    }

    if let values = value as? [Any] {
      for item in values {
        if let path = firstExistingImagePath(in: item) {
          return path
        }
      }
      return nil
    }

    if let dictionary = value as? [String: Any] {
      for key in ["relative", "url", "path", "absolute"] {
        if let path = firstExistingImagePath(in: dictionary[key]) {
          return path
        }
      }
      for item in dictionary.values {
        if let path = firstExistingImagePath(in: item) {
          return path
        }
      }
    }

    return nil
  }

  // Normalize file URLs and plain paths before accepting them as previewable wallpaper images.
  private func existingImagePath(from rawPath: String) -> String? {
    let path: String
    if let url = URL(string: rawPath), url.isFileURL {
      path = url.path
    } else {
      path = rawPath
    }

    guard isSupportedWallpaperImagePath(path), FileManager.default.fileExists(atPath: path) else {
      return nil
    }
    return path
  }

  // Keep non-image cache metadata out of the theme preview path.
  private func isSupportedWallpaperImagePath(_ path: String) -> Bool {
    let supportedExtensions = ["bmp", "gif", "heic", "jpeg", "jpg", "png", "tif", "tiff", "webp"]
    return supportedExtensions.contains(URL(fileURLWithPath: path).pathExtension.lowercased())
  }

  private func logScrollingCaptureTiming(_ fields: String, traceId: String? = nil) {
    let resolvedTraceId = traceId ?? activeScrollingCaptureTraceId
    guard !resolvedTraceId.isEmpty else {
      return
    }

    // Native timing probes must share the scrolling request trace with Dart so a single ui.log
    // filter shows the wheel input, native capture, decode/stitch, and preview repaint path.
    log("scrolling_capture_timing \(fields)", traceId: resolvedTraceId)
  }

  private func emitSelectionDisplayHint(_ capture: CachedDisplayCapture) {
    let payload: [String: Any] = [
      "displayId": capture.displayId,
      "displayBounds": buildRectPayload(capture.logicalBounds),
    ]

    DispatchQueue.main.async { [weak self] in
      self?.screenshotEventChannel?.invokeMethod("onSelectionDisplayHint", arguments: payload)
    }
  }

  private func keyCode(for key: String) -> CGKeyCode? {
    switch key.lowercased() {
    case "a": return 0
    case "s": return 1
    case "d": return 2
    case "f": return 3
    case "h": return 4
    case "g": return 5
    case "z": return 6
    case "x": return 7
    case "c": return 8
    case "v": return 9
    case "b": return 11
    case "q": return 12
    case "w": return 13
    case "e": return 14
    case "r": return 15
    case "y": return 16
    case "t": return 17
    case "1": return 18
    case "2": return 19
    case "3": return 20
    case "4": return 21
    case "6": return 22
    case "5": return 23
    case "9": return 25
    case "7": return 26
    case "8": return 28
    case "0": return 29
    case "o": return 31
    case "u": return 32
    case "i": return 34
    case "p": return 35
    case "l": return 37
    case "j": return 38
    case "k": return 40
    case "n": return 45
    case "m": return 46
    case "enter": return 36
    case "tab": return 48
    case "space": return 49
    case "escape": return 53
    case "meta": return 55
    case "shift": return 56
    case "alt": return 58
    case "control": return 59
    case "arrowleft": return 123
    case "arrowright": return 124
    case "arrowdown": return 125
    case "arrowup": return 126
    default: return nil
    }
  }

  private func mouseButton(for button: String) -> CGMouseButton? {
    switch button.lowercased() {
    case "left": return .left
    case "right": return .right
    case "middle": return .center
    default: return nil
    }
  }

  private func mouseEventTypes(for button: CGMouseButton) -> (down: CGEventType, up: CGEventType) {
    switch button {
    case .left:
      return (.leftMouseDown, .leftMouseUp)
    case .right:
      return (.rightMouseDown, .rightMouseUp)
    default:
      return (.otherMouseDown, .otherMouseUp)
    }
  }

  private func screenPoint(fromTopLeft point: CGPoint) -> CGPoint {
    return CGPoint(x: point.x, y: appKitY(fromTopLeftY: point.y))
  }

  private func screen(for displayId: CGDirectDisplayID) -> NSScreen? {
    return NSScreen.screens.first { screen in
      guard let screenNumber = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber else {
        return false
      }

      return CGDirectDisplayID(screenNumber.uint32Value) == displayId
    }
  }

  private func parseRect(arguments: [String: Any]) -> NSRect? {
    guard
      let x = arguments["x"] as? Double,
      let y = arguments["y"] as? Double,
      let width = arguments["width"] as? Double,
      let height = arguments["height"] as? Double
    else {
      return nil
    }

    return NSRect(x: x, y: y, width: width, height: height)
  }

  private func buildRectPayload(_ rect: NSRect) -> [String: Any] {
    return [
      "x": rect.origin.x,
      "y": rect.origin.y,
      "width": rect.size.width,
      "height": rect.size.height,
    ]
  }

  private func displayIntersectsSelection(_ displayBounds: NSRect, logicalSelection: NSRect?) -> Bool {
    guard let logicalSelection else {
      return true
    }

    return !displayBounds.intersection(logicalSelection).isEmpty
  }

  private func writeClipboardImageFile(arguments: [String: Any]) throws {
    guard let filePath = arguments["filePath"] as? String else {
      throw DisplayCaptureError(code: "INVALID_ARGS", message: "Invalid arguments for writeClipboardImageFile", details: nil)
    }

    // Flutter already exported the final annotated PNG to disk. Loading that file directly here
    // keeps the screenshot clipboard handoff inside the macOS runner and removes the extra Go-side
    // reopen/decode/write pass without changing the exported artifact contract.
    guard let image = NSImage(contentsOfFile: filePath) else {
      throw DisplayCaptureError(code: "clipboard_write_failed", message: "Failed to load screenshot file for clipboard export", details: ["filePath": filePath])
    }

    let pasteboard = NSPasteboard.general
    pasteboard.clearContents()
    if !pasteboard.writeObjects([image]) {
      throw DisplayCaptureError(code: "clipboard_write_failed", message: "Failed to write screenshot image to clipboard", details: ["filePath": filePath])
    }
  }

  private func payloadCaptureForSelection(_ capture: CachedDisplayCapture, logicalSelection: NSRect?) throws -> CachedDisplayCapture {
    guard let logicalSelection else {
      return capture
    }

    let logicalCrop = capture.logicalBounds.intersection(logicalSelection)
    if logicalCrop.isEmpty {
      throw DisplayCaptureError(code: "capture_failed", message: "Selection does not intersect macOS display image", details: ["displayId": capture.displayId])
    }

    let scaleX = CGFloat(capture.image.width) / capture.logicalBounds.width
    let scaleY = CGFloat(capture.image.height) / capture.logicalBounds.height
    let cropMinX = floor((logicalCrop.minX - capture.logicalBounds.minX) * scaleX)
    let cropMinY = floor((logicalCrop.minY - capture.logicalBounds.minY) * scaleY)
    let cropMaxX = ceil((logicalCrop.maxX - capture.logicalBounds.minX) * scaleX)
    let cropMaxY = ceil((logicalCrop.maxY - capture.logicalBounds.minY) * scaleY)
    let imageBounds = CGRect(x: 0, y: 0, width: CGFloat(capture.image.width), height: CGFloat(capture.image.height))
    let cropRect = CGRect(x: cropMinX, y: cropMinY, width: max(1, cropMaxX - cropMinX), height: max(1, cropMaxY - cropMinY)).intersection(imageBounds)
    if cropRect.isNull || cropRect.isEmpty {
      throw DisplayCaptureError(code: "capture_failed", message: "Selection crop is outside macOS display image", details: ["displayId": capture.displayId])
    }

    // Scrolling preview only needs the selected page rectangle. Cropping before PNG/base64 encoding
    // removes the previous per-frame cost of serializing entire 4K displays that Flutter discarded
    // immediately after checking display intersection.
    guard let croppedImage = capture.image.cropping(to: cropRect) else {
      throw DisplayCaptureError(code: "capture_failed", message: "Failed to crop macOS display image for scrolling capture", details: ["displayId": capture.displayId])
    }

    return CachedDisplayCapture(
      displayId: capture.displayId,
      logicalBounds: logicalCrop,
      visibleBounds: capture.visibleBounds.intersection(logicalCrop),
      scale: capture.scale,
      rotation: capture.rotation,
      image: croppedImage
    )
  }

  private func isStandardWindowButtonHidden(_ buttonType: NSWindow.ButtonType, on window: NSWindow) -> Bool {
    return window.standardWindowButton(buttonType)?.isHidden ?? false
  }

  private func applyStandardWindowButtonVisibility(
    on window: NSWindow,
    closeHidden: Bool,
    miniaturizeHidden: Bool,
    zoomHidden: Bool
  ) {
    // Screenshot presentation temporarily swaps the launcher into a borderless panel. Restoring the
    // style mask alone was not enough because AppKit can recreate the traffic-light buttons with
    // default visibility, so the launcher must explicitly reapply its hidden-button contract.
    window.standardWindowButton(.closeButton)?.isHidden = closeHidden
    window.standardWindowButton(.miniaturizeButton)?.isHidden = miniaturizeHidden
    window.standardWindowButton(.zoomButton)?.isHidden = zoomHidden
  }

  private func cancelNativeOverlayDismissTimeout() {
    nativeOverlayDismissTimeoutWorkItem?.cancel()
    nativeOverlayDismissTimeoutWorkItem = nil
  }

  private func dismissNativeSelectionOverlays() {
    cancelNativeOverlayDismissTimeout()
    guard let activeOverlaySession else {
      return
    }

    self.activeOverlaySession = nil
    activeOverlaySession.dismissOverlays()
  }

  private func beginScrollingCaptureOverlay(on window: NSWindow, arguments: [String: Any]) throws {
    let beginStart = Date()
    guard
      let workspacePayload = arguments["workspaceBounds"] as? [String: Any],
      let selectionPayload = arguments["selection"] as? [String: Any],
      let controlsPayload = arguments["controlsBounds"] as? [String: Any],
      let workspaceBounds = parseRect(arguments: workspacePayload),
      let selection = parseRect(arguments: selectionPayload),
      let controlsBounds = parseRect(arguments: controlsPayload)
    else {
      throw DisplayCaptureError(code: "INVALID_ARGS", message: "Invalid arguments for beginScrollingCaptureOverlay", details: nil)
    }

    dismissScrollingCaptureOverlay()
    let traceId = arguments["traceId"] as? String ?? ""
    activeScrollingCaptureTraceId = traceId
    logScrollingCaptureTiming("event=native_begin_overlay_start workspace=\(formatTimingRect(workspaceBounds)) selection=\(formatTimingRect(selection)) controls=\(formatTimingRect(controlsBounds))", traceId: traceId)
    // Scrolling capture differs from annotation mode: the selected rectangle must contain no Wox
    // window at all so native app interaction, text selection, and inertial scrolling continue
    // naturally. A single passive overlay window draws the outside dimming and border together so
    // the mask looks continuous while the reused Flutter window moves to the preview/toolbox area.
    let overlaySession = ScrollingCaptureOverlaySession(
      workspaceBounds: workspaceBounds,
      selection: selection,
      controlsWindow: window,
      controlsBounds: controlsBounds,
      onScroll: { [weak self] deltaY, rawDeltaY in
        DispatchQueue.main.async {
          self?.screenshotEventChannel?.invokeMethod("onScrollingCaptureWheel", arguments: ["deltaY": deltaY, "rawDeltaY": rawDeltaY])
        }
      },
      onTimingLog: { [weak self] message in
        self?.logScrollingCaptureTiming(message)
      }
    )
    activeScrollingCaptureOverlaySession = overlaySession
    overlaySession.begin()
    logScrollingCaptureTiming("event=native_begin_overlay_done elapsedMs=\(elapsedMilliseconds(since: beginStart))", traceId: traceId)
  }

  private func dismissScrollingCaptureOverlay() {
    activeScrollingCaptureTraceId = ""
    guard let activeScrollingCaptureOverlaySession else {
      return
    }

    self.activeScrollingCaptureOverlaySession = nil
    activeScrollingCaptureOverlaySession.dismiss()
  }

  private func scheduleNativeOverlayDismissTimeout() {
    cancelNativeOverlayDismissTimeout()
    guard activeOverlaySession != nil else {
      return
    }

    let workItem = DispatchWorkItem { [weak self] in
      // Flutter should acknowledge the handoff quickly by converting the selector into a passive
      // backdrop. This timeout only exists to prevent a leaked topmost drag overlay if that
      // handoff fails before Flutter starts driving the session.
      self?.dismissNativeSelectionOverlays()
    }
    nativeOverlayDismissTimeoutWorkItem = workItem
    DispatchQueue.main.asyncAfter(deadline: .now() + 10, execute: workItem)
  }

  private func selectCaptureRegion(
    workspaceBounds: NSRect,
    completion: @escaping (Result<[String: Any], DisplayCaptureError>) -> Void
  ) {
    guard activeOverlaySession == nil else {
      completion(
        .failure(
          DisplayCaptureError(
            code: "busy",
            message: "A screenshot selection session is already active",
            details: nil
          )
        )
      )
      return
    }

    let captures = cachedDisplayCaptures.filter { !$0.logicalBounds.intersection(workspaceBounds).isEmpty }
    guard captures.count >= 2 else {
      completion(.success(["wasHandled": false]))
      return
    }

    // The native selector uses one borderless window per display because the single Flutter window
    // path only renders reliably on the primary monitor. Matching Apple's per-screen overlay model
    // keeps the drag interaction stable across mixed-resolution and fullscreen spaces.
    let overlaySession = ScreenshotOverlaySession(
      workspaceBounds: workspaceBounds,
      captures: captures,
      onPreferredDisplayChanged: { [weak self] capture in
        self?.emitSelectionDisplayHint(capture)
      }
    ) { [weak self] result in
      guard let self else {
        completion(.success(["wasHandled": false]))
        return
      }

      if result.selection == nil {
        self.cancelNativeOverlayDismissTimeout()
        self.activeOverlaySession = nil
      } else {
        self.scheduleNativeOverlayDismissTimeout()
      }
      let payload: [String: Any] = [
        "wasHandled": true,
        "selection": result.selection.map { self.buildRectPayload($0) } ?? NSNull(),
        "editorVisibleBounds": result.editorVisibleBounds.map { self.buildRectPayload($0) } ?? NSNull(),
      ]
      completion(
        .success(payload)
      )
    }

    activeOverlaySession = overlaySession
    // Native selection activates Wox again after the launcher window hid itself. Saving the current
    // frontmost app here preserves the same focus-restore behavior that the regular window show/hide
    // path already has when the screenshot session later cancels or finishes from a hidden state.
    savePreviousActiveAppIfNeeded()
    overlaySession.begin()
  }

  private func prepareCaptureWorkspace(on window: NSWindow, bounds: NSRect) -> [String: Any] {
    let hadAcrylicEffect = containsAcrylicEffect(in: window)
    if screenshotPresentationState == nil {
      screenshotPresentationState = ScreenshotPresentationState(
        collectionBehavior: window.collectionBehavior,
        level: window.level,
        animationBehavior: window.animationBehavior,
        isOpaque: window.isOpaque,
        backgroundColor: window.backgroundColor,
        hasShadow: window.hasShadow,
        styleMask: window.styleMask,
        ignoresMouseEvents: window.ignoresMouseEvents,
        acceptsMouseMovedEvents: window.acceptsMouseMovedEvents,
        titleVisibility: window.titleVisibility,
        titlebarAppearsTransparent: window.titlebarAppearsTransparent,
        closeButtonHidden: isStandardWindowButtonHidden(.closeButton, on: window),
        miniaturizeButtonHidden: isStandardWindowButtonHidden(.miniaturizeButton, on: window),
        zoomButtonHidden: isStandardWindowButtonHidden(.zoomButton, on: window),
        panelHidesOnDeactivate: (window as? NSPanel)?.hidesOnDeactivate,
        panelBecomesKeyOnlyIfNeeded: (window as? NSPanel)?.becomesKeyOnlyIfNeeded,
        panelIsFloating: (window as? NSPanel)?.isFloatingPanel,
        hadAcrylicEffect: hadAcrylicEffect
      )
    }

    // Making the window borderless ensures the content rect equals the frame rect, so the
    // captured background fills the entire monitor area without a title bar offset pushing
    // the content down and exposing the macOS menubar underneath.
    window.styleMask = .borderless
    let flippedY = appKitY(fromTopLeftY: bounds.origin.y, height: bounds.height)

    let requestedScreenshotLevel = screenshotWindowLevel()
    // The launcher window's default collection behavior is tuned for a compact app panel. Screenshot
    // capture needs a system-overlay style contract instead so one window can stay pinned across the
    // full virtual desktop without inheriting the main launcher panel's single-display assumptions.
    window.collectionBehavior = screenshotCollectionBehavior()
    // The normal Wox launcher uses an acrylic NSVisualEffectView behind Flutter. In compact
    // scrolling-capture preview mode, unpainted Flutter pixels intentionally need to be transparent;
    // leaving the acrylic view in place made the preview look like it had a large gray backing panel.
    removeAcrylicEffect(from: window)
    makeScreenshotWindowTransparent(window)
    // The native selection overlay already appears without AppKit window animations. Leaving the
    // reused Flutter window at its default animation behavior makes the handoff look like a short
    // fade/zoom even when the content is ready. Disable window animation for screenshot mode so the
    // prepared Flutter frame replaces the native selector without any visible transition.
    window.animationBehavior = .none
    window.hasShadow = false
    // The launcher panel never had to prove that it owned the full-screen mouse path. Once the
    // screenshot flow turned that same panel into a borderless overlay, macOS could leave it in a
    // half-active panel state where clicks reactivated the app underneath and the annotation editor
    // appeared to disappear instantly. Force the screenshot shell to keep mouse ownership and stay
    // active on deactivation so the next trace can verify whether the blur loop was caused by stale
    // panel defaults rather than Flutter's annotation layer.
    window.ignoresMouseEvents = false
    window.acceptsMouseMovedEvents = true
    if let panel = window as? NSPanel {
      panel.hidesOnDeactivate = false
      panel.becomesKeyOnlyIfNeeded = false
      // Reusing the launcher NSPanel for screenshot annotation exposed a hidden AppKit constraint:
      // leaving the panel in floating-panel mode silently clamps the effective level back to
      // `.floating`, which puts the editor underneath other always-on-top windows. Disable that
      // panel behavior for screenshot mode so the requested shielding-level ordering can stick.
      panel.isFloatingPanel = false
    }
    window.setFrame(NSRect(x: bounds.origin.x, y: flippedY, width: bounds.width, height: bounds.height), display: true)
    window.contentView?.needsLayout = true
    window.contentView?.layoutSubtreeIfNeeded()
    window.displayIfNeeded()
    // Borderless/style mutations on NSPanel can still reset the effective level after the earlier
    // configuration changes. Reapply the screenshot level at the end of preparation so the visible
    // handoff window matches the native selector's ordering instead of falling back to `.floating`.
    window.level = requestedScreenshotLevel

    isCapturePresentationActive = true
    captureWorkspaceBounds = bounds
    captureWorkspaceScale = 1
    return [
      "workspaceBounds": buildRectPayload(bounds),
      "workspaceScale": 1,
      "presentedByPlatform": true,
    ]
  }

  private func revealPreparedCaptureWorkspace(on window: NSWindow) {
    guard screenshotPresentationState != nil else {
      return
    }

    // Preparing the borderless screenshot shell while the native overlay is still on-screen keeps
    // all geometry/layout work off the visible path. Reveal only happens once Flutter has already
    // produced the annotation frame that will immediately replace the native selector.
    savePreviousActiveAppIfNeeded()
    // The launcher window is reused across normal launcher, settings, and screenshot flows. Reapply
    // the screenshot level right before reveal so no earlier launcher path can leave the handoff
    // window stuck at a lower panel level once it becomes visible.
    if let panel = window as? NSPanel {
      panel.isFloatingPanel = false
    }
    window.collectionBehavior = screenshotCollectionBehavior()
    window.level = screenshotWindowLevel()
    window.makeKeyAndOrderFront(nil)
    NSApp.activate(ignoringOtherApps: true)
  }

  private func presentCaptureWorkspace(on window: NSWindow, bounds: NSRect) -> [String: Any] {
    let payload = prepareCaptureWorkspace(on: window, bounds: bounds)
    revealPreparedCaptureWorkspace(on: window)
    return payload
  }

  private func dismissCaptureWorkspacePresentation(on window: NSWindow) {
    dismissScrollingCaptureOverlay()
    cachedDisplayCaptures.removeAll()
    guard let savedState = screenshotPresentationState else {
      isCapturePresentationActive = false
      captureWorkspaceBounds = .zero
      captureWorkspaceScale = 1
      return
    }

    window.collectionBehavior = savedState.collectionBehavior
    window.level = savedState.level
    window.isOpaque = savedState.isOpaque
    window.backgroundColor = savedState.backgroundColor ?? NSColor.windowBackgroundColor
    window.hasShadow = savedState.hasShadow
    window.styleMask = savedState.styleMask
    window.ignoresMouseEvents = savedState.ignoresMouseEvents
    window.acceptsMouseMovedEvents = savedState.acceptsMouseMovedEvents
    // The screenshot selector only needs a temporary borderless shell. Once it exits, restore the
    // original title-bar appearance as well so Wox returns to the exact launcher chrome instead of
    // leaving the default macOS window controls visible in the top-left corner.
    window.titleVisibility = savedState.titleVisibility
    window.titlebarAppearsTransparent = savedState.titlebarAppearsTransparent
    if let panel = window as? NSPanel {
      if let hidesOnDeactivate = savedState.panelHidesOnDeactivate {
        panel.hidesOnDeactivate = hidesOnDeactivate
      }
      if let becomesKeyOnlyIfNeeded = savedState.panelBecomesKeyOnlyIfNeeded {
        panel.becomesKeyOnlyIfNeeded = becomesKeyOnlyIfNeeded
      }
      if let isFloatingPanel = savedState.panelIsFloating {
        panel.isFloatingPanel = isFloatingPanel
      }
    }
    // Screenshot mode temporarily disables AppKit's window-order animations to keep the native-to-
    // Flutter handoff seamless. Restore the original launcher behavior here so non-screenshot window
    // flows keep their previous animation semantics.
    window.animationBehavior = savedState.animationBehavior
    applyStandardWindowButtonVisibility(
      on: window,
      closeHidden: savedState.closeButtonHidden,
      miniaturizeHidden: savedState.miniaturizeButtonHidden,
      zoomHidden: savedState.zoomButtonHidden
    )
    if savedState.hadAcrylicEffect {
      applyAcrylicEffect(to: window)
    }
    screenshotPresentationState = nil
    isCapturePresentationActive = false
    captureWorkspaceBounds = .zero
    captureWorkspaceScale = 1
  }

  private func debugCaptureWorkspaceState(for window: NSWindow) -> [String: Any] {
    let contentRect = window.contentRect(forFrameRect: window.frame)
    let windowTopLeftRect = NSRect(
      x: window.frame.origin.x,
      y: topLeftY(fromAppKitY: window.frame.origin.y, height: contentRect.height),
      width: contentRect.width,
      height: contentRect.height
    )

    return [
      "isCapturePresentationActive": isCapturePresentationActive,
      "workspaceScale": captureWorkspaceScale,
      "workspaceBounds": buildRectPayload(captureWorkspaceBounds),
      "windowBounds": buildRectPayload(windowTopLeftRect),
      "collectionBehavior": window.collectionBehavior.rawValue,
      "levelRawValue": window.level.rawValue,
      "styleMask": Int(window.styleMask.rawValue),
      "ignoresMouseEvents": window.ignoresMouseEvents,
      "acceptsMouseMovedEvents": window.acceptsMouseMovedEvents,
      "isKeyWindow": window.isKeyWindow,
      "isMainWindow": window.isMainWindow,
      "titleVisibility": window.titleVisibility.rawValue,
      "titlebarAppearsTransparent": window.titlebarAppearsTransparent,
      "closeButtonHidden": isStandardWindowButtonHidden(.closeButton, on: window),
      "miniaturizeButtonHidden": isStandardWindowButtonHidden(.miniaturizeButton, on: window),
      "zoomButtonHidden": isStandardWindowButtonHidden(.zoomButton, on: window),
      "frontmostApplication": describeFrontmostApplication(),
      "panelHidesOnDeactivate": (window as? NSPanel)?.hidesOnDeactivate as Any,
      "panelBecomesKeyOnlyIfNeeded": (window as? NSPanel)?.becomesKeyOnlyIfNeeded as Any,
      "panelIsFloating": (window as? NSPanel)?.isFloatingPanel as Any,
    ]
  }

  private func currentMouseLocation() -> CGPoint {
    return CGEvent(source: nil)?.location ?? NSEvent.mouseLocation
  }

  private func buildDisplaySnapshotPayload(
    capture: CachedDisplayCapture,
    imagePayloadMode: DisplaySnapshotImagePayloadMode,
    logicalSelection: NSRect? = nil
  ) throws -> [String: Any] {
    let payloadStart = Date()
    let payloadCapture = try payloadCaptureForSelection(capture, logicalSelection: imagePayloadMode.includesImagePayload ? logicalSelection : nil)
    var payload: [String: Any] = [
      "displayId": payloadCapture.displayId,
      "logicalBounds": [
        "x": payloadCapture.logicalBounds.origin.x,
        "y": payloadCapture.logicalBounds.origin.y,
        "width": payloadCapture.logicalBounds.width,
        "height": payloadCapture.logicalBounds.height,
      ],
      "pixelBounds": [
        "x": payloadCapture.logicalBounds.origin.x * payloadCapture.scale,
        "y": payloadCapture.logicalBounds.origin.y * payloadCapture.scale,
        "width": CGFloat(payloadCapture.image.width),
        "height": CGFloat(payloadCapture.image.height),
      ],
      "scale": payloadCapture.scale,
      "rotation": payloadCapture.rotation,
    ]

    if imagePayloadMode.includesImagePayload {
      let encodeStart = Date()
      let bitmap = NSBitmapImageRep(cgImage: payloadCapture.image)
      guard let pngData = bitmap.representation(using: .png, properties: [:]) else {
        throw DisplayCaptureError(code: "capture_failed", message: "Failed to encode macOS display image", details: nil)
      }
      switch imagePayloadMode {
      case .base64:
        payload["imageBytesBase64"] = pngData.base64EncodedString()
      case .filePath:
        // Match the Windows deferred hydration path: keep MethodChannel payloads tiny and let Dart
        // load the PNG from disk only when it is preparing a visible annotation frame.
        let fileURL = FileManager.default.temporaryDirectory.appendingPathComponent("wox-\(UUID().uuidString).png")
        try pngData.write(to: fileURL, options: .atomic)
        payload["imageFilePath"] = fileURL.path
      case .none:
        break
      }
      logScrollingCaptureTiming("event=native_payload_encode_done displayId=\(payloadCapture.displayId) payloadMode=\(imagePayloadMode.logName) logical=\(formatTimingRect(payloadCapture.logicalBounds)) pixel=\(payloadCapture.image.width)x\(payloadCapture.image.height) cropped=\(logicalSelection != nil) elapsedMs=\(elapsedMilliseconds(since: encodeStart))")
    }

    logScrollingCaptureTiming("event=native_payload_done displayId=\(payloadCapture.displayId) payloadMode=\(imagePayloadMode.logName) logical=\(formatTimingRect(payloadCapture.logicalBounds)) pixel=\(payloadCapture.image.width)x\(payloadCapture.image.height) cropped=\(logicalSelection != nil) elapsedMs=\(elapsedMilliseconds(since: payloadStart))")
    return payload
  }

  private func buildDisplaySnapshotPayloads(
    captures: [CachedDisplayCapture],
    imagePayloadMode: DisplaySnapshotImagePayloadMode,
    logicalSelection: NSRect? = nil
  ) throws -> [[String: Any]] {
    let payloadsStart = Date()
    let payloads = try captures.map { capture in
      // The native overlay consumes cached CGImages directly, but Flutter only needs PNG payloads
      // once it is about to reveal an annotation frame or export pixels. Building payloads on
      // demand keeps the overlay path off the slow serialization step that made screenshot startup lag.
      try buildDisplaySnapshotPayload(capture: capture, imagePayloadMode: imagePayloadMode, logicalSelection: logicalSelection)
    }
    logScrollingCaptureTiming("event=native_payloads_done snapshotCount=\(captures.count) payloadMode=\(imagePayloadMode.logName) cropped=\(logicalSelection != nil) elapsedMs=\(elapsedMilliseconds(since: payloadsStart))")
    return payloads
  }

  private func captureAllDisplaysLegacy(logicalSelection: NSRect? = nil) throws -> [CachedDisplayCapture] {
    let legacyStart = Date()
    if !CGPreflightScreenCaptureAccess() {
      throw DisplayCaptureError(code: "permission_denied", message: "Screen recording permission is required", details: nil)
    }

    let screens = NSScreen.screens
    if screens.isEmpty {
      throw DisplayCaptureError(code: "capture_failed", message: "No screens are available for capture", details: nil)
    }

    var cachedCaptures: [CachedDisplayCapture] = []

    for screen in screens {
      let displayStart = Date()
      guard let screenNumber = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber else {
        throw DisplayCaptureError(code: "capture_failed", message: "Failed to resolve macOS display id", details: nil)
      }

      let displayId = CGDirectDisplayID(screenNumber.uint32Value)
      let frame = topLeftRect(fromAppKitRect: screen.frame)
      if !displayIntersectsSelection(frame, logicalSelection: logicalSelection) {
        logScrollingCaptureTiming("event=native_legacy_display_capture_skipped displayId=\(displayId) logical=\(formatTimingRect(frame)) selection=\(logicalSelection.map { formatTimingRect($0) } ?? "null")")
        continue
      }

      logScrollingCaptureTiming("event=native_legacy_display_capture_start displayId=\(displayId)")
      guard let cgImage = CGDisplayCreateImage(displayId) else {
        throw DisplayCaptureError(code: "capture_failed", message: "Failed to capture macOS display image", details: nil)
      }

      let visibleFrame = topLeftRect(fromAppKitRect: screen.visibleFrame)
      let scale = screen.backingScaleFactor
      let rotation = Int(CGDisplayRotation(displayId).rounded())

      cachedCaptures.append(
        CachedDisplayCapture(
          displayId: String(displayId),
          logicalBounds: frame,
          visibleBounds: visibleFrame,
          scale: scale,
          rotation: rotation,
          image: cgImage
        )
      )
      logScrollingCaptureTiming("event=native_legacy_display_capture_done displayId=\(displayId) logical=\(formatTimingRect(frame)) pixel=\(cgImage.width)x\(cgImage.height) elapsedMs=\(elapsedMilliseconds(since: displayStart))")
    }

    if cachedCaptures.isEmpty && logicalSelection != nil {
      throw DisplayCaptureError(code: "capture_failed", message: "Selection does not intersect any macOS display", details: nil)
    }

    logScrollingCaptureTiming("event=native_legacy_capture_done snapshotCount=\(cachedCaptures.count) elapsedMs=\(elapsedMilliseconds(since: legacyStart))")
    return cachedCaptures
  }

  @available(macOS 14.0, *)
  private func captureAllDisplaysWithScreenCaptureKit(logicalSelection: NSRect? = nil) async throws -> [CachedDisplayCapture] {
    let screenCaptureKitStart = Date()
    if !CGPreflightScreenCaptureAccess() {
      throw DisplayCaptureError(code: "permission_denied", message: "Screen recording permission is required", details: nil)
    }

    let shareableContent = try await SCShareableContent.excludingDesktopWindows(
      false,
      onScreenWindowsOnly: true
    )
    if shareableContent.displays.isEmpty {
      throw DisplayCaptureError(
        code: "capture_failed",
        message: "No displays are available for ScreenCaptureKit capture",
        details: nil
      )
    }

    let excludedApplications = shareableContent.applications.filter {
      $0.bundleIdentifier == Bundle.main.bundleIdentifier
    }
    var cachedCaptures: [CachedDisplayCapture] = []

    for display in shareableContent.displays {
      let displayStart = Date()
      let matchedScreen = screen(for: display.displayID)
      // Mixed-resolution desktops exposed that ScreenCaptureKit's display frame can drift from the
      // AppKit window server layout that actually decides where NSWindow overlays appear. Basing the
      // overlay geometry on the matching NSScreen frame keeps the shaded window aligned with the
      // real monitor bounds even when one display uses a different native resolution or scale.
      let logicalFrame = matchedScreen.map { topLeftRect(fromAppKitRect: $0.frame) } ?? topLeftRect(fromAppKitRect: display.frame)
      if !displayIntersectsSelection(logicalFrame, logicalSelection: logicalSelection) {
        logScrollingCaptureTiming("event=native_sck_display_capture_skipped displayId=\(display.displayID) logical=\(formatTimingRect(logicalFrame)) selection=\(logicalSelection.map { formatTimingRect($0) } ?? "null")")
        continue
      }

      // ScreenCaptureKit replaces CGDisplayCreateImage on modern macOS. We exclude the current
      // Wox process here so the screenshot workspace does not appear in the captured background
      // after the launcher window is hidden and resized across the virtual desktop.
      let contentFilter = SCContentFilter(
        display: display,
        excludingApplications: excludedApplications,
        exceptingWindows: []
      )
      let scale = CGFloat(contentFilter.pointPixelScale)
      let visibleFrame = matchedScreen.map { topLeftRect(fromAppKitRect: $0.visibleFrame) } ?? logicalFrame
      let captureLogicalBounds = logicalSelection.map { logicalFrame.intersection($0) } ?? logicalFrame
      let captureOutputSize = logicalSelection == nil ? display.frame.size : captureLogicalBounds.size
      let captureVisibleBounds = logicalSelection == nil ? visibleFrame : visibleFrame.intersection(captureLogicalBounds)
      let filterContentRect = contentFilter.contentRect
      let sourceRect = logicalSelection.map { _ in
        NSRect(
          x: filterContentRect.minX + captureLogicalBounds.minX - logicalFrame.minX,
          y: filterContentRect.minY + captureLogicalBounds.minY - logicalFrame.minY,
          width: captureLogicalBounds.width,
          height: captureLogicalBounds.height
        )
      }
      let streamConfiguration = SCStreamConfiguration()
      streamConfiguration.width = max(1, Int((captureOutputSize.width * scale).rounded()))
      streamConfiguration.height = max(1, Int((captureOutputSize.height * scale).rounded()))
      if let sourceRect {
        // Scrolling capture previously asked ScreenCaptureKit for the whole display and cropped the
        // selected page after capture. That kept correctness but made every preview refresh pay for
        // a full 4K display frame; using sourceRect lets the native capture return only the same
        // logical crop that Flutter will stitch/export, preserving preview/export consistency while
        // removing the avoidable full-display capture cost.
        streamConfiguration.sourceRect = sourceRect
      }
      // Long screenshots stitch multiple captures together. If the system cursor is included in
      // every tile, the final image repeats the pointer at each sampled scroll position and also
      // corrupts overlap matching, so keep captured display pixels cursor-free.
      streamConfiguration.showsCursor = false

      logScrollingCaptureTiming(
        "event=native_sck_display_capture_start displayId=\(display.displayID) logical=\(formatTimingRect(captureLogicalBounds)) source=\(sourceRect.map { formatTimingRect($0) } ?? "full") pixel=\(streamConfiguration.width)x\(streamConfiguration.height)"
      )
      let cgImage = try await SCScreenshotManager.captureImage(
        contentFilter: contentFilter,
        configuration: streamConfiguration
      )
      let rotation = Int(CGDisplayRotation(display.displayID).rounded())

      cachedCaptures.append(
        CachedDisplayCapture(
          displayId: String(display.displayID),
          logicalBounds: captureLogicalBounds,
          visibleBounds: captureVisibleBounds,
          scale: scale,
          rotation: rotation,
          image: cgImage
        )
      )
      logScrollingCaptureTiming(
        "event=native_sck_display_capture_done displayId=\(display.displayID) logical=\(formatTimingRect(captureLogicalBounds)) pixel=\(cgImage.width)x\(cgImage.height) elapsedMs=\(elapsedMilliseconds(since: displayStart))"
      )
    }

    if cachedCaptures.isEmpty && logicalSelection != nil {
      throw DisplayCaptureError(code: "capture_failed", message: "Selection does not intersect any ScreenCaptureKit display", details: nil)
    }

    logScrollingCaptureTiming("event=native_sck_capture_done snapshotCount=\(cachedCaptures.count) elapsedMs=\(elapsedMilliseconds(since: screenCaptureKitStart))")
    return cachedCaptures
  }

  private func captureDisplayCaptures(logicalSelection: NSRect? = nil) async throws -> [CachedDisplayCapture] {
    let captureStart = Date()
    if #available(macOS 14.0, *) {
      do {
        let captures = try await captureAllDisplaysWithScreenCaptureKit(logicalSelection: logicalSelection)
        logScrollingCaptureTiming("event=native_capture_display_captures_done branch=screen_capture_kit snapshotCount=\(captures.count) elapsedMs=\(elapsedMilliseconds(since: captureStart))")
        return captures
      } catch let error as DisplayCaptureError {
        if error.code == "permission_denied" {
          throw error
        }

        // Keep the existing CGDisplay fallback for older runners or partial ScreenCaptureKit
        // failures so screenshot capture still works on supported macOS builds that haven't
        // fully transitioned to the newer API surface yet.
        log("ScreenCaptureKit capture failed, falling back to CGDisplayCreateImage: \(error.message)")
      } catch {
        // Surface unexpected native failures in the fallback log as well so we still capture
        // evidence when the newer API fails before the legacy path is attempted.
        log("ScreenCaptureKit capture failed, falling back to CGDisplayCreateImage: \(error.localizedDescription)")
      }
    }

    let captures = try captureAllDisplaysLegacy(logicalSelection: logicalSelection)
    logScrollingCaptureTiming("event=native_capture_display_captures_done branch=legacy snapshotCount=\(captures.count) elapsedMs=\(elapsedMilliseconds(since: captureStart))")
    return captures
  }

  private func captureDisplayMetadata() async throws -> [[String: Any]] {
    let captures = try await captureDisplayCaptures()
    cachedDisplayCaptures = captures
    return try buildDisplaySnapshotPayloads(captures: captures, imagePayloadMode: .none)
  }

  private func loadDisplaySnapshots(displayIds: [String]) async throws -> [[String: Any]] {
    if displayIds.isEmpty {
      return []
    }

    let requestedDisplayIds = Set(displayIds)
    let cacheAlreadyMatches = !cachedDisplayCaptures.isEmpty && requestedDisplayIds.allSatisfy { displayId in
      cachedDisplayCaptures.contains(where: { $0.displayId == displayId })
    }

    // Native multi-display selection already captured one CGImage per screen for the overlay. Reuse
    // that cache here so Flutter can hydrate only the displays it truly needs instead of forcing
    // the overlay startup path to PNG-encode every monitor up front.
    if !cacheAlreadyMatches {
      cachedDisplayCaptures = try await captureDisplayCaptures()
    }

    let filteredCaptures = cachedDisplayCaptures.filter { requestedDisplayIds.contains($0.displayId) }
    if filteredCaptures.count != requestedDisplayIds.count {
      throw DisplayCaptureError(
        code: "capture_failed",
        message: "Failed to locate cached macOS display image",
        details: ["requestedDisplayIds": Array(requestedDisplayIds), "availableDisplayIds": cachedDisplayCaptures.map(\.displayId)]
      )
    }

    return try buildDisplaySnapshotPayloads(captures: filteredCaptures, imagePayloadMode: .filePath)
  }

  private func captureAllDisplays(traceId: String? = nil, logicalSelection: NSRect? = nil) async throws -> [[String: Any]] {
    // The first scrolling frame is captured before the native overlay exists. Scope its trace to
    // this capture call so normal screenshots do not inherit stale scrolling timing metadata.
    let shouldClearTraceAfterCall = activeScrollingCaptureOverlaySession == nil
    if let traceId, !traceId.isEmpty {
      activeScrollingCaptureTraceId = traceId
    } else if shouldClearTraceAfterCall {
      activeScrollingCaptureTraceId = ""
    }
    defer {
      if shouldClearTraceAfterCall {
        activeScrollingCaptureTraceId = ""
      }
    }
    let captureAllStart = Date()
    let captures = try await captureDisplayCaptures(logicalSelection: logicalSelection)
    if logicalSelection == nil {
      cachedDisplayCaptures = captures
    }
    let payloads = try buildDisplaySnapshotPayloads(captures: captures, imagePayloadMode: .base64, logicalSelection: logicalSelection)
    logScrollingCaptureTiming("event=native_capture_all_displays_done snapshotCount=\(captures.count) selection=\(logicalSelection.map { formatTimingRect($0) } ?? "null") elapsedMs=\(elapsedMilliseconds(since: captureAllStart))")
    return payloads
  }

  private func postKeyboardEvent(key: String, isDown: Bool) -> FlutterError? {
    guard let keyCode = keyCode(for: key) else {
      return FlutterError(code: "UNSUPPORTED_KEY", message: "Unsupported key for macOS system input", details: key)
    }

    guard let event = CGEvent(keyboardEventSource: CGEventSource(stateID: .hidSystemState), virtualKey: keyCode, keyDown: isDown) else {
      return FlutterError(code: "INPUT_ERROR", message: "Failed to create macOS keyboard event", details: key)
    }

    event.post(tap: .cghidEventTap)
    return nil
  }

  private func postMouseButtonEvent(button: String, isDown: Bool) -> FlutterError? {
    guard let mouseButton = mouseButton(for: button) else {
      return FlutterError(code: "UNSUPPORTED_BUTTON", message: "Unsupported mouse button for macOS system input", details: button)
    }

    let eventTypes = mouseEventTypes(for: mouseButton)
    let eventType = isDown ? eventTypes.down : eventTypes.up
    guard
      let event = CGEvent(mouseEventSource: CGEventSource(stateID: .hidSystemState), mouseType: eventType, mouseCursorPosition: currentMouseLocation(), mouseButton: mouseButton)
    else {
      return FlutterError(code: "INPUT_ERROR", message: "Failed to create macOS mouse event", details: button)
    }

    event.post(tap: .cghidEventTap)
    return nil
  }

  private func postMouseScrollEvent(deltaY: Double, through window: NSWindow) -> FlutterError? {
    let scrollStart = Date()
    // Scrolling capture drives the app underneath Wox after the screenshot shell hides. Using a
    // CGEvent keeps the scroll targeted at the current cursor location instead of requiring app-
    // specific accessibility APIs, which would make the feature depend on each target window type.
    guard
      let event = CGEvent(
        scrollWheelEvent2Source: CGEventSource(stateID: .hidSystemState),
        units: .line,
        wheelCount: 1,
        wheel1: Int32(-deltaY.rounded()),
        wheel2: 0,
        wheel3: 0
      )
    else {
      return FlutterError(code: "INPUT_ERROR", message: "Failed to create macOS scroll event", details: deltaY)
    }

    let wasVisible = window.isVisible
    logScrollingCaptureTiming("event=native_post_scroll_start deltaY=\(deltaY) wasVisible=\(wasVisible)")
    // `ignoresMouseEvents` is not reliable for synthetic scroll-wheel routing on macOS: the Wox
    // screenshot window can still be the effective event target even when the cursor visually sits
    // inside the selected page. Briefly ordering the capture window out before posting the wheel
    // event gives the system a real underlying target, then restores the overlay quickly enough for
    // the Flutter preview refresh to repaint the new page position.
    if wasVisible {
      window.orderOut(nil)
      logScrollingCaptureTiming("event=native_post_scroll_window_ordered_out elapsedMs=\(elapsedMilliseconds(since: scrollStart))")
    }
    DispatchQueue.main.asyncAfter(deadline: .now() + 0.01) {
      event.post(tap: .cghidEventTap)
      self.logScrollingCaptureTiming("event=native_post_scroll_event_posted elapsedMs=\(elapsedMilliseconds(since: scrollStart))")
    }
    DispatchQueue.main.asyncAfter(deadline: .now() + 0.08) {
      if wasVisible {
        let restoreLevel = self.activeScrollingCaptureOverlaySession == nil ? screenshotWindowLevel() : scrollingCaptureControlsWindowLevel()
        applyScreenshotWindowPresentation(to: window, level: restoreLevel)
        if self.activeScrollingCaptureOverlaySession != nil {
          makeScreenshotWindowTransparent(window)
        }
        window.orderFrontRegardless()
        window.makeKeyAndOrderFront(nil)
        // Match the initial reveal path: after synthetic scrolling, AppKit may restore panel
        // defaults while the window is reinserted, so keep the final level explicit.
        window.level = restoreLevel
        NSApp.activate(ignoringOtherApps: true)
        self.logScrollingCaptureTiming("event=native_post_scroll_window_restored elapsedMs=\(elapsedMilliseconds(since: scrollStart))")
      }
    }
    return nil
  }

  private func moveMouse(to point: CGPoint) -> FlutterError? {
    CGWarpMouseCursorPosition(screenPoint(fromTopLeft: point))
    return nil
  }

  override func applicationShouldTerminateAfterLastWindowClosed(_: NSApplication) -> Bool {
    return false
  }

  override func applicationSupportsSecureRestorableState(_: NSApplication) -> Bool {
    return true
  }

  /// Apply acrylic effect to window
  private func applyAcrylicEffect(to window: NSWindow) {
    // Set appearance based on current theme
    if currentAppearance == "dark" {
      window.appearance = NSAppearance(named: .darkAqua)
    } else {
      window.appearance = NSAppearance(named: .aqua)
    }

    if let contentView = window.contentView {
      // Remove existing visual effect view if any to avoid stacking
      for subview in contentView.subviews {
        if subview is NSVisualEffectView {
          subview.removeFromSuperview()
        }
      }

      let effectView = NSVisualEffectView(frame: contentView.bounds)
      effectView.material = .popover
      effectView.state = .active
      effectView.blendingMode = .behindWindow
      // Ensure the effect view resizes with the window
      effectView.autoresizingMask = [.width, .height]
      contentView.addSubview(effectView, positioned: .below, relativeTo: nil)

      // Try to make all Flutter-related views transparent
      for subview in contentView.subviews where !(subview is NSVisualEffectView) {
        subview.wantsLayer = true
        subview.layer?.backgroundColor = NSColor.clear.cgColor
      }
    }
  }

  private func containsAcrylicEffect(in window: NSWindow) -> Bool {
    return window.contentView?.subviews.contains(where: { $0 is NSVisualEffectView }) ?? false
  }

  private func removeAcrylicEffect(from window: NSWindow) {
    guard let contentView = window.contentView else {
      return
    }

    // Screenshot mode has its own Flutter-rendered backdrop or native dimming overlay. Removing the
    // launcher acrylic layer lets intentionally transparent preview pixels show the desktop instead
    // of the launcher blur material.
    for subview in contentView.subviews where subview is NSVisualEffectView {
      subview.removeFromSuperview()
    }
  }

  private func targetWindow(from arguments: Any?, fallback: NSWindow) -> NSWindow {
    guard
      let args = arguments as? [String: Any],
      let rawWindowHandle = args["windowHandle"],
      let windowHandle = nativeWindowHandle(from: rawWindowHandle),
      let pointer = UnsafeRawPointer(bitPattern: windowHandle)
    else {
      return fallback
    }

    return Unmanaged<NSWindow>.fromOpaque(pointer).takeUnretainedValue()
  }

  private func nativeWindowHandle(from value: Any) -> UInt? {
    if let value = value as? UInt {
      return value
    }
    if let value = value as? UInt64 {
      return UInt(truncatingIfNeeded: value)
    }
    if let value = value as? Int {
      return value > 0 ? UInt(value) : nil
    }
    if let value = value as? NSNumber {
      return UInt(truncatingIfNeeded: value.uint64Value)
    }
    return nil
  }

  private func applyManagedWindowStyle(to window: NSWindow, mica: Bool, roundedCorners: Bool, minimizable: Bool, resizable: Bool) {
    // RegularWindowController creates a titled AppKit window. Wox draws the
    // visible titlebar in Flutter, so managed windows must use the same
    // borderless native contract as the Windows secondary-window path.
    window.styleMask.insert(.fullSizeContentView)
    window.styleMask.remove(.titled)
    window.styleMask.remove(.closable)
    // Keep native capabilities in sync with the Flutter-drawn traffic lights;
    // hidden AppKit buttons still need these masks for minimize/zoom commands.
    if minimizable {
      window.styleMask.insert(.miniaturizable)
    } else {
      window.styleMask.remove(.miniaturizable)
    }
    if resizable {
      window.styleMask.insert(.resizable)
    } else {
      window.styleMask.remove(.resizable)
    }
    window.titleVisibility = .hidden
    window.titlebarAppearsTransparent = true
    window.standardWindowButton(.closeButton)?.isHidden = true
    window.standardWindowButton(.miniaturizeButton)?.isHidden = true
    window.standardWindowButton(.zoomButton)?.isHidden = true
    window.isOpaque = false
    window.backgroundColor = .clear
    window.hasShadow = roundedCorners

    if let contentView = window.contentView {
      clearBackingLayers(in: contentView)
    }
    if mica {
      applyAcrylicEffect(to: window)
    } else {
      removeAcrylicEffect(from: window)
    }
    applyManagedWindowCornerMask(to: window, roundedCorners: roundedCorners)
  }

  private func applyManagedWindowCornerMask(to window: NSWindow, roundedCorners: Bool) {
    guard let contentView = window.contentView else {
      return
    }

    // Borderless transparent windows do not receive AppKit's normal rounded
    // frame clipping, so the Flutter surface must be clipped explicitly.
    for view in [contentView, contentView.superview].compactMap({ $0 }) {
      view.wantsLayer = true
      view.layer?.cornerRadius = roundedCorners ? 12 : 0
      view.layer?.cornerCurve = .continuous
      view.layer?.masksToBounds = roundedCorners
    }
  }

  // Setup notification for window blur event
  private func setupWindowBlurNotification() {
    guard let window = mainFlutterWindow else { return }

    NotificationCenter.default.addObserver(
      self,
      selector: #selector(windowDidResignKey),
      name: NSWindow.didResignKeyNotification,
      object: window
    )
  }

  // Handle window loss of focus
  @objc private func windowDidResignKey(_: Notification) {
    log("Window did resign key (blur)")
    if mainFlutterWindow?.isVisible == true {
      shouldRestorePreviousAppOnHide = false
    }
    // Notify Flutter about the window blur event
    DispatchQueue.main.async {
      self.windowEventChannel?.invokeMethod("onWindowBlur", arguments: nil)
    }
  }

  override func applicationDidFinishLaunching(_ notification: Notification) {
    let controller = mainFlutterWindow?.contentViewController as! FlutterViewController

    // Try to make Flutter view background transparent
    let flutterView = controller.view
    flutterView.wantsLayer = true
    flutterView.layer?.backgroundColor = NSColor.clear.cgColor

    let channel = FlutterMethodChannel(
      name: "com.wox.macos_window_manager",
      binaryMessenger: controller.engine.binaryMessenger
    )
    let screenshotChannel = FlutterMethodChannel(
      name: "com.wox.macos_screenshot_events",
      binaryMessenger: controller.engine.binaryMessenger
    )

    // Store window event channel for use in window events
    windowEventChannel = channel
    screenshotEventChannel = screenshotChannel
    resultDragBridge = ResultDragBridge(binaryMessenger: controller.engine.binaryMessenger, sourceView: flutterView)

    // Setup window blur notification
    setupWindowBlurNotification()

    channel.setMethodCallHandler { [weak self] call, result in
      guard let window = self?.mainFlutterWindow else {
        result(FlutterError(code: "NO_WINDOW", message: "No window found", details: nil))
        return
      }

      DispatchQueue.main.async {
        switch call.method {
        case "captureAllDisplays":
          Task { @MainActor in
            do {
              let arguments = call.arguments as? [String: Any]
              let selectionPayload = arguments?["logicalSelection"] as? [String: Any]
              let logicalSelection: NSRect?
              if let selectionPayload {
                logicalSelection = self?.parseRect(arguments: selectionPayload)
              } else {
                logicalSelection = nil
              }
              result(try await self?.captureAllDisplays(traceId: arguments?["traceId"] as? String, logicalSelection: logicalSelection))
            } catch let error as DisplayCaptureError {
              // Convert the Swift-native capture error back to `FlutterError` only when returning
              // through the method channel so the Dart side keeps the existing error contract.
              result(error.asFlutterError())
            } catch {
              result(
                FlutterError(
                  code: "capture_failed",
                  message: error.localizedDescription,
                  details: nil
                ))
            }
          }
          return

        case "captureDisplayMetadata":
          Task { @MainActor in
            do {
              result(try await self?.captureDisplayMetadata())
            } catch let error as DisplayCaptureError {
              result(error.asFlutterError())
            } catch {
              result(
                FlutterError(
                  code: "capture_failed",
                  message: error.localizedDescription,
                  details: nil
                ))
            }
          }
          return

        case "loadDisplaySnapshots":
          Task { @MainActor in
            do {
              guard
                let args = call.arguments as? [String: Any],
                let displayIds = args["displayIds"] as? [String]
              else {
                throw DisplayCaptureError(code: "INVALID_ARGS", message: "Invalid arguments for loadDisplaySnapshots", details: nil)
              }

              result(try await self?.loadDisplaySnapshots(displayIds: displayIds))
            } catch let error as DisplayCaptureError {
              result(error.asFlutterError())
            } catch {
              result(
                FlutterError(
                  code: "capture_failed",
                  message: error.localizedDescription,
                  details: nil
                ))
            }
          }
          return

        case "selectCaptureRegion":
          if let args = call.arguments as? [String: Any], let bounds = self?.parseRect(arguments: args) {
            self?.selectCaptureRegion(workspaceBounds: bounds) { selectionResult in
              switch selectionResult {
              case .success(let payload):
                result(payload)
              case .failure(let error):
                result(error.asFlutterError())
              }
            }
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS",
                message: "Invalid arguments for selectCaptureRegion",
                details: nil
              )
            )
          }

        case "presentCaptureWorkspace":
          if let args = call.arguments as? [String: Any], let bounds = self?.parseRect(arguments: args) {
            let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
            result(self?.presentCaptureWorkspace(on: targetWindow, bounds: bounds))
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS",
                message: "Invalid arguments for presentCaptureWorkspace",
                details: nil
              )
            )
          }

        case "prepareCaptureWorkspace":
          if let args = call.arguments as? [String: Any], let bounds = self?.parseRect(arguments: args) {
            let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
            result(self?.prepareCaptureWorkspace(on: targetWindow, bounds: bounds))
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS",
                message: "Invalid arguments for prepareCaptureWorkspace",
                details: nil
              )
            )
          }

        case "revealPreparedCaptureWorkspace":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          self?.revealPreparedCaptureWorkspace(on: targetWindow)
          result(nil)

        case "dismissCaptureWorkspacePresentation":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          self?.dismissCaptureWorkspacePresentation(on: targetWindow)
          result(nil)

        case "beginScrollingCaptureOverlay":
          if let args = call.arguments as? [String: Any] {
            do {
              let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
              try self?.beginScrollingCaptureOverlay(on: targetWindow, arguments: args)
              result(nil)
            } catch let error as DisplayCaptureError {
              result(error.asFlutterError())
            } catch {
              result(FlutterError(code: "scrolling_overlay_failed", message: error.localizedDescription, details: nil))
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Invalid arguments for beginScrollingCaptureOverlay", details: nil))
          }

        case "dismissNativeSelectionOverlays":
          self?.dismissNativeSelectionOverlays()
          result(nil)

        case "writeClipboardImageFile":
          if let args = call.arguments as? [String: Any] {
            do {
              try self?.writeClipboardImageFile(arguments: args)
              result(nil)
            } catch let error as DisplayCaptureError {
              result(error.asFlutterError())
            } catch {
              result(
                FlutterError(
                  code: "clipboard_write_failed",
                  message: error.localizedDescription,
                  details: nil
                ))
            }
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS",
                message: "Invalid arguments for writeClipboardImageFile",
                details: nil
              )
            )
          }

        case "debugCaptureWorkspaceState":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          result(self?.debugCaptureWorkspaceState(for: targetWindow))

        case "activateScreenshotDiagonalResizeCursor":
          if let args = call.arguments as? [String: Any],
            let kind = args["kind"] as? String,
            kind == "resizeUpLeftDownRight" || kind == "resizeUpRightDownLeft"
          {
            // Bug fix: the Flutter macOS engine falls back to the arrow cursor for diagonal resize
            // names, so screenshot corner handles looked inactive even though dragging worked. Use
            // a Wox-owned native cursor only for those two missing diagonal directions.
            screenshotDiagonalResizeCursor(kind: kind).set()
            result(nil)
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Invalid screenshot diagonal resize cursor", details: nil))
          }

        case "applyManagedWindowStyle":
          if let args = call.arguments as? [String: Any] {
            let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
            let mica = args["mica"] as? Bool ?? true
            let roundedCorners = args["roundedCorners"] as? Bool ?? true
            let minimizable = args["minimizable"] as? Bool ?? true
            let resizable = args["resizable"] as? Bool ?? false
            self?.applyManagedWindowStyle(to: targetWindow, mica: mica, roundedCorners: roundedCorners, minimizable: minimizable, resizable: resizable)
            result(nil)
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Invalid arguments for applyManagedWindowStyle", details: nil))
          }
        case "getDesktopWallpaperPath":
          result(self?.desktopWallpaperPath())

        case "setSize":
          if let args = call.arguments as? [String: Any],
            let width = args["width"] as? Double,
            let height = args["height"] as? Double
          {
            let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
            // Keep top-left stable when resizing; direct setContentSize can shift Y on macOS.
            let currentFrame = targetWindow.frame
            let currentTop = currentFrame.origin.y + currentFrame.height
            let targetFrame = targetWindow.frameRect(forContentRect: NSRect(x: 0, y: 0, width: width, height: height))
            let newOriginY = currentTop - targetFrame.height
            targetWindow.setFrame(
              NSRect(
                x: currentFrame.origin.x,
                y: newOriginY,
                width: targetFrame.width,
                height: targetFrame.height
              ),
              display: true
            )
            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setSize", details: nil
              ))
          }

        case "inputKeyDown":
          if let args = call.arguments as? [String: Any],
            let key = args["key"] as? String
          {
            if let error = self?.postKeyboardEvent(key: key, isDown: true) {
              result(error)
            } else {
              result(nil)
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Missing key for keyboard input", details: nil))
          }

        case "inputKeyUp":
          if let args = call.arguments as? [String: Any],
            let key = args["key"] as? String
          {
            if let error = self?.postKeyboardEvent(key: key, isDown: false) {
              result(error)
            } else {
              result(nil)
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Missing key for keyboard input", details: nil))
          }

        case "inputMouseMove":
          if let args = call.arguments as? [String: Any],
            let x = args["x"] as? Double,
            let y = args["y"] as? Double
          {
            if let error = self?.moveMouse(to: CGPoint(x: x, y: y)) {
              result(error)
            } else {
              result(nil)
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Missing coordinates for mouse move", details: nil))
          }

        case "inputMouseDown":
          if let args = call.arguments as? [String: Any],
            let button = args["button"] as? String
          {
            if let error = self?.postMouseButtonEvent(button: button, isDown: true) {
              result(error)
            } else {
              result(nil)
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Missing mouse button", details: nil))
          }

        case "inputMouseUp":
          if let args = call.arguments as? [String: Any],
            let button = args["button"] as? String
          {
            if let error = self?.postMouseButtonEvent(button: button, isDown: false) {
              result(error)
            } else {
              result(nil)
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Missing mouse button", details: nil))
          }

        case "inputMouseScroll":
          if let args = call.arguments as? [String: Any],
            let deltaY = args["deltaY"] as? Double
          {
            if let error = self?.postMouseScrollEvent(deltaY: deltaY, through: window) {
              result(error)
            } else {
              result(nil)
            }
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Missing scroll delta", details: nil))
          }

        case "setBounds":
          if let args = call.arguments as? [String: Any],
            let x = args["x"] as? Double,
            let y = args["y"] as? Double,
            let width = args["width"] as? Double,
            let height = args["height"] as? Double
          {
            let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
            let frameRect = targetWindow.frameRect(forContentRect: NSRect(x: 0, y: 0, width: width, height: height))
            // Screenshot capture stretches one window across the virtual desktop, so screen-local
            // Y conversion is not enough once monitors sit at different heights.
            let flippedY = appKitY(fromTopLeftY: y, height: frameRect.height)
            targetWindow.setFrame(NSRect(x: x, y: flippedY, width: frameRect.width, height: frameRect.height), display: true)
            if self?.isCapturePresentationActive == true {
              // Scrolling preview resizes the same transparent Flutter window as more stitched rows
              // arrive. Re-clearing backing layers after the native resize prevents AppKit from
              // reintroducing an opaque frame-sized surface around the compact controls.
              makeScreenshotWindowTransparent(targetWindow)
            }
            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setBounds", details: nil
              ))
          }

        case "getPosition":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          let frame = targetWindow.frame
          // Return the shared virtual-desktop top-left position so saved window locations round-trip
          // correctly even after the window temporarily spans multiple displays for screenshot capture.
          let x = frame.origin.x
          let y = topLeftY(fromAppKitY: frame.origin.y, height: frame.height)
          result(["x": x, "y": y])

        case "getSize":
          // Keep getSize consistent with setSize/setBounds, which both accept
          // content rect dimensions from Dart. Returning frame size here makes
          // the controller think the window is taller than the actual Flutter
          // content area on macOS, which can skip needed resizes and cause
          // transient RenderFlex overflows in smoke tests.
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          let contentRect = targetWindow.contentRect(forFrameRect: targetWindow.frame)
          result(["width": contentRect.width, "height": contentRect.height])

        case "getWindowHandle":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          result(UInt(bitPattern: Unmanaged.passUnretained(targetWindow).toOpaque()))

        case "setPosition":
          if let args = call.arguments as? [String: Any],
            let x = args["x"] as? Double,
            let y = args["y"] as? Double
          {
            let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
            // Keep launcher positioning on the same virtual-desktop contract as screenshot capture.
            // The previous screen-relative conversion restored windows to the wrong Y whenever the
            // saved position belonged to a display that was vertically offset from the main screen.
            let flippedY = appKitY(fromTopLeftY: y, height: targetWindow.frame.height)

            targetWindow.setFrameOrigin(NSPoint(x: x, y: flippedY))
            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setPosition", details: nil
              ))
          }

        case "center":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          // Get the screen where the mouse cursor is located
          let mouseLocation = NSEvent.mouseLocation
          var targetScreen: NSScreen? = nil
          for screen in NSScreen.screens {
            if screen.frame.contains(mouseLocation) {
              targetScreen = screen
              break
            }
          }
          let screenFrame = targetScreen?.visibleFrame ?? NSScreen.main?.visibleFrame ?? NSRect.zero

          var windowWidth: CGFloat = targetWindow.frame.width
          var windowHeight: CGFloat = targetWindow.frame.height
          if let args = call.arguments as? [String: Any] {
            if let width = args["width"] as? Double {
              windowWidth = CGFloat(width)
            }
            if let height = args["height"] as? Double {
              windowHeight = CGFloat(height)
            }
          }

          let x = (screenFrame.width - windowWidth) / 2 + screenFrame.minX
          let y = (screenFrame.height - windowHeight) / 2 + screenFrame.minY

          self?.log("Center: window to \(x),\(y) on screen at \(screenFrame.minX),\(screenFrame.minY)")
          let newFrame = NSRect(x: x, y: y, width: windowWidth, height: windowHeight)
          targetWindow.setFrame(newFrame, display: true)
          result(nil)

        case "show":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          self?.log("Showing Wox window")
          self?.savePreviousActiveAppIfNeeded()

          targetWindow.makeKeyAndOrderFront(nil)
          NSApp.activate(ignoringOtherApps: true)
          result(nil)

        case "hide":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          self?.log("Hiding Wox window")
          let isWoxFrontmost = NSApp.isActive || NSWorkspace.shared.frontmostApplication == NSRunningApplication.current
          let shouldRestorePreviousApp = self?.shouldRestorePreviousAppOnHide == true
          targetWindow.orderOut(nil)
          // Only restore the previous app when Wox stayed focused since the last show/focus.
          if isWoxFrontmost && shouldRestorePreviousApp {
            if let prevApp = self?.previousActiveApp, prevApp != NSRunningApplication.current, !prevApp.isTerminated {
              self?.log("Activating previous app: \(prevApp.localizedName ?? "Unknown") (bundleID: \(prevApp.bundleIdentifier ?? "Unknown"))")
              prevApp.activate(options: .activateIgnoringOtherApps)
            } else {
              self?.log("No valid previous app saved for activation")
            }
          } else if !shouldRestorePreviousApp {
            self?.log("Skipping previous app activation because Wox already lost focus before hiding")
          } else {
            self?.log("Wox is not frontmost when hiding, skipping previous app activation")
          }
          self?.previousActiveApp = nil
          self?.shouldRestorePreviousAppOnHide = false
          result(nil)

        case "focus":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          if self?.isCapturePresentationActive == true {
            // Blur recovery is part of the screenshot interaction loop, so a generic focus call must
            // not revive the launcher's lower panel ordering. Reapply screenshot-specific window
            // traits here to keep recovery aligned with the prepared capture workspace contract.
            if let panel = targetWindow as? NSPanel {
              panel.isFloatingPanel = false
            }
            targetWindow.collectionBehavior = screenshotCollectionBehavior()
            targetWindow.level = screenshotWindowLevel()
          }
          self?.log("Focusing Wox window")
          self?.savePreviousActiveAppIfNeeded()
          targetWindow.makeKeyAndOrderFront(nil)
          NSApp.activate(ignoringOtherApps: true)
          result(nil)

        case "isVisible":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          result(targetWindow.isVisible)

        case "setAlwaysOnTop":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          let alwaysOnTop: Bool?
          if let value = call.arguments as? Bool {
            alwaysOnTop = value
          } else if let args = call.arguments as? [String: Any] {
            alwaysOnTop = args["value"] as? Bool
          } else {
            alwaysOnTop = nil
          }

          if let alwaysOnTop {
            if self?.isCapturePresentationActive == true {
              // Screenshot presentation owns the window level while capture is active. Letting the
              // generic always-on-top toggle run here would collapse the shielding-level editor back
              // into launcher panel levels and reintroduce the "annotation UI behind other windows"
              // regression we are debugging.
              result(nil)
              return
            }
            if alwaysOnTop {
              targetWindow.level = .popUpMenu
            } else {
              targetWindow.level = .normal
            }

            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setAlwaysOnTop", details: nil
              )
            )
          }

        case "setAppearance":
          if let appearance = call.arguments as? String {
            self?.currentAppearance = appearance
            self?.applyAcrylicEffect(to: window)
            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setAppearance", details: nil
              )
            )
          }

        case "startDragging":
          let targetWindow = self?.targetWindow(from: call.arguments, fallback: window) ?? window
          if let currentEvent = targetWindow.currentEvent {
            self?.log("Performing drag with event: \(currentEvent)")
            targetWindow.performDrag(with: currentEvent)
          }
          result(nil)

        case "waitUntilReadyToShow":
          // Set app appearance based on current theme
          if self?.currentAppearance == "dark" {
            NSApp.appearance = NSAppearance(named: .darkAqua)
          } else {
            NSApp.appearance = NSAppearance(named: .aqua)
          }

          window.level = .popUpMenu
          window.titlebarAppearsTransparent = true
          window.styleMask.insert(.fullSizeContentView)
          window.styleMask.insert(.nonactivatingPanel)
          window.styleMask.remove(.resizable)

          // Hide windows buttons
          window.titleVisibility = .hidden
          window.standardWindowButton(.closeButton)?.isHidden = true
          window.standardWindowButton(.miniaturizeButton)?.isHidden = true
          window.standardWindowButton(.zoomButton)?.isHidden = true

          // Make window can join all spaces
          window.collectionBehavior.insert(.canJoinAllSpaces)
          window.collectionBehavior.insert(.fullScreenAuxiliary)
          window.styleMask.insert(.nonactivatingPanel)
          self?.applyAcrylicEffect(to: window)

          if let mainWindow = window as? MainFlutterWindow {
            mainWindow.isReadyToShow = true
          }

          result(nil)

        default:
          result(FlutterMethodNotImplemented)
        }
      }
    }

    super.applicationDidFinishLaunching(notification)
  }
}
