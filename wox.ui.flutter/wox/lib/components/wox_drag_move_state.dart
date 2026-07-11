/// Tracks short-lived window drag state so Linux blur handling can separate a
/// compositor-driven drag focus change from a normal outside-window blur.
class WoxDragMoveState {
  static const Duration _activeWindow = Duration(seconds: 5);
  static DateTime? _activeUntil;
  static String? _activeTraceId;
  static String? _activeSource;

  /// Marks a window drag as active long enough to correlate focus loss events.
  static void begin(String traceId, String source) {
    _activeTraceId = traceId;
    _activeSource = source;
    _activeUntil = DateTime.now().add(_activeWindow);
  }

  /// Clears the active window drag marker after normal pan completion or cancel.
  static void end() {
    _activeUntil = null;
    _activeTraceId = null;
    _activeSource = null;
  }

  static bool get isActive {
    final activeUntil = _activeUntil;
    return activeUntil != null && DateTime.now().isBefore(activeUntil);
  }

  static String? get activeTraceId => _activeTraceId;

  static String get debugSummary {
    final activeUntil = _activeUntil;
    final expiresInMs = activeUntil == null ? 0 : activeUntil.difference(DateTime.now()).inMilliseconds.clamp(0, _activeWindow.inMilliseconds);
    return "active=$isActive source=${_activeSource ?? "unknown"} traceId=${_activeTraceId ?? "none"} expiresInMs=$expiresInMs";
  }
}
