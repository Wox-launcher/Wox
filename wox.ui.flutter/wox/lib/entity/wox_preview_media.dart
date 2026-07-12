import 'dart:convert';

// WoxPreviewMedia is the structured payload for the dedicated now-playing surface.
class WoxPreviewMedia {
  final String title;
  final String artist;
  final String album;
  final String appName;
  final String artwork;
  final int position;
  final int duration;
  final bool isPlaying;

  int get trackIdentity => Object.hash(title, artist, album, artwork.hashCode);

  const WoxPreviewMedia({
    required this.title,
    required this.artist,
    required this.album,
    required this.appName,
    required this.artwork,
    required this.position,
    required this.duration,
    required this.isPlaying,
  });

  // Missing metadata remains renderable because media providers expose different field sets.
  factory WoxPreviewMedia.fromPreviewData(String previewData) {
    final decoded = jsonDecode(previewData);
    final data = decoded is Map<String, dynamic> ? decoded : <String, dynamic>{};

    int readSeconds(String key) {
      final value = data[key];
      return value is int ? value : int.tryParse(value?.toString() ?? "") ?? 0;
    }

    return WoxPreviewMedia(
      title: data["title"]?.toString() ?? "",
      artist: data["artist"]?.toString() ?? "",
      album: data["album"]?.toString() ?? "",
      appName: data["appName"]?.toString() ?? "",
      artwork: data["artwork"]?.toString() ?? "",
      position: readSeconds("position"),
      duration: readSeconds("duration"),
      isPlaying: data["isPlaying"] == true,
    );
  }
}
