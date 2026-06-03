import 'dart:convert';

import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

enum _WoxTimeTrackerFieldType { raw, quoted }

class _WoxTimeTrackerField {
  _WoxTimeTrackerField(this.name, this.value, this.type);

  final String name;
  final String value;
  final _WoxTimeTrackerFieldType type;
}

class WoxTimeTracker {
  static final WoxTimeTracker _disabled = WoxTimeTracker._disabledInstance();

  WoxTimeTracker._disabledInstance() : enabled = false, traceId = "", stage = "", _fields = const [], _fieldIndexes = const {};

  WoxTimeTracker._(this.traceId, this.stage) : enabled = true, _fields = <_WoxTimeTrackerField>[], _fieldIndexes = <String, int>{};

  final bool enabled;
  final String traceId;
  final String stage;
  final List<_WoxTimeTrackerField> _fields;
  final Map<String, int> _fieldIndexes;

  /// Creates a dev-only timing tracker. Non-dev builds get a no-op instance.
  static WoxTimeTracker start(String traceId, String stage) {
    if (!Env.isDev) {
      return _disabled;
    }
    return WoxTimeTracker._(traceId, stage);
  }

  int checkpointUs() {
    if (!enabled) {
      return 0;
    }
    return DateTime.now().microsecondsSinceEpoch;
  }

  int checkpointMs() {
    if (!enabled) {
      return 0;
    }
    return DateTime.now().millisecondsSinceEpoch;
  }

  void setRawString(String name, String value) {
    if (!enabled) {
      return;
    }
    _setField(name, value, _WoxTimeTrackerFieldType.raw);
  }

  void setString(String name, String value) {
    if (!enabled) {
      return;
    }
    _setField(name, value, _WoxTimeTrackerFieldType.quoted);
  }

  void setInt(String name, int value) {
    if (!enabled) {
      return;
    }
    _setField(name, value.toString(), _WoxTimeTrackerFieldType.raw);
  }

  void setDouble(String name, double value) {
    if (!enabled) {
      return;
    }
    _setField(name, value.toStringAsFixed(1), _WoxTimeTrackerFieldType.raw);
  }

  void setBool(String name, bool value) {
    if (!enabled) {
      return;
    }
    _setField(name, value.toString(), _WoxTimeTrackerFieldType.raw);
  }

  void setElapsedUs(String name, int startUs) {
    if (!enabled || startUs <= 0) {
      return;
    }
    setInt(name, DateTime.now().microsecondsSinceEpoch - startUs);
  }

  void setElapsedMs(String name, int startMs) {
    if (!enabled || startMs <= 0) {
      return;
    }
    setInt(name, DateTime.now().millisecondsSinceEpoch - startMs);
  }

  /// Writes one structured timing line after callers finish adding fields.
  void log() {
    if (!enabled) {
      return;
    }

    final builder = StringBuffer("query_timing source=ui stage=$stage traceId=$traceId");
    for (final field in _fields) {
      final encodedValue = field.type == _WoxTimeTrackerFieldType.quoted ? jsonEncode(field.value) : field.value;
      builder.write(" ${field.name}=$encodedValue");
    }
    Logger.instance.debug(traceId, builder.toString());
  }

  void _setField(String name, String value, _WoxTimeTrackerFieldType type) {
    final existingIndex = _fieldIndexes[name];
    final field = _WoxTimeTrackerField(name, value, type);
    if (existingIndex == null) {
      _fieldIndexes[name] = _fields.length;
      _fields.add(field);
      return;
    }
    _fields[existingIndex] = field;
  }
}
