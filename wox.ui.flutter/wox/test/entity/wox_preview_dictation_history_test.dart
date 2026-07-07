import 'package:flutter_test/flutter_test.dart';
import 'package:wox/entity/wox_preview_dictation_history.dart';

void main() {
  test('dictation correction display keeps only changed phrase as replacement segment', () {
    final data = WoxDictationHistoryPreviewData(
      recordId: 'record-1',
      originalContent: '拼写设置里面怎么看不到字典的table?',
      content: '听写设置里面怎么看不到字典的table?',
      timestamp: 1000,
      corrections: [
        WoxDictationHistoryCorrection(selectedText: '拼写', replacementText: '听写', previousContent: '拼写设置里面怎么看不到字典的table?', updatedContent: '听写设置里面怎么看不到字典的table?', timestamp: 1001),
      ],
    );

    final display = data.buildCorrectionDisplay();

    expect(display.displayText, '拼写听写设置里面怎么看不到字典的table?');
    expect(display.contentText, data.content);
    expect(display.segments.length, 2);
    expect(display.segments.first.isCorrection, isTrue);
    expect(display.segments.first.oldText, '拼写');
    expect(display.segments.first.newText, '听写');
    expect(display.segments.last.text, '设置里面怎么看不到字典的table?');
  });

  test('dictation correction display maps visible offsets back to current content', () {
    final data = WoxDictationHistoryPreviewData(
      recordId: 'record-1',
      originalContent: '拼写设置里面怎么看不到字典的table?',
      content: '听写设置里面怎么看不到字典的table?',
      timestamp: 1000,
      corrections: [
        WoxDictationHistoryCorrection(selectedText: '拼写', replacementText: '听写', previousContent: '拼写设置里面怎么看不到字典的table?', updatedContent: '听写设置里面怎么看不到字典的table?', timestamp: 1001),
      ],
    );

    final display = data.buildCorrectionDisplay();
    final visibleStart = display.displayText.indexOf('设置');
    final visibleEnd = visibleStart + '设置'.length;
    final contentRange = display.contentRangeForDisplayRange(visibleStart, visibleEnd);

    expect(data.content.substring(contentRange.start, contentRange.end), '设置');
  });
}
