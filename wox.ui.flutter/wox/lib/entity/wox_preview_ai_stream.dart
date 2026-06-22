import 'dart:convert';

class WoxPreviewAIStream {
  final String answer;
  final String reasoning;
  final String status;
  final String statusLabel;
  final String reasoningTitle;
  final String answerTitle;

  const WoxPreviewAIStream({
    required this.answer,
    required this.reasoning,
    required this.status,
    required this.statusLabel,
    required this.reasoningTitle,
    required this.answerTitle,
  });

  factory WoxPreviewAIStream.fromPreviewData(String previewData) {
    final decoded = jsonDecode(previewData);
    final data = decoded is Map<String, dynamic> ? decoded : <String, dynamic>{};

    // AI stream previews keep answer and reasoning separate so the renderer can
    // preserve the shared text-preview frame while giving reasoning lower visual
    // priority. Missing fields degrade to empty strings for malformed updates.
    return WoxPreviewAIStream(
      answer: data["answer"]?.toString() ?? "",
      reasoning: data["reasoning"]?.toString() ?? "",
      status: data["status"]?.toString() ?? "",
      statusLabel: data["statusLabel"]?.toString() ?? "",
      reasoningTitle: data["reasoningTitle"]?.toString() ?? "",
      answerTitle: data["answerTitle"]?.toString() ?? "",
    );
  }
}
