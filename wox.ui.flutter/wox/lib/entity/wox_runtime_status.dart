class WoxRuntimeStatus {
  WoxRuntimeStatus({
    required this.runtime,
    required this.isStarted,
    required this.hostVersion,
    required this.statusCode,
    required this.statusMessage,
    required this.executablePath,
    required this.lastStartError,
    required this.canRestart,
    required this.installUrl,
    required this.loadedPluginCount,
    required this.loadedPluginNames,
  });

  final String runtime;
  final bool isStarted;
  final String hostVersion;
  final String statusCode;
  final String statusMessage;
  final String executablePath;
  final String lastStartError;
  final bool canRestart;
  final String installUrl;
  final int loadedPluginCount;
  final List<String> loadedPluginNames;

  bool get isActionableFailure => statusCode == 'executable_missing' || statusCode == 'unsupported_version' || statusCode == 'start_failed';

  factory WoxRuntimeStatus.fromJson(dynamic json) {
    if (json == null) {
      return WoxRuntimeStatus(
        runtime: '',
        isStarted: false,
        hostVersion: '',
        statusCode: 'stopped',
        statusMessage: '',
        executablePath: '',
        lastStartError: '',
        canRestart: false,
        installUrl: '',
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
      parsedCount = int.tryParse(rawCount == null ? '0' : rawCount.toString()) ?? 0;
    }

    return WoxRuntimeStatus(
      runtime: json['Runtime']?.toString() ?? '',
      isStarted: json['IsStarted'] == true,
      hostVersion: json['HostVersion']?.toString() ?? '',
      statusCode: json['StatusCode']?.toString() ?? (json['IsStarted'] == true ? 'running' : 'stopped'),
      statusMessage: json['StatusMessage']?.toString() ?? '',
      executablePath: json['ExecutablePath']?.toString() ?? '',
      lastStartError: json['LastStartError']?.toString() ?? '',
      canRestart: json['CanRestart'] == true,
      installUrl: json['InstallUrl']?.toString() ?? '',
      loadedPluginCount: parsedCount,
      loadedPluginNames: List<String>.from((json['LoadedPluginNames'] ?? const <dynamic>[]).map((dynamic item) => item.toString())),
    );
  }
}
