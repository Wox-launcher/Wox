class WoxCloudSyncStatus {
  final bool enabled;
  final String deviceId;
  final WoxCloudSyncKeyStatus keyStatus;
  final WoxCloudSyncState? state;

  WoxCloudSyncStatus({
    required this.enabled,
    required this.deviceId,
    required this.keyStatus,
    required this.state,
  });

  factory WoxCloudSyncStatus.empty() {
    return WoxCloudSyncStatus(
      enabled: false,
      deviceId: '',
      keyStatus: WoxCloudSyncKeyStatus.empty(),
      state: null,
    );
  }

  factory WoxCloudSyncStatus.fromJson(Map<String, dynamic> json) {
    final keyStatusJson = json['key_status'];
    final stateJson = json['state'];
    return WoxCloudSyncStatus(
      enabled: json['enabled'] ?? false,
      deviceId: json['device_id'] ?? '',
      keyStatus: keyStatusJson is Map<String, dynamic>
          ? WoxCloudSyncKeyStatus.fromJson(keyStatusJson)
          : WoxCloudSyncKeyStatus.empty(),
      state: stateJson is Map<String, dynamic> ? WoxCloudSyncState.fromJson(stateJson) : null,
    );
  }
}

class WoxCloudSyncKeyStatus {
  final bool available;
  final int version;

  WoxCloudSyncKeyStatus({
    required this.available,
    required this.version,
  });

  factory WoxCloudSyncKeyStatus.empty() {
    return WoxCloudSyncKeyStatus(available: false, version: 0);
  }

  factory WoxCloudSyncKeyStatus.fromJson(Map<String, dynamic> json) {
    return WoxCloudSyncKeyStatus(
      available: json['available'] ?? false,
      version: json['version'] ?? 0,
    );
  }
}

class WoxCloudSyncState {
  final String cursor;
  final int lastPullTs;
  final int lastPushTs;
  final int backoffUntil;
  final int retryCount;
  final String lastError;
  final bool bootstrapped;

  WoxCloudSyncState({
    required this.cursor,
    required this.lastPullTs,
    required this.lastPushTs,
    required this.backoffUntil,
    required this.retryCount,
    required this.lastError,
    required this.bootstrapped,
  });

  factory WoxCloudSyncState.fromJson(Map<String, dynamic> json) {
    return WoxCloudSyncState(
      cursor: json['cursor'] ?? '',
      lastPullTs: json['last_pull_ts'] ?? 0,
      lastPushTs: json['last_push_ts'] ?? 0,
      backoffUntil: json['backoff_until'] ?? 0,
      retryCount: json['retry_count'] ?? 0,
      lastError: json['last_error'] ?? '',
      bootstrapped: json['bootstrapped'] ?? false,
    );
  }
}
