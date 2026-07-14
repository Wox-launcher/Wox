enum MacOSPermissionState {
  granted,
  notGranted,
  unknown;

  factory MacOSPermissionState.fromJson(dynamic value) {
    return MacOSPermissionState.values.firstWhere((state) => state.name == value, orElse: () => MacOSPermissionState.unknown);
  }
}

class MacOSPermissionStatus {
  const MacOSPermissionStatus({required this.accessibility, required this.fullDiskAccess});

  const MacOSPermissionStatus.unknown() : accessibility = MacOSPermissionState.unknown, fullDiskAccess = MacOSPermissionState.unknown;

  final MacOSPermissionState accessibility;
  final MacOSPermissionState fullDiskAccess;

  factory MacOSPermissionStatus.fromJson(Map<String, dynamic> json) {
    return MacOSPermissionStatus(accessibility: MacOSPermissionState.fromJson(json['accessibility']), fullDiskAccess: MacOSPermissionState.fromJson(json['fullDiskAccess']));
  }
}
