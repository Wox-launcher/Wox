import 'dart:convert';

import 'package:wox/entity/wox_preview.dart';

class WoxDictationHistoryCorrection {
  final String selectedText;
  final String replacementText;
  final String previousContent;
  final String updatedContent;
  final int timestamp;

  WoxDictationHistoryCorrection({required this.selectedText, required this.replacementText, required this.previousContent, required this.updatedContent, required this.timestamp});

  factory WoxDictationHistoryCorrection.fromJson(Map<String, dynamic> json) {
    return WoxDictationHistoryCorrection(
      selectedText: json["selectedText"]?.toString() ?? "",
      replacementText: json["replacementText"]?.toString() ?? "",
      previousContent: json["previousContent"]?.toString() ?? "",
      updatedContent: json["updatedContent"]?.toString() ?? "",
      timestamp: int.tryParse(json["timestamp"]?.toString() ?? "") ?? 0,
    );
  }
}

class WoxDictationHistoryPreviewData {
  final String recordId;
  final String originalContent;
  final String content;
  final int timestamp;
  final List<WoxDictationHistoryCorrection> corrections;

  WoxDictationHistoryPreviewData({required this.recordId, required this.originalContent, required this.content, required this.timestamp, this.corrections = const []});

  factory WoxDictationHistoryPreviewData.fromJson(Map<String, dynamic> json) {
    final rawCorrections = json["corrections"];
    return WoxDictationHistoryPreviewData(
      recordId: json["recordId"]?.toString() ?? "",
      originalContent: json["originalContent"]?.toString() ?? "",
      content: json["content"]?.toString() ?? "",
      timestamp: int.tryParse(json["timestamp"]?.toString() ?? "") ?? 0,
      corrections: rawCorrections is List ? rawCorrections.whereType<Map<String, dynamic>>().map(WoxDictationHistoryCorrection.fromJson).toList() : const [],
    );
  }

  factory WoxDictationHistoryPreviewData.fromPreviewData(String previewData) {
    return WoxDictationHistoryPreviewData.fromJson(jsonDecode(previewData));
  }

  WoxDictationHistoryPreviewData copyWith({String? originalContent, String? content, int? timestamp, List<WoxDictationHistoryCorrection>? corrections}) {
    return WoxDictationHistoryPreviewData(
      recordId: recordId,
      originalContent: originalContent ?? this.originalContent,
      content: content ?? this.content,
      timestamp: timestamp ?? this.timestamp,
      corrections: corrections ?? this.corrections,
    );
  }

  WoxDictationHistoryCorrectionDisplay buildCorrectionDisplay() {
    return WoxDictationHistoryCorrectionDisplay.build(originalContent: originalContent, content: content, corrections: corrections);
  }
}

class WoxTextRange {
  final int start;
  final int end;

  const WoxTextRange(this.start, this.end);
}

class WoxDictationHistoryCorrectionDisplaySegment {
  final String text;
  final String oldText;
  final String newText;

  const WoxDictationHistoryCorrectionDisplaySegment.text(this.text) : oldText = "", newText = "";

  const WoxDictationHistoryCorrectionDisplaySegment.correction({required this.oldText, required this.newText}) : text = "";

  bool get isCorrection => oldText.isNotEmpty || newText.isNotEmpty;

  String get contentText => isCorrection ? newText : text;

  String get displayText => isCorrection ? oldText + newText : text;
}

class WoxDictationHistoryCorrectionDisplay {
  final List<WoxDictationHistoryCorrectionDisplaySegment> segments;
  final String contentText;
  final String displayText;
  final List<int> _displayToContentOffsets;

  const WoxDictationHistoryCorrectionDisplay._({required this.segments, required this.contentText, required this.displayText, required List<int> displayToContentOffsets})
    : _displayToContentOffsets = displayToContentOffsets;

  factory WoxDictationHistoryCorrectionDisplay.build({required String originalContent, required String content, required List<WoxDictationHistoryCorrection> corrections}) {
    if (corrections.isEmpty || originalContent.trim().isEmpty) {
      return WoxDictationHistoryCorrectionDisplay._fromSegments([WoxDictationHistoryCorrectionDisplaySegment.text(content)]);
    }

    var segments = <WoxDictationHistoryCorrectionDisplaySegment>[WoxDictationHistoryCorrectionDisplaySegment.text(originalContent)];
    for (final correction in corrections) {
      final diff = _CorrectionDiff.fromCorrection(correction);
      final currentContent = _segmentsContentText(segments);
      if (diff == null || currentContent != correction.previousContent) {
        return WoxDictationHistoryCorrectionDisplay._fromSegments([WoxDictationHistoryCorrectionDisplaySegment.text(content)]);
      }
      segments = _applyCorrectionSegment(segments, diff.start, diff.end, diff.oldText, diff.newText);
    }

    if (_segmentsContentText(segments) != content) {
      return WoxDictationHistoryCorrectionDisplay._fromSegments([WoxDictationHistoryCorrectionDisplaySegment.text(content)]);
    }

    return WoxDictationHistoryCorrectionDisplay._fromSegments(segments);
  }

  factory WoxDictationHistoryCorrectionDisplay._fromSegments(List<WoxDictationHistoryCorrectionDisplaySegment> segments) {
    final contentBuffer = StringBuffer();
    final displayBuffer = StringBuffer();
    final offsets = <int>[0];
    var contentOffset = 0;

    for (final segment in segments) {
      if (segment.isCorrection) {
        for (var i = 0; i < segment.oldText.length; i++) {
          displayBuffer.write(segment.oldText[i]);
          offsets.add(contentOffset);
        }
        for (var i = 0; i < segment.newText.length; i++) {
          displayBuffer.write(segment.newText[i]);
          contentBuffer.write(segment.newText[i]);
          contentOffset++;
          offsets.add(contentOffset);
        }
      } else {
        for (var i = 0; i < segment.text.length; i++) {
          displayBuffer.write(segment.text[i]);
          contentBuffer.write(segment.text[i]);
          contentOffset++;
          offsets.add(contentOffset);
        }
      }
    }

    return WoxDictationHistoryCorrectionDisplay._(
      segments: segments,
      contentText: contentBuffer.toString(),
      displayText: displayBuffer.toString(),
      displayToContentOffsets: offsets,
    );
  }

