import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:wox/controllers/wox_screenshot_controller.dart';
import 'package:wox/entity/screenshot_session.dart';

void main() {
  test('selected annotation updates keep per-annotation color and text font size independent', () {
    final controller = WoxScreenshotController();
    controller.annotations.addAll([
      ScreenshotAnnotation(id: 'rect-a', type: ScreenshotAnnotationType.rect, rect: const Rect.fromLTWH(40, 40, 60, 40), color: Colors.red),
      ScreenshotAnnotation(id: 'text-a', type: ScreenshotAnnotationType.text, start: const Offset(80, 90), text: 'Hello', color: Colors.green, fontSize: 24),
    ]);

    controller.selectAnnotation('rect-a');
    controller.updateSelectedAnnotationColor(Colors.blue);
    controller.selectAnnotation('text-a');
    controller.updateSelectedTextFontSize(8);

    expect(controller.annotations[0].color, Colors.blue);
    expect(controller.annotations[1].color, Colors.green);
    expect(controller.annotations[1].fontSize, 32);
  });

  test('double-click text edit updates existing content instead of creating a new annotation', () {
    final controller = WoxScreenshotController();
    controller.annotations.addAll([
      ScreenshotAnnotation(id: 'text-a', type: ScreenshotAnnotationType.text, start: const Offset(80, 90), text: 'Before', color: Colors.orange, fontSize: 20),
    ]);

    controller.startTextDraft(const Offset(80, 90), annotationId: 'text-a', initialText: 'Before', fontSize: 20, color: Colors.orange);
    controller.textDraftController.text = 'After';
    controller.commitTextDraft();

    expect(controller.annotations, hasLength(1));
    expect(controller.annotations.single.text, 'After');
    expect(controller.annotations.single.fontSize, 20);
  });
}
