class WoxCloudSyncStatus {
  final bool enabled;
  final String deviceId;
  final WoxCloudSyncKeyStatus keyStatus;
  final WoxCloudSyncState? state;

  WoxCloudSyncStatus({required this.enabled, required this.deviceId, required this.keyStatus, required this.state});

  factory WoxCloudSyncStatus.empty() {
    return WoxCloudSyncStatus(enabled: false, deviceId: '', keyStatus: WoxCloudSyncKeyStatus.empty(), state: null);
  }

  factory WoxCloudSyncStatus.fromJson(Map<String, dynamic> json) {
    final keyStatusJson = json['key_status'];
    final stateJson = json['state'];
    return WoxCloudSyncStatus(
      enabled: json['enabled'] ?? false,
      deviceId: json['device_id'] ?? '',
      keyStatus: keyStatusJson is Map<String, dynamic> ? WoxCloudSyncKeyStatus.fromJson(keyStatusJson) : WoxCloudSyncKeyStatus.empty(),
      state: stateJson is Map<String, dynamic> ? WoxCloudSyncState.fromJson(stateJson) : null,
    );
  }
}

class WoxCloudSyncBootstrapStatus {
  final bool hasRemoteData;
  final bool hasRemoteKey;

  WoxCloudSyncBootstrapStatus({required this.hasRemoteData, required this.hasRemoteKey});

  factory WoxCloudSyncBootstrapStatus.fromJson(Map<String, dynamic> json) {
    return WoxCloudSyncBootstrapStatus(hasRemoteData: json['has_remote_data'] ?? false, hasRemoteKey: json['has_remote_key'] ?? false);
  }
}

class WoxAccountStatus {
  final bool loggedIn;
  final String userId;
  final String email;
  final bool emailVerified;
  final String subscriptionStatus;
  final int subscriptionCurrentPeriodEnd;
  final bool syncEligible;
  final bool syncEnabled;
  final bool sessionExpired;

  WoxAccountStatus({
    required this.loggedIn,
    required this.userId,
    required this.email,
    required this.emailVerified,
    required this.subscriptionStatus,
    required this.subscriptionCurrentPeriodEnd,
    required this.syncEligible,
    required this.syncEnabled,
    required this.sessionExpired,
  });

  factory WoxAccountStatus.empty() {
    return WoxAccountStatus(
      loggedIn: false,
      userId: '',
      email: '',
      emailVerified: false,
      subscriptionStatus: 'none',
      subscriptionCurrentPeriodEnd: 0,
      syncEligible: false,
      syncEnabled: false,
      sessionExpired: false,
    );
  }

  factory WoxAccountStatus.fromJson(Map<String, dynamic> json) {
    return WoxAccountStatus(
      loggedIn: json['logged_in'] ?? false,
      userId: json['user_id'] ?? '',
      email: json['email'] ?? '',
      emailVerified: json['email_verified'] ?? false,
      subscriptionStatus: json['subscription_status'] ?? 'none',
      subscriptionCurrentPeriodEnd: json['subscription_current_period_end'] ?? 0,
      syncEligible: json['sync_eligible'] ?? false,
      syncEnabled: json['sync_enabled'] ?? false,
      sessionExpired: json['session_expired'] ?? false,
    );
  }
}

class WoxBillingSession {
  final String url;

  WoxBillingSession({required this.url});

  factory WoxBillingSession.fromJson(Map<String, dynamic> json) {
    return WoxBillingSession(url: json['url'] ?? '');
  }
}

class WoxAccountActionResult {
  final String code;
  final String message;
  final String email;
  final int expiresAt;

  WoxAccountActionResult({required this.code, required this.message, required this.email, required this.expiresAt});

  bool get isOk => code == 'ok';

  bool get needsEmailVerification => code == 'need_verify_email';

  factory WoxAccountActionResult.empty() {
    return WoxAccountActionResult(code: '', message: '', email: '', expiresAt: 0);
  }

  factory WoxAccountActionResult.fromJson(Map<String, dynamic> json) {
    return WoxAccountActionResult(code: json['code'] ?? '', message: json['message'] ?? '', email: json['email'] ?? '', expiresAt: json['expires_at'] ?? 0);
  }
}

class WoxCloudSyncKeyStatus {
  final bool available;
  final int version;

  WoxCloudSyncKeyStatus({required this.available, required this.version});

  factory WoxCloudSyncKeyStatus.empty() {
    return WoxCloudSyncKeyStatus(available: false, version: 0);
  }

  factory WoxCloudSyncKeyStatus.fromJson(Map<String, dynamic> json) {
    return WoxCloudSyncKeyStatus(available: json['available'] ?? false, version: json['version'] ?? 0);
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
