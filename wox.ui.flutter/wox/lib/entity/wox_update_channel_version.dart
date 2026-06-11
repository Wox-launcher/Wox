class WoxUpdateChannelVersion {
  final String channel;
  final String latestVersion;
  final String error;

  const WoxUpdateChannelVersion({required this.channel, required this.latestVersion, required this.error});

  factory WoxUpdateChannelVersion.fromJson(dynamic json) {
    if (json is! Map) {
      return const WoxUpdateChannelVersion(channel: '', latestVersion: '', error: '');
    }

    return WoxUpdateChannelVersion(channel: json['Channel']?.toString() ?? '', latestVersion: json['LatestVersion']?.toString() ?? '', error: json['Error']?.toString() ?? '');
  }
}
