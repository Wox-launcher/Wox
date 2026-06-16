import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_screenshot_controller.dart';
import 'package:wox/entity/screenshot_session.dart';
import 'package:wox/modules/screenshot/views/wox_screenshot_view.dart';

void main() {
  tearDown(() {
    Get.reset();
  });

  testWidgets('annotation toolbar follows the selection bottom-right anchor', (tester) async {
    final controller = Get.put(WoxScreenshotController());
    controller.displaySnapshots.assignAll([
      DisplaySnapshot(
        displayId: 'display-a',
        logicalBounds: const ScreenshotRect(x: 0, y: 0, width: 800, height: 600),
        pixelBounds: const ScreenshotRect(x: 0, y: 0, width: 800, height: 600),
        scale: 1,
        rotation: 0,
        imageBytesBase64: _transparentPixelPngBase64,
      ),
    ]);
    controller.virtualBounds.value = const ScreenshotRect(x: 0, y: 0, width: 800, height: 600);
    controller.selection.value = const ScreenshotRect(x: 220, y: 100, width: 500, height: 160);
    controller.stage.value = ScreenshotSessionStage.annotating;

    await tester.pumpWidget(const GetMaterialApp(home: Material(child: SizedBox(width: 800, height: 600, child: WoxScreenshotView()))));
    await tester.pumpAndSettle();

    final toolbarRect = tester.getRect(find.byKey(screenshotToolbarKey));
    expect(toolbarRect.right, closeTo(720, 1));
    expect(toolbarRect.top, greaterThan(260));
  });

  testWidgets('annotation toolbar exposes scrolling capture after selecting a region', (tester) async {
    final controller = Get.put(WoxScreenshotController());
    controller.displaySnapshots.assignAll([
      DisplaySnapshot(
        displayId: 'display-a',
        logicalBounds: const ScreenshotRect(x: 0, y: 0, width: 800, height: 600),
        pixelBounds: const ScreenshotRect(x: 0, y: 0, width: 800, height: 600),
        scale: 1,
        rotation: 0,
        imageBytesBase64: _transparentPixelPngBase64,
      ),
    ]);
    controller.virtualBounds.value = const ScreenshotRect(x: 0, y: 0, width: 800, height: 600);
    controller.selection.value = const ScreenshotRect(x: 100, y: 100, width: 420, height: 160);
    controller.stage.value = ScreenshotSessionStage.annotating;

    await tester.pumpWidget(const GetMaterialApp(home: Material(child: SizedBox(width: 800, height: 600, child: WoxScreenshotView()))));
    await tester.pumpAndSettle();

    // Scrolling capture is a smoke-level toolbar check because the full workflow depends on native
    // wheel input and live desktop capture. Verifying the action appears keeps the user-facing entry
    // point covered without adding a brittle platform-specific integration test here.
    expect(find.byKey(screenshotScrollingCaptureKey), findsOneWidget);
  });

  testWidgets('selected annotation edit bar falls back to the left side when the selection has no room on the right', (tester) async {
    final controller = Get.put(WoxScreenshotController());
    controller.displaySnapshots.assignAll([
      DisplaySnapshot(
        displayId: 'display-a',
        logicalBounds: const ScreenshotRect(x: 0, y: 0, width: 800, height: 600),
        pixelBounds: const ScreenshotRect(x: 0, y: 0, width: 800, height: 600),
        scale: 1,
        rotation: 0,
        imageBytesBase64: _transparentPixelPngBase64,
      ),
    ]);
    controller.virtualBounds.value = const ScreenshotRect(x: 0, y: 0, width: 800, height: 600);
    controller.selection.value = const ScreenshotRect(x: 560, y: 120, width: 200, height: 220);
    controller.annotations.assignAll([ScreenshotAnnotation(id: 'rect-a', type: ScreenshotAnnotationType.rect, rect: const Rect.fromLTWH(600, 180, 80, 60))]);
    controller.selectAnnotation('rect-a');
    controller.stage.value = ScreenshotSessionStage.annotating;

    await tester.pumpWidget(const GetMaterialApp(home: Material(child: SizedBox(width: 800, height: 600, child: WoxScreenshotView()))));
    await tester.pumpAndSettle();

    final editBarRect = tester.getRect(find.byKey(screenshotEditBarKey));
    expect(editBarRect.right, lessThan(560));
  });

  test('shape annotations render at the pointer position inside the selection', () async {
    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    paintWorkspaceAnnotations(
      canvas,
      annotations: [ScreenshotAnnotation(id: 'rect-a', type: ScreenshotAnnotationType.rect, rect: const Rect.fromLTWH(120, 130, 60, 40))],
      canvasOrigin: Offset.zero,
      selectionClipRect: const Rect.fromLTWH(100, 100, 180, 120),
    );
    final picture = recorder.endRecording();
    final image = await picture.toImage(400, 300);
    final expectedPixel = await _readPixel(image, 150, 130);
    final wrongOffsetPixel = await _readPixel(image, 50, 30);

    expect(_channelToByte(expectedPixel.r), greaterThan(220));
    expect(_channelToByte(expectedPixel.g), lessThan(120));
    expect(_channelToByte(wrongOffsetPixel.r), lessThan(180));

    image.dispose();
  });
}

const String _transparentPixelPngBase64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9pW4n1sAAAAASUVORK5CYII=';

Future<Color> _readPixel(ui.Image image, int x, int y) async {
  final byteData = await image.toByteData(format: ui.ImageByteFormat.rawRgba);
  if (byteData == null) {
    throw StateError('Failed to inspect rendered screenshot pixel');
  }

  final width = image.width;
  final offset = (y * width + x) * 4;
  return Color.fromARGB(byteData.getUint8(offset + 3), byteData.getUint8(offset), byteData.getUint8(offset + 1), byteData.getUint8(offset + 2));
}

int _channelToByte(double value) => (value * 255).round().clamp(0, 255);
