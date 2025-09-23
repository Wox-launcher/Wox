class FlickerStatus {
  final bool flicker;
  final String reason; // direction_change | not_enough_events | below_threshold
  final int events;
  const FlickerStatus(this.flicker, this.reason, this.events);
}

class AdjustDelayResult {
  final int newDelay;
  final FlickerStatus status;
  const AdjustDelayResult(this.newDelay, this.status);
}

class WindowFlickerDetector {
  int _lastAppliedHeight = 0;
  final List<_ResizeRecord> _resizeRecords = [];

  int flickerWindowMs;

  int flickerMinEvents;
  int flickerMinDirectionChanges; // minimum direction reversals within the window to consider oscillation
  int stableDecreaseRequired; // consecutive non-flicker confirmations required before decreasing delay

  int _stableNonFlickerCount = 0; // internal counter for consecutive stable windows

  int minDelay;
  int maxDelay;
  int step;

  WindowFlickerDetector({
    this.flickerWindowMs = 400,
    this.flickerMinEvents = 2,
    this.flickerMinDirectionChanges = 1,
    this.stableDecreaseRequired = 5,
    this.minDelay = 100,
    this.maxDelay = 200,
    this.step = 10,
  });

  void recordResize(int height) {
    final now = DateTime.now().millisecondsSinceEpoch;
    if (_lastAppliedHeight == 0) {
      _lastAppliedHeight = height;
    }
    final delta = height - _lastAppliedHeight;
    _resizeRecords.add(_ResizeRecord(now, height, delta));
    _lastAppliedHeight = height;
    _compact(now);
  }

  /// Determine whether the window is visually "flickering" within the last
  /// [flickerWindowMs] milliseconds.
  ///
  /// Definition (high-level):
  /// - Flickering is a short-term oscillation or jitter of the window height during
  ///   rapid typing. Typical causes are: clearing results (shrink) followed quickly
  ///   by new results (expand), or many micro-resizes that add up to a noticeable move.
  ///
  /// We apply a single heuristic:
  /// 1) Direction-change oscillation (reason = "direction_change"):
  ///    - For each resize, we compute the sign of the height delta:
  ///      +1 = expand, -1 = shrink, 0 = unchanged.
  ///    - We count direction reversals between consecutive non-zero signs.
  ///    - If the number of reversals within the time window is >= [flickerMinDirectionChanges]
  ///      we consider this true flicker.
  ///      Rationale: a single reversal is often benign (a one-off correction), whereas
  ///      multiple reversals in a short time indicate oscillation that users perceive as
  ///      flicker.
  ///
  ///
  /// Other reasons:
  /// - "not_enough_events": Fewer than [flickerMinEvents] resizes in the window; insufficient
  ///   signal to classify flicker.
  /// - "below_threshold": Enough events exist, but neither heuristic exceeded its threshold.
  FlickerStatus isWindowFlickering() {
    final now = DateTime.now().millisecondsSinceEpoch;
    final windowStart = now - flickerWindowMs;
    final recent = _resizeRecords.where((r) => r.ts >= windowStart).toList();

    if (recent.length < flickerMinEvents) {
      return FlickerStatus(false, "not_enough_events", recent.length);
    }

    int directionReversals = 0;
    int? lastNonZeroSign;
    for (final r in recent) {
      final sign = r.delta == 0 ? 0 : (r.delta > 0 ? 1 : -1);
      if (sign != 0) {
        if (lastNonZeroSign != null && sign != lastNonZeroSign) {
          directionReversals++;
        }
        lastNonZeroSign = sign;
      }
    }

    if (directionReversals >= flickerMinDirectionChanges) {
      return FlickerStatus(true, "direction_change", recent.length);
    }

    return FlickerStatus(false, "below_threshold", recent.length);
  }

  /// Adjust the clear delay conservatively based on flicker status.
  ///
  /// Stability preference policy:
  /// - If flicker is detected (reason = "direction_change" | "total_delta_exceeds"),
  ///   increase delay by +step to damp oscillation, and reset stability counter.
  /// - If non-flicker with reason = "not_enough_events" or "below_threshold",
  ///   do not change the delay immediately.
  /// - Only after [stableDecreaseRequired] consecutive "below_threshold" windows
  ///   do we decrease by -step (then reset the counter). "not_enough_events" is not
  ///   counted as stability evidence to avoid premature decreases.
  AdjustDelayResult adjustClearDelay(int currentDelay) {
    final status = isWindowFlickering();
    int next = currentDelay;

    if (status.flicker) {
      // Clear evidence of oscillation: raise delay and reset stability
      _stableNonFlickerCount = 0;
      next = currentDelay + step;
    } else {
      if (status.reason == "below_threshold" || status.reason == "not_enough_events") {
        // Evidence of stability accumulates only on solid non-flicker windows
        _stableNonFlickerCount++;
        if (_stableNonFlickerCount >= stableDecreaseRequired) {
          next = currentDelay - step;
          _stableNonFlickerCount = 0; // reset after a decrease
        } else {
          next = currentDelay; // hold delay until we have enough consecutive stable windows
        }
      } else {
        // Unknown non-flicker reason (should not happen): keep as-is
        next = currentDelay;
      }
    }

    if (next < minDelay) next = minDelay;
    if (next > maxDelay) next = maxDelay;
    return AdjustDelayResult(next, status);
  }

  void _compact(int nowMs) {
    final cutoff = nowMs - (flickerWindowMs * 3);
    while (_resizeRecords.isNotEmpty && _resizeRecords.first.ts < cutoff) {
      _resizeRecords.removeAt(0);
    }
  }
}

class _ResizeRecord {
  final int ts; // ms
  final int height;
  final int delta;
  const _ResizeRecord(this.ts, this.height, this.delta);
}
