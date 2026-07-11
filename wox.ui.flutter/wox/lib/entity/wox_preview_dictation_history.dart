import 'dart:convert';

// WoxPreviewDictationHistory is the structured payload for the dedicated
// dictation result comparison surface.
class WoxPreviewDictationHistory {
  final String refinedText;
  final String originalText;
  final String refinedLabel;
  final String originalLabel;
  final String statusLabel;
  final bool isChanged;
  final String rawAudioPath;
  final String processedAudioPath;
  final String audioLabel;
  final String rawAudioLabel;
  final String processedAudioLabel;

  bool get hasOriginalTranscript => originalText.trim().isNotEmpty;
  bool get hasDiagnosticAudio => rawAudioPath.trim().isNotEmpty && processedAudioPath.trim().isNotEmpty;

  const WoxPreviewDictationHistory({
    required this.refinedText,
    required this.originalText,
    required this.refinedLabel,
    required this.originalLabel,
    required this.statusLabel,
    required this.isChanged,
    required this.rawAudioPath,
    required this.processedAudioPath,
    required this.audioLabel,
    required this.rawAudioLabel,
    required this.processedAudioLabel,
  });

  // fromPreviewData tolerates absent fields so an interrupted preview update
  // can still render its available transcript instead of failing the panel.
  factory WoxPreviewDictationHistory.fromPreviewData(String previewData) {
    final decoded = jsonDecode(previewData);
    final data = decoded is Map<String, dynamic> ? decoded : <String, dynamic>{};

    return WoxPreviewDictationHistory(
      refinedText: data["refinedText"]?.toString() ?? "",
      originalText: data["originalText"]?.toString() ?? "",
      refinedLabel: data["refinedLabel"]?.toString() ?? "",
      originalLabel: data["originalLabel"]?.toString() ?? "",
      statusLabel: data["statusLabel"]?.toString() ?? "",
      isChanged: data["isChanged"] == true,
      rawAudioPath: data["rawAudioPath"]?.toString() ?? "",
      processedAudioPath: data["processedAudioPath"]?.toString() ?? "",
      audioLabel: data["audioLabel"]?.toString() ?? "",
      rawAudioLabel: data["rawAudioLabel"]?.toString() ?? "",
      processedAudioLabel: data["processedAudioLabel"]?.toString() ?? "",
    );
  }
}
