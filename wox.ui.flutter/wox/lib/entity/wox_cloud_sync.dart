class WoxCloudSyncStatus {
  final bool enabled;
  final String deviceId;
  final WoxCloudSyncKeyStatus keyStatus;
  final WoxCloudSyncState? state;
  final WoxCloudSyncProgress? progress;

  WoxCloudSyncStatus({required this.enabled, required this.deviceId, required this.keyStatus, required this.state, required this.progress});

  factory WoxCloudSyncStatus.empty() {
    return WoxCloudSyncStatus(enabled: false, deviceId: '', keyStatus: WoxCloudSyncKeyStatus.empty(), state: null, progress: null);
  }

  factory WoxCloudSyncStatus.fromJson(Map<String, dynamic> json) {
    final keyStatusJson = json['key_status'];
    final stateJson = json['state'];
    final progressJson = json['progress'];
    return WoxCloudSyncStatus(
      enabled: json['enabled'] ?? false,
      deviceId: json['device_id'] ?? '',
      keyStatus: keyStatusJson is Map<String, dynamic> ? WoxCloudSyncKeyStatus.fromJson(keyStatusJson) : WoxCloudSyncKeyStatus.empty(),
      state: stateJson is Map<String, dynamic> ? WoxCloudSyncState.fromJson(stateJson) : null,
      progress: progressJson is Map<String, dynamic> ? WoxCloudSyncProgress.fromJson(progressJson) : null,
    );
  }

  WoxCloudSyncStatus withProgress(WoxCloudSyncProgress? progress) {
    return WoxCloudSyncStatus(enabled: enabled, deviceId: deviceId, keyStatus: keyStatus, state: state, progress: progress);
  }
}

class WoxCloudSyncProgress {
  final bool active;
  final String operation;
  final String entityType;
  final String pluginId;
  final String key;
  final int current;
  final int total;

  WoxCloudSyncProgress({
    required this.active,
    required this.operation,
    required this.entityType,
    required this.pluginId,
    required this.key,
    required this.current,
    required this.total,
  });

  factory WoxCloudSyncProgress.fromJson(Map<String, dynamic> json) {
    return WoxCloudSyncProgress(
      active: json['active'] ?? false,
      operation: json['operation'] ?? '',
      entityType: json['entity_type'] ?? '',
      pluginId: json['plugin_id'] ?? '',
      key: json['key'] ?? '',
      current: json['current'] ?? 0,
      total: json['total'] ?? 0,
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
  final String plan;
  final WoxSyncLimits syncLimits;
  final int deviceCount;
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
    required this.plan,
    required this.syncLimits,
    required this.deviceCount,
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
      plan: 'free',
      syncLimits: WoxSyncLimits.free(),
      deviceCount: 0,
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
      plan: json['plan'] ?? 'free',
      syncLimits: json['sync_limits'] is Map<String, dynamic> ? WoxSyncLimits.fromJson(json['sync_limits']) : WoxSyncLimits.free(),
      deviceCount: json['device_count'] ?? 0,
      syncEnabled: json['sync_enabled'] ?? false,
      sessionExpired: json['session_expired'] ?? false,
    );
  }

  bool get isPro => plan == 'pro';
}

class WoxSyncLimits {
  final int? deviceLimit;
  final int? syncIntervalSeconds;
  final int? syncWindowSeconds;

  WoxSyncLimits({required this.deviceLimit, required this.syncIntervalSeconds, required this.syncWindowSeconds});

  factory WoxSyncLimits.free() {
    return WoxSyncLimits(deviceLimit: 2, syncIntervalSeconds: 3600, syncWindowSeconds: 600);
  }

  factory WoxSyncLimits.fromJson(Map<String, dynamic> json) {
    return WoxSyncLimits(deviceLimit: json['device_limit'], syncIntervalSeconds: json['sync_interval_seconds'], syncWindowSeconds: json['sync_window_seconds']);
  }
}

class WoxBillingPlan {
  final WoxBillingPlanTier free;
  final WoxBillingPlanTier pro;

  WoxBillingPlan({required this.free, required this.pro});

  factory WoxBillingPlan.empty() {
    return WoxBillingPlan(
      free: WoxBillingPlanTier(price: WoxBillingPlanPrice(currency: 'usd', unitAmount: 0, interval: 'month', formatted: r'$0/month'), limits: WoxSyncLimits.free()),
      pro: WoxBillingPlanTier(price: WoxBillingPlanPrice.empty(), limits: WoxSyncLimits(deviceLimit: null, syncIntervalSeconds: null, syncWindowSeconds: null)),
    );
  }

