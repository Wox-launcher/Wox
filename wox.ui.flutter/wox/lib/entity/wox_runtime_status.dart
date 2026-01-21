class WoxRuntimeStatus {
  WoxRuntimeStatus({
    required this.runtime,
    required this.isStarted,
    required this.hostVersion,
    required this.loadedPluginCount,
    required this.loadedPluginNames,
  });

  final String runtime;
  final bool isStarted;
  final String hostVersion;
  final int loadedPluginCount;
  final List<String> loadedPluginNames;

  factory WoxRuntimeStatus.fromJson(dynamic json) {
    if (json == null) {
      return WoxRuntimeStatus(
        runtime: '',
        isStarted: false,
        hostVersion: '',
        loadedPluginCount: 0,
        loadedPluginNames: const <String>[],
      );
    }

    final dynamic rawCount = json['LoadedPluginCount'];
    final int parsedCount;
    if (rawCount is int) {
      parsedCount = rawCount;
    } else if (rawCount is double) {
      parsedCount = rawCount.toInt();
    } else {
      parsedCount =
          int.tryParse(rawCount == null ? '0' : rawCount.toString()) ?? 0;
    }

    return WoxRuntimeStatus(
      runtime: json['Runtime']?.toString() ?? '',
      isStarted: json['IsStarted'] == true,
      hostVersion: json['HostVersion']?.toString() ?? '',
      loadedPluginCount: parsedCount,
      loadedPluginNames: List<String>.from(
          (json['LoadedPluginNames'] ?? const <dynamic>[])
              .map((dynamic item) => item.toString())),
    );
  }
}
