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

  bool get hasOriginalTranscript => originalText.trim().isNotEmpty;

  const WoxPreviewDictationHistory({
    required this.refinedText,
    required this.originalText,
    required this.refinedLabel,
    required this.originalLabel,
    required this.statusLabel,
    required this.isChanged,
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
    );
  }
}