  factory WoxBillingPlan.fromJson(Map<String, dynamic> json) {
    final freeJson = json['free'];
    final proJson = json['pro'];
    return WoxBillingPlan(
      free: freeJson is Map<String, dynamic> ? WoxBillingPlanTier.fromJson(freeJson) : WoxBillingPlan.empty().free,
      pro: proJson is Map<String, dynamic> ? WoxBillingPlanTier.fromJson(proJson) : WoxBillingPlan.empty().pro,
    );
  }
}

class WoxBillingPlanTier {
  final WoxBillingPlanPrice price;
  final WoxSyncLimits limits;

  WoxBillingPlanTier({required this.price, required this.limits});

  factory WoxBillingPlanTier.fromJson(Map<String, dynamic> json) {
    final priceJson = json['price'];
    final limitsJson = json['limits'];
    return WoxBillingPlanTier(
      price: priceJson is Map<String, dynamic> ? WoxBillingPlanPrice.fromJson(priceJson) : WoxBillingPlanPrice.empty(),
      limits: limitsJson is Map<String, dynamic> ? WoxSyncLimits.fromJson(limitsJson) : WoxSyncLimits(deviceLimit: null, syncIntervalSeconds: null, syncWindowSeconds: null),
    );
  }
}

class WoxBillingPlanPrice {
  final String currency;
  final int? unitAmount;
  final String interval;
  final String formatted;

  WoxBillingPlanPrice({required this.currency, required this.unitAmount, required this.interval, required this.formatted});

  factory WoxBillingPlanPrice.empty() {
    return WoxBillingPlanPrice(currency: '', unitAmount: null, interval: '', formatted: '');
  }

  factory WoxBillingPlanPrice.fromJson(Map<String, dynamic> json) {
    return WoxBillingPlanPrice(currency: json['currency'] ?? '', unitAmount: json['unit_amount'], interval: json['interval'] ?? '', formatted: json['formatted'] ?? '');
  }
}

class WoxCloudSyncDevice {
  final String deviceId;
  final String deviceName;
  final String platform;
  final int createdAt;
  final int updatedAt;
  final int lastSeenAt;
  final int revokedAt;
  final bool current;

  WoxCloudSyncDevice({
    required this.deviceId,
    required this.deviceName,
    required this.platform,
    required this.createdAt,
    required this.updatedAt,
    required this.lastSeenAt,
    required this.revokedAt,
    required this.current,
  });

  factory WoxCloudSyncDevice.fromJson(Map<String, dynamic> json) {
    return WoxCloudSyncDevice(
      deviceId: json['device_id'] ?? '',
      deviceName: json['device_name'] ?? '',
      platform: json['platform'] ?? '',
      createdAt: json['created_at'] ?? 0,
      updatedAt: json['updated_at'] ?? 0,
      lastSeenAt: json['last_seen_at'] ?? 0,
      revokedAt: json['revoked_at'] ?? 0,
      current: json['current'] ?? false,
    );
  }

  bool get revoked => revokedAt > 0;
}

class WoxCloudSyncDeviceList {
  final List<WoxCloudSyncDevice> devices;
  final String currentDeviceId;
  final int? deviceLimit;
  final int deviceCount;

  WoxCloudSyncDeviceList({required this.devices, required this.currentDeviceId, required this.deviceLimit, required this.deviceCount});

  factory WoxCloudSyncDeviceList.empty() {
    return WoxCloudSyncDeviceList(devices: [], currentDeviceId: '', deviceLimit: 2, deviceCount: 0);
  }

  factory WoxCloudSyncDeviceList.fromJson(Map<String, dynamic> json) {
    final rawDevices = json['devices'];
    final devices = rawDevices is List ? rawDevices.whereType<Map<String, dynamic>>().map(WoxCloudSyncDevice.fromJson).toList() : <WoxCloudSyncDevice>[];
    return WoxCloudSyncDeviceList(
      devices: devices,
      currentDeviceId: json['current_device_id'] ?? '',
      deviceLimit: json['device_limit'],
      deviceCount: json['device_count'] ?? devices.where((device) => !device.revoked).length,
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
