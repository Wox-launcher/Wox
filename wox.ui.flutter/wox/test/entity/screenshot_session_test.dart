import 'package:flutter_test/flutter_test.dart';
import 'package:wox/entity/screenshot_session.dart';

void main() {
  group('ScreenshotSession entity parsing', () {
    test('DisplaySnapshot.fromJson accepts json-like nested maps from method channels', () {
      final json = <String, dynamic>{
        'displayId': 'display-a',
        'logicalBounds': <Object?, Object?>{'x': 0, 'y': 0, 'width': 1920, 'height': 1080},
        'pixelBounds': <Object?, Object?>{'x': 0, 'y': 0, 'width': 1920, 'height': 1080},
        'scale': 1,
        'rotation': 0,
        'imageBytesBase64': '',
      };

      expect(() => DisplaySnapshot.fromJson(json), returnsNormally, reason: 'MethodChannel payloads use json-like maps whose nested values are typed as Map<Object?, Object?>.');
    });

    test('ScreenshotWorkspacePresentation.fromJson accepts json-like nested maps from method channels', () {
      final json = <String, dynamic>{
        'workspaceBounds': <Object?, Object?>{'x': 120, 'y': 40, 'width': 800, 'height': 600},
        'workspaceScale': 2,
        'presentedByPlatform': true,
      };

      final presentation = ScreenshotWorkspacePresentation.fromJson(json);
      expect(presentation.workspaceBounds.x, 120);
      expect(presentation.workspaceBounds.y, 40);
      expect(presentation.workspaceBounds.width, 800);
      expect(presentation.workspaceBounds.height, 600);
      expect(presentation.workspaceScale, 2);
      expect(presentation.presentedByPlatform, isTrue);
    });

    test('ScreenshotNativeSelectionResult.fromJson accepts an optional selection payload', () {
      final json = <String, dynamic>{
        'wasHandled': true,
        'selection': <Object?, Object?>{'x': 24, 'y': 32, 'width': 180, 'height': 96},
        'editorVisibleBounds': <Object?, Object?>{'x': 0, 'y': 0, 'width': 1280, 'height': 720},
      };

      final selection = ScreenshotNativeSelectionResult.fromJson(json);

      expect(selection.wasHandled, isTrue);
      expect(selection.selection, const ScreenshotRect(x: 24, y: 32, width: 180, height: 96));
      expect(selection.editorVisibleBounds, const ScreenshotRect(x: 0, y: 0, width: 1280, height: 720));
    });
  });
}