  bool get hasCorrections => segments.any((segment) => segment.isCorrection);

  WoxTextRange contentRangeForDisplayRange(int start, int end) {
    final normalizedStart = start.clamp(0, _displayToContentOffsets.length - 1);
    final normalizedEnd = end.clamp(0, _displayToContentOffsets.length - 1);
    return WoxTextRange(_displayToContentOffsets[normalizedStart], _displayToContentOffsets[normalizedEnd]);
  }
}

class _CorrectionDiff {
  final int start;
  final int end;
  final String oldText;
  final String newText;

  const _CorrectionDiff({required this.start, required this.end, required this.oldText, required this.newText});

  static _CorrectionDiff? fromCorrection(WoxDictationHistoryCorrection correction) {
    final previous = correction.previousContent;
    final updated = correction.updatedContent;
    if (previous.isEmpty || updated.isEmpty || previous == updated) {
      return null;
    }

    final selectedText = correction.selectedText;
    final replacementText = correction.replacementText;
    if (selectedText.isNotEmpty && replacementText.isNotEmpty) {
      final selectedStart = previous.indexOf(selectedText);
      if (selectedStart >= 0) {
        return _CorrectionDiff(start: selectedStart, end: selectedStart + selectedText.length, oldText: selectedText, newText: replacementText);
      }
    }

    var prefix = 0;
    while (prefix < previous.length && prefix < updated.length && previous[prefix] == updated[prefix]) {
      prefix++;
    }

    var previousSuffix = previous.length;
    var updatedSuffix = updated.length;
    while (previousSuffix > prefix && updatedSuffix > prefix && previous[previousSuffix - 1] == updated[updatedSuffix - 1]) {
      previousSuffix--;
      updatedSuffix--;
    }

    final oldText = previous.substring(prefix, previousSuffix);
    final newText = updated.substring(prefix, updatedSuffix);
    if (oldText.isEmpty && newText.isEmpty) {
      return null;
    }

    return _CorrectionDiff(
      start: prefix,
      end: previousSuffix,
      oldText: oldText.isNotEmpty ? oldText : correction.selectedText,
      newText: newText.isNotEmpty ? newText : correction.replacementText,
    );
  }
}

String _segmentsContentText(List<WoxDictationHistoryCorrectionDisplaySegment> segments) {
  return segments.map((segment) => segment.contentText).join();
}

List<WoxDictationHistoryCorrectionDisplaySegment> _applyCorrectionSegment(
  List<WoxDictationHistoryCorrectionDisplaySegment> segments,
  int start,
  int end,
  String oldText,
  String newText,
) {
  final updated = <WoxDictationHistoryCorrectionDisplaySegment>[];
  var cursor = 0;
  var inserted = false;

  for (final segment in segments) {
    final segmentText = segment.contentText;
    final segmentStart = cursor;
    final segmentEnd = cursor + segmentText.length;

    if (segmentEnd <= start || segmentStart >= end) {
      updated.add(segment);
    } else {
      final prefixEnd = (start - segmentStart).clamp(0, segmentText.length);
      if (prefixEnd > 0) {
        updated.add(WoxDictationHistoryCorrectionDisplaySegment.text(segmentText.substring(0, prefixEnd)));
      }
      if (!inserted) {
        updated.add(WoxDictationHistoryCorrectionDisplaySegment.correction(oldText: oldText, newText: newText));
        inserted = true;
      }
      final suffixStart = (end - segmentStart).clamp(0, segmentText.length);
      if (suffixStart < segmentText.length) {
        updated.add(WoxDictationHistoryCorrectionDisplaySegment.text(segmentText.substring(suffixStart)));
      }
    }

    cursor = segmentEnd;
  }

  if (!inserted) {
    updated.add(WoxDictationHistoryCorrectionDisplaySegment.correction(oldText: oldText, newText: newText));
  }

  return updated.where((segment) => segment.displayText.isNotEmpty).toList();
}

class WoxDictationHistoryCorrectionResponse {
  final String recordId;
  final String originalContent;
  final String content;
  final int timestamp;
  final String title;
  final WoxPreview preview;

  WoxDictationHistoryCorrectionResponse({
    required this.recordId,
    required this.originalContent,
    required this.content,
    required this.timestamp,
    required this.title,
    required this.preview,
  });

  factory WoxDictationHistoryCorrectionResponse.fromJson(Map<String, dynamic> json) {
    return WoxDictationHistoryCorrectionResponse(
      recordId: json["recordId"]?.toString() ?? "",
      originalContent: json["originalContent"]?.toString() ?? "",
      content: json["content"]?.toString() ?? "",
      timestamp: int.tryParse(json["timestamp"]?.toString() ?? "") ?? 0,
      title: json["title"]?.toString() ?? "",
      preview: WoxPreview.fromJson(Map<String, dynamic>.from(json["preview"] as Map)),
    );
  }

  WoxDictationHistoryPreviewData toPreviewData() {
    if (preview.previewData.isNotEmpty) {
      return WoxDictationHistoryPreviewData.fromPreviewData(preview.previewData);
    }
    return WoxDictationHistoryPreviewData(recordId: recordId, originalContent: originalContent, content: content, timestamp: timestamp);
  }
}
