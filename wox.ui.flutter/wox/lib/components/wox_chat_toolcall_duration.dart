import 'dart:async';
import 'package:flutter/material.dart';
import 'package:get/get.dart';

/// A component that automatically updates duration
/// Updates every 100ms when endTimestamp is null
class WoxChatToolcallDuration extends StatefulWidget {
  /// Unique identifier
  final String id;

  /// Start timestamp (milliseconds)
  final int startTimestamp;

  /// End timestamp (milliseconds), uses current time if null
  final int? endTimestamp;

  /// Whether to show millisecond unit
  final bool showUnit;

  /// Text style
  final TextStyle? style;

  /// Custom builder callback
  final Widget Function(BuildContext context, int duration)? builder;

  const WoxChatToolcallDuration({
    super.key,
    required this.id,
    required this.startTimestamp,
    this.endTimestamp,
    this.showUnit = true,
    this.style,
    this.builder,
  });

  @override
  State<WoxChatToolcallDuration> createState() => _WoxChatToolcallDurationState();
}

class _WoxChatToolcallDurationState extends State<WoxChatToolcallDuration> {
  // Using RxInt to auto-update UI when value changes
  final RxInt _duration = 0.obs;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _updateDuration();

    // Start timer if no end time is provided
    if (widget.endTimestamp == null) {
      _startTimer();
    }
  }

  @override
  void didUpdateWidget(WoxChatToolcallDuration oldWidget) {
    super.didUpdateWidget(oldWidget);

    // Update duration if ID, start time or end time changes
    if (oldWidget.id != widget.id || oldWidget.startTimestamp != widget.startTimestamp || oldWidget.endTimestamp != widget.endTimestamp) {
      _updateDuration();

      // Start or stop timer based on end time
      if (widget.endTimestamp == null) {
        _startTimer();
      } else {
        _stopTimer();
      }
    }
  }

  void _updateDuration() {
    final endTime = widget.endTimestamp ?? DateTime.now().millisecondsSinceEpoch;
    _duration.value = endTime - widget.startTimestamp;
  }

  void _startTimer() {
    // Cancel existing timer
    _stopTimer();

    // Create new timer that updates every 100ms
    _timer = Timer.periodic(const Duration(milliseconds: 100), (timer) {
      _updateDuration();
    });
  }

  void _stopTimer() {
    _timer?.cancel();
    _timer = null;
  }

  @override
  void dispose() {
    _stopTimer();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      if (widget.builder != null) {
        return widget.builder!(context, _duration.value);
      }

      return Text(
        widget.showUnit ? '${_duration.value}ms' : '${_duration.value}',
        style: widget.style,
      );
    });
  }
}
