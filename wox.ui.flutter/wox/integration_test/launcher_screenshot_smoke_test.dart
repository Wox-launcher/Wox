import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:get/get.dart';
import 'package:integration_test/integration_test.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_screenshot_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/screenshot_session.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/screenshot/views/wox_screenshot_view.dart';
import 'package:wox/utils/screenshot/screenshot_platform_bridge.dart';

import 'smoke_test_helper.dart';

class _FakeScreenshotBridge implements ScreenshotPlatformBridge {
  _FakeScreenshotBridge(this._capture, {this.nativeSelection, this.presentation, this.dismissNativeOverlays, this.delegateNativePresentation = false});

  final Future<List<DisplaySnapshot>> Function() _capture;
  final Future<ScreenshotNativeSelectionResult> Function(ScreenshotRect nativeWorkspaceBounds)? nativeSelection;
  final Future<ScreenshotWorkspacePresentation> Function(ScreenshotRect nativeWorkspaceBounds)? presentation;
  final Future<void> Function()? dismissNativeOverlays;
  final bool delegateNativePresentation;
  final ScreenshotPlatformBridge _delegate = MethodChannelScreenshotPlatformBridge();

  @override
  Future<List<DisplaySnapshot>> captureAllDisplays({ScreenshotRect? logicalSelection, String? traceId}) => _capture();

  @override
  Future<List<DisplaySnapshot>> captureDisplayMetadata() async {
    // Native-presentation smoke tests still need deterministic display metadata.
    // Delegating monitor capture to the real platform bridge made the test fail
    // before presentation state could be validated when bitmap capture was not
    // available in the integration-test environment.
    return _capture();
  }

  @override
  Future<List<DisplaySnapshot>> loadDisplaySnapshots(List<String> displayIds) async {
    // Keep snapshot bytes paired with the fake metadata above; native delegation
    // is reserved for presentation/debug APIs, not for environment-sensitive
    // monitor bitmap capture.
    final snapshots = await _capture();
    if (displayIds.isEmpty) {
      return snapshots;
    }

    final displayIdSet = displayIds.toSet();
    return snapshots.where((snapshot) => displayIdSet.contains(snapshot.displayId)).toList();
  }

  @override
  Future<ScreenshotNativeSelectionResult> selectCaptureRegion(ScreenshotRect nativeWorkspaceBounds) async {
    if (nativeSelection != null) {
      return nativeSelection!(nativeWorkspaceBounds);
    }
    return const ScreenshotNativeSelectionResult(wasHandled: false);
  }

  @override
  Future<ScreenshotWorkspacePresentation> presentCaptureWorkspace(ScreenshotRect nativeWorkspaceBounds, {int? windowHandle}) async {
    if (presentation != null) {
      return presentation!(nativeWorkspaceBounds);
    }
    if (delegateNativePresentation) {
      return _delegate.presentCaptureWorkspace(nativeWorkspaceBounds, windowHandle: windowHandle);
    }
    return ScreenshotWorkspacePresentation(workspaceBounds: nativeWorkspaceBounds, workspaceScale: 1, presentedByPlatform: false);
  }

  @override
  Future<ScreenshotWorkspacePresentation> prepareCaptureWorkspace(ScreenshotRect nativeWorkspaceBounds, {int? windowHandle}) async {
    if (presentation != null) {
      return presentation!(nativeWorkspaceBounds);
    }
    if (delegateNativePresentation) {
      return _delegate.prepareCaptureWorkspace(nativeWorkspaceBounds, windowHandle: windowHandle);
    }
    return ScreenshotWorkspacePresentation(workspaceBounds: nativeWorkspaceBounds, workspaceScale: 1, presentedByPlatform: false);
  }

  @override
  Future<void> revealPreparedCaptureWorkspace({int? windowHandle}) async {
    if (delegateNativePresentation) {
      await _delegate.revealPreparedCaptureWorkspace(windowHandle: windowHandle);
    }
  }

  @override
  Future<void> focusCaptureWorkspace({int? windowHandle}) async {
    if (delegateNativePresentation) {
      await _delegate.focusCaptureWorkspace(windowHandle: windowHandle);
    }
  }

  @override
  Stream<ScreenshotSelectionDisplayHint> selectionDisplayHints() {
    if (delegateNativePresentation) {
      return _delegate.selectionDisplayHints();
    }
    return const Stream<ScreenshotSelectionDisplayHint>.empty();
  }

  @override
  Future<void> dismissCaptureWorkspacePresentation({int? windowHandle}) async {
    if (delegateNativePresentation) {
      await _delegate.dismissCaptureWorkspacePresentation(windowHandle: windowHandle);
    }
  }

  @override
  Future<void> dismissNativeSelectionOverlays() async {
    if (dismissNativeOverlays != null) {
      await dismissNativeOverlays!();
      return;
    }
    if (delegateNativePresentation) {
      await _delegate.dismissNativeSelectionOverlays();
    }
  }

  @override
  Future<Map<String, dynamic>> debugCaptureWorkspaceState() async {
    if (delegateNativePresentation) {
      return _delegate.debugCaptureWorkspaceState();
    }
    return <String, dynamic>{};
  }

  @override
  Future<void> writeClipboardImageFile({required String filePath}) async {
    if (delegateNativePresentation) {
      await _delegate.writeClipboardImageFile(filePath: filePath);
    }
  }

  @override
  Future<void> moveMouseTo(Offset position) async {
    if (delegateNativePresentation) {
      await _delegate.moveMouseTo(position);
    }
  }

  @override
  Future<void> scrollMouse({required double deltaY}) async {
    if (delegateNativePresentation) {
      await _delegate.scrollMouse(deltaY: deltaY);
    }
  }

  @override
  Future<void> beginScrollingCaptureOverlay({
    required ScreenshotRect workspaceBounds,
    required ScreenshotRect selection,
    required ScreenshotRect controlsBounds,
    int? windowHandle,
    String? traceId,
  }) async {
    if (delegateNativePresentation) {
      // The fake bridge mirrors the production bridge's optional trace id so
      // screenshot smoke tests keep compiling when observability data is added
      // to bridge calls without changing the fake capture behavior.
      await _delegate.beginScrollingCaptureOverlay(
        workspaceBounds: workspaceBounds,
        selection: selection,
        controlsBounds: controlsBounds,
        windowHandle: windowHandle,
        traceId: traceId,
      );
    }
  }

  @override
  Future<void> moveScrollingCaptureControlsWindow({required ScreenshotRect controlsBounds, int? windowHandle}) async {
    if (delegateNativePresentation) {
      await _delegate.moveScrollingCaptureControlsWindow(controlsBounds: controlsBounds, windowHandle: windowHandle);
    }
  }

  @override
  Stream<ScrollingCaptureWheelEvent> scrollingCaptureWheelEvents() {
    if (delegateNativePresentation) {
      return _delegate.scrollingCaptureWheelEvents();
    }
    return const Stream<ScrollingCaptureWheelEvent>.empty();
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  registerLauncherScreenshotSmokeTests();
}

void registerLauncherScreenshotSmokeTests() {
  group('T11: Screenshot Smoke Tests', () {
    testWidgets('T11-01: Screenshot flow exports a non-empty PNG after multi-display selection', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(() async {
          return [
            await _buildSnapshot('display-a', const Color(0xFF273469), const ScreenshotRect(x: 0, y: 0, width: 400, height: 300)),
            await _buildSnapshot('display-b', const Color(0xFF7A306C), const ScreenshotRect(x: 400, y: 0, width: 400, height: 300)),
          ];
        }),
      );

      Map<String, dynamic>? sessionResult;
      Object? sessionError;
      final sessionFuture = screenshotController
          .startCaptureSession('smoke-success', _defaultRequest())
          .then((value) {
            final json = value.toJson();
            sessionResult = json;
            return json;
          })
          .catchError((error) {
            sessionError = error;
            throw error;
          });
      await pumpUntil(tester, () => find.byKey(screenshotCanvasKey).evaluate().isNotEmpty || sessionResult != null || sessionError != null, timeout: const Duration(seconds: 15));

      expect(sessionError, isNull);
      expect(sessionResult, isNull, reason: 'Screenshot session completed before the workspace became interactive.');
      expect(screenshotController.isSessionActive.value, isTrue);
      expect(screenshotController.virtualBoundsRect.width, equals(800));
      expect(screenshotController.virtualBoundsRect.height, equals(300));
      expect(find.byType(WoxScreenshotView), findsOneWidget);

      final canvasFinder = find.byKey(screenshotCanvasKey);
      expect(canvasFinder, findsOneWidget);
      final canvasOrigin = tester.getTopLeft(canvasFinder);
      await tester.dragFrom(canvasOrigin + const Offset(80, 50), const Offset(280, 150));
      await tester.pump(const Duration(milliseconds: 250));

      final selectionRect = screenshotController.selectionRect;
      expect(selectionRect, isNotNull);
      expect(selectionRect!.width, greaterThan(200));
      expect(selectionRect.height, greaterThan(120));

      // The integration-test host can render the floating toolbar partially outside the
      // hit-testable root once the selection moves near the edge. Drive the controller directly
      // so the smoke test keeps validating the screenshot workflow instead of toolbar hit tests.
      screenshotController.setTool(ScreenshotTool.rect);
      await tester.pump();
      expect(screenshotController.currentTool.value, ScreenshotTool.rect);

      screenshotController.addShapeAnnotation(ScreenshotAnnotationType.rect, Rect.fromLTWH(selectionRect.left + 18, selectionRect.top + 16, 90, 44));
      screenshotController.addShapeAnnotation(ScreenshotAnnotationType.ellipse, Rect.fromLTWH(selectionRect.left + 130, selectionRect.top + 40, 76, 50));
      screenshotController.addArrowAnnotation(selectionRect.topLeft + const Offset(24, 96), selectionRect.topLeft + const Offset(160, 108));
      screenshotController.setAnnotationCreationColor(const Color(0xFFF9C74F));
      screenshotController.setMosaicBrushRadius(screenshotMosaicBrushRadii.last);
      final mosaicAnnotationId = screenshotController.addMosaicAnnotation(selectionRect.topLeft + const Offset(215, 78));
      screenshotController.appendMosaicPoint(mosaicAnnotationId, selectionRect.topLeft + const Offset(255, 110));
      expect(screenshotController.annotationById(mosaicAnnotationId)?.color, equals(const Color(0xFF29FF72)));
      expect(screenshotController.annotationById(mosaicAnnotationId)?.mosaicRadius, equals(screenshotMosaicBrushRadii.last));
      screenshotController.selectAnnotation(mosaicAnnotationId);
      screenshotController.updateSelectedMosaicBrushRadius(screenshotMosaicBrushRadii.first);
      expect(screenshotController.annotationById(mosaicAnnotationId)?.mosaicRadius, equals(screenshotMosaicBrushRadii.first));
      screenshotController.selectAnnotation(null);
      screenshotController.startTextDraft(selectionRect.topLeft + const Offset(32, 20));
      screenshotController.textDraftController.text = 'Smoke';
      screenshotController.commitTextDraft();
      expect(screenshotController.annotations.length, equals(5));

      screenshotController.undoAnnotation();
      expect(screenshotController.annotations.length, equals(4));
      screenshotController.startTextDraft(selectionRect.topLeft + const Offset(32, 20));
      screenshotController.textDraftController.text = 'Smoke';
      screenshotController.commitTextDraft();
      expect(screenshotController.annotations.length, equals(5));

      // Bug fix: macOS integration smoke can render the floating screenshot toolbar
      // outside the reliable hit-test root after repeated window handoffs. Confirm
      // through the controller so this smoke validates export instead of toolbar geometry.
      await screenshotController.confirmSelection('smoke-success-confirm');
      final result = await sessionFuture.timeout(const Duration(seconds: 15));

      expect(result['status'], equals('completed'));
      final screenshotPath = result['screenshotPath'] as String? ?? '';
      expect(screenshotPath, isNotEmpty);
      expect(await File(screenshotPath).exists(), isTrue);
      expect(_screenshotFileNameForPath(screenshotPath), matches(RegExp(r'^\d{8}_\d{6}_wox_snapshots(?:_\d+)?\.png$')));
      expect((await File(screenshotPath).readAsBytes()).length, greaterThan(2048));

      // Bug fix: confirming a screenshot intentionally hides Wox. Pumping after
      // the hide can wait forever for macOS vsync in full-suite smoke runs, so
      // assert the hidden postcondition without driving another frame.
      await waitForWindowVisibility(tester, false, timeout: const Duration(seconds: 15));
      expect(screenshotController.isSessionActive.value, isFalse);
    });

    testWidgets('T11-02: Screenshot cancel restores launcher without exporting', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(() async => [await _buildSnapshot('display-c', const Color(0xFF144552), const ScreenshotRect(x: 0, y: 0, width: 320, height: 240))]),
      );

      Map<String, dynamic>? sessionResult;
      Object? sessionError;
      final sessionFuture = Get.find<WoxScreenshotController>()
          .startCaptureSession('smoke-cancel', _defaultRequest())
          .then((value) {
            final json = value.toJson();
            sessionResult = json;
            return json;
          })
          .catchError((error) {
            sessionError = error;
            throw error;
          });
      await pumpUntil(tester, () => find.byKey(screenshotCancelKey).evaluate().isNotEmpty || sessionResult != null || sessionError != null, timeout: const Duration(seconds: 15));

      expect(sessionError, isNull);
      expect(sessionResult, isNull, reason: 'Screenshot session completed before the cancel path became interactive.');
      expect(find.byType(WoxScreenshotView), findsOneWidget);
      // The integration-test render surface can stay smaller than the virtual desktop window, so
      // the toolbar may render partially outside the hit-testable root even when the cancel action
      // is visible. Trigger the controller directly so the smoke test verifies the cancel path
      // instead of the test harness hit-testing limits.
      await Get.find<WoxScreenshotController>().cancelSession('smoke-cancel-complete');
      await tester.pump(const Duration(milliseconds: 250));
      final result = await sessionFuture;

      expect(result['status'], equals('cancelled'));
      await pumpUntil(tester, () => find.byType(WoxLauncherView).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));
      expect(Get.find<WoxScreenshotController>().isSessionActive.value, isFalse);
    });

    testWidgets('T11-02P: Screenshot pin exports a PNG and marks the result for screen pinning', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(() async => [await _buildSnapshot('display-pin', const Color(0xFF31572C), const ScreenshotRect(x: 0, y: 0, width: 420, height: 280))]),
      );

      final sessionFuture = screenshotController.startCaptureSession('smoke-pin', _defaultRequest());
      await pumpUntil(tester, () => find.byKey(screenshotCanvasKey).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));

      screenshotController.updateSelection(const Rect.fromLTWH(40, 32, 180, 120));
      await tester.pumpAndSettle();

      // Pin uses the same export path as confirm but returns an explicit result flag so Go can decide
      // to display the saved PNG through the native overlay instead of treating it as a clipboard-only capture.
      await screenshotController.pinSelection('smoke-pin-confirm');
      final result = (await sessionFuture.timeout(const Duration(seconds: 15))).toJson();

      expect(result['status'], equals('completed'));
      expect(result['pinToScreen'], isTrue);
      final screenshotPath = result['screenshotPath'] as String? ?? '';
      expect(screenshotPath, isNotEmpty);
      expect(await File(screenshotPath).exists(), isTrue);
      final codec = await ui.instantiateImageCodec(await File(screenshotPath).readAsBytes());
      final frame = await codec.getNextFrame();
      expect(frame.image.width, equals(180));
      expect(frame.image.height, equals(120));
      frame.image.dispose();

      await waitForWindowVisibility(tester, false, timeout: const Duration(seconds: 15));
      expect(screenshotController.isSessionActive.value, isFalse);
    });

    testWidgets('T11-02A: Plugin-triggered screenshot shows the caller icon before the tools', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      final callerIcon = WoxImage(imageType: 'emoji', imageData: 'P');
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(() async => [await _buildSnapshot('display-plugin-icon', const Color(0xFF345995), const ScreenshotRect(x: 0, y: 0, width: 360, height: 240))]),
      );

      final request = CaptureScreenshotRequest.fromJson({
        'sessionId': 'smoke-plugin-icon',
        'trigger': 'plugin',
        'scope': 'all_displays',
        'output': 'clipboard',
        'tools': ['rect', 'ellipse', 'arrow', 'text'],
        'exportFilePath': _smokeScreenshotExportPath(),
        'callerIcon': callerIcon.toJson(),
      });
      final sessionFuture = screenshotController.startCaptureSession('smoke-plugin-icon', request);
      await pumpUntil(tester, () => find.byKey(screenshotToolbarKey).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));

      final callerIconFinder = find.byWidgetPredicate((widget) => widget is WoxImageView && widget.woxImage == callerIcon);
      expect(callerIconFinder, findsOneWidget);
      expect(tester.getRect(callerIconFinder).left, lessThan(tester.getRect(find.byKey(screenshotToolSelectKey)).left));

      await screenshotController.cancelSession('smoke-plugin-icon-cancel');
      await tester.pump(const Duration(milliseconds: 250));
      final result = (await sessionFuture).toJson();

      expect(result['status'], equals('cancelled'));
    });

    testWidgets('T11-03: Screenshot bridge failure restores launcher and returns failed status', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(() async {
          throw StateError('permission denied');
        }),
      );

      final result = (await Get.find<WoxScreenshotController>().startCaptureSession('smoke-failed', _defaultRequest())).toJson();
      await tester.pump(const Duration(milliseconds: 250));

      expect(result['status'], equals('failed'));
      expect((result['errorMessage'] as String?) ?? '', contains('permission denied'));
      expect(Get.find<WoxScreenshotController>().isSessionActive.value, isFalse);
      expect(find.byType(WoxLauncherView), findsOneWidget);
    });

    testWidgets('T11-04: Screenshot annotation editing updates existing text and keeps the edit bar outside the selection', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(() async => [await _buildSnapshot('display-d', const Color(0xFF23395B), const ScreenshotRect(x: 0, y: 0, width: 800, height: 600))]),
      );

      final sessionFuture = screenshotController.startCaptureSession('smoke-edit', _defaultRequest());
      await pumpUntil(tester, () => find.byKey(screenshotCanvasKey).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));

      screenshotController.updateSelection(const Rect.fromLTWH(560, 120, 200, 220));
      screenshotController.annotations.add(
        ScreenshotAnnotation(id: 'text-a', type: ScreenshotAnnotationType.text, start: const Offset(590, 180), text: 'Before', color: Color(0xFFFF5B36), fontSize: 20),
      );
      screenshotController.selectAnnotation('text-a');
      await tester.pumpAndSettle();

      final editBarRect = tester.getRect(find.byKey(screenshotEditBarKey));
      expect(editBarRect.right, lessThan(560));

      screenshotController.updateSelectedAnnotationColor(const Color(0xFF4DA3FF));
      screenshotController.updateSelectedTextFontSize(6);
      screenshotController.startTextDraft(const Offset(590, 180), annotationId: 'text-a', initialText: 'Before', fontSize: 26, color: const Color(0xFF4DA3FF));
      screenshotController.textDraftController.text = 'After';
      screenshotController.commitTextDraft();
      await tester.pumpAndSettle();

      final editedAnnotation = screenshotController.annotations.single;
      expect(editedAnnotation.text, equals('After'));
      expect(editedAnnotation.fontSize, equals(26));
      expect(editedAnnotation.color, equals(const Color(0xFF4DA3FF)));

      // Keep the text-edit smoke focused on export behavior because toolbar hit tests
      // are environment-sensitive on macOS after screenshot window handoffs.
      await screenshotController.confirmSelection('smoke-edit-confirm');
      final result = (await sessionFuture.timeout(const Duration(seconds: 15))).toJson();

      expect(result['status'], equals('completed'));
      // Screenshot confirmation leaves the launcher hidden; checking visibility
      // avoids a hidden-window pump that can deadlock on macOS.
      await waitForWindowVisibility(tester, false, timeout: const Duration(seconds: 15));
    });

    testWidgets('T11-05: Screenshot export uses workspaceScale to map the selected area back to native pixels', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      ScreenshotRect? capturedNativeWorkspaceBounds;
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(
          () async => [await _buildSnapshot('display-scaled', const Color(0xFF1F6FEB), const ScreenshotRect(x: 0, y: 0, width: 400, height: 200))],
          presentation: (nativeWorkspaceBounds) async {
            // Flutter test expectations cannot run inside this bridge callback
            // because the screenshot startup is already guarded by testWidgets.
            // Capture the value and assert from the test body after startup.
            capturedNativeWorkspaceBounds = nativeWorkspaceBounds;
            return const ScreenshotWorkspacePresentation(workspaceBounds: ScreenshotRect(x: 0, y: 0, width: 200, height: 100), workspaceScale: 2, presentedByPlatform: false);
          },
        ),
      );

      final sessionFuture = screenshotController.startCaptureSession('smoke-scaled-export', _defaultRequest());
      await pumpUntil(tester, () => find.byKey(screenshotCanvasKey).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));

      expect(capturedNativeWorkspaceBounds, const ScreenshotRect(x: 0, y: 0, width: 400, height: 200));
      expect(screenshotController.virtualBoundsRect, const Rect.fromLTWH(0, 0, 200, 100));

      screenshotController.updateSelection(const Rect.fromLTWH(20, 10, 150, 80));
      await tester.pumpAndSettle();
      // The scaled workspace is intentionally only 200x100 logical pixels, so
      // the floating toolbar can sit outside the integration-test root. Confirm
      // through the controller to keep this test focused on native-pixel export
      // mapping instead of toolbar hit-test geometry.
      await screenshotController.confirmSelection('smoke-scaled-export-confirm');

      final result = (await sessionFuture.timeout(const Duration(seconds: 15))).toJson();
      final pngBytes = await File(result['screenshotPath'] as String).readAsBytes();
      final codec = await ui.instantiateImageCodec(pngBytes);
      final frame = await codec.getNextFrame();

      expect(result['status'], equals('completed'));
      expect(frame.image.width, equals(300));
      expect(frame.image.height, equals(160));

      frame.image.dispose();
      // Confirmation exports and hides Wox, so do not pump on the hidden macOS
      // window while checking the final state.
      await waitForWindowVisibility(tester, false, timeout: const Duration(seconds: 15));
    });

    testWidgets('T11-06: Native screenshot presentation debug state toggles during the session', (tester) async {
      if (!Platform.isMacOS && !Platform.isWindows) {
        return;
      }

      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      final bridge = _FakeScreenshotBridge(
        () async => [await _buildSnapshot('display-native-debug', const Color(0xFF23395B), const ScreenshotRect(x: 0, y: 0, width: 800, height: 600))],
        delegateNativePresentation: true,
      );
      ScreenshotPlatformBridge.setInstanceForTest(bridge);

      final sessionFuture = screenshotController.startCaptureSession('smoke-native-debug', _defaultRequest());
      await pumpUntil(tester, () => find.byKey(screenshotCanvasKey).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));

      final activeDebugState = await bridge.debugCaptureWorkspaceState();
      expect(activeDebugState['isCapturePresentationActive'], isTrue);
      expect((activeDebugState['workspaceScale'] as num?)?.toDouble() ?? 0, greaterThan(0));

      await screenshotController.cancelSession('smoke-native-debug-cancel');
      await tester.pump(const Duration(milliseconds: 250));
      final result = (await sessionFuture).toJson();
      final restoredDebugState = await bridge.debugCaptureWorkspaceState();

      expect(result['status'], equals('cancelled'));
      expect(restoredDebugState['isCapturePresentationActive'], isFalse);
      await pumpUntil(tester, () => find.byType(WoxLauncherView).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));
    });

    testWidgets('T11-07: Native macOS multi-display selection keeps shaded context around the selected area', (tester) async {
      if (!Platform.isMacOS) {
        return;
      }

      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      ScreenshotRect? capturedNativeSelectionWorkspaceBounds;
      ScreenshotRect? capturedPresentationWorkspaceBounds;
      var dismissCalls = 0;
      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(
          () async => [
            await _buildSnapshot('display-left', const Color(0xFF0B7285), const ScreenshotRect(x: 0, y: 0, width: 200, height: 120)),
            await _buildSnapshot('display-right', const Color(0xFFE8590C), const ScreenshotRect(x: 200, y: 0, width: 200, height: 120)),
          ],
          nativeSelection: (nativeWorkspaceBounds) async {
            // Bug fix: this callback runs while pumpUntil is driving a guarded
            // tester.pump. Store the contract value and assert from the test
            // body instead of calling Flutter test expectations inside the bridge.
            capturedNativeSelectionWorkspaceBounds = nativeWorkspaceBounds;
            return const ScreenshotNativeSelectionResult(
              wasHandled: true,
              selection: ScreenshotRect(x: 220, y: 20, width: 100, height: 70),
              editorVisibleBounds: ScreenshotRect(x: 40, y: 0, width: 320, height: 220),
            );
          },
          presentation: (nativeWorkspaceBounds) async {
            // Keep bridge callbacks free of guarded test APIs for the same
            // reason as nativeSelection above.
            capturedPresentationWorkspaceBounds = nativeWorkspaceBounds;
            return const ScreenshotWorkspacePresentation(workspaceBounds: ScreenshotRect(x: 0, y: 0, width: 400, height: 120), workspaceScale: 1, presentedByPlatform: false);
          },
          dismissNativeOverlays: () async {
            dismissCalls += 1;
          },
        ),
      );

      final sessionFuture = screenshotController.startCaptureSession('smoke-native-multi-display', _defaultRequest());
      await pumpUntil(tester, () => find.byKey(screenshotCanvasKey).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));
      // Bug fix: the native macOS handoff sets the annotating stage before the
      // startup coroutine finishes its end-of-frame overlay dismissal. Confirming
      // before that point can complete the session while startCaptureSession is
      // still waiting on the startup frame, leaving the outer future unresolved.
      await pumpUntil(tester, () => dismissCalls == 1, timeout: const Duration(seconds: 15));

      expect(capturedNativeSelectionWorkspaceBounds, const ScreenshotRect(x: 0, y: 0, width: 400, height: 120));
      // macOS native selection starts on the full virtual desktop, then prepares
      // Flutter annotation on the display that contains the selection center.
      // The selected rect below sits on the right display, so presentation is
      // requested for that display while the fake bridge still returns a larger
      // editor workspace to keep the surrounding shaded context testable.
      expect(capturedPresentationWorkspaceBounds, const ScreenshotRect(x: 200, y: 0, width: 200, height: 120));
      expect(screenshotController.stage.value, ScreenshotSessionStage.annotating);
      expect(screenshotController.displaySnapshots, hasLength(2));
      expect(screenshotController.virtualBoundsRect, const Rect.fromLTWH(0, 0, 400, 120));
      expect(screenshotController.selectionRect, const Rect.fromLTWH(20, 20, 100, 70));
      expect(screenshotController.virtualBoundsRect.width, greaterThan(screenshotController.selectionRect!.width));
      expect(screenshotController.virtualBoundsRect.height, greaterThan(screenshotController.selectionRect!.height));

      // Native macOS presentation can leave the toolbar outside the integration-test
      // hit-test root. Confirm directly so this test covers rendering and export.
      await screenshotController.confirmSelection('smoke-native-multi-display-confirm');

      final result = (await sessionFuture.timeout(const Duration(seconds: 15))).toJson();
      final pngBytes = await File(result['screenshotPath'] as String).readAsBytes();
      final codec = await ui.instantiateImageCodec(pngBytes);
      final frame = await codec.getNextFrame();

      expect(result['status'], equals('completed'));
      expect(frame.image.width, equals(100));
      expect(frame.image.height, equals(70));

      frame.image.dispose();
      // Native confirmation follows the same hidden-window contract as the
      // Flutter toolbar path, so avoid pumping after the export hides Wox.
      await waitForWindowVisibility(tester, false, timeout: const Duration(seconds: 15));
    });

    testWidgets('T11-08: Native selection overlays are dismissed after Flutter renders its first frame', (tester) async {
      if (!Platform.isMacOS) {
        return;
      }

      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final screenshotController = Get.find<WoxScreenshotController>();
      var dismissCalls = 0;

      ScreenshotPlatformBridge.setInstanceForTest(
        _FakeScreenshotBridge(
          () async => [
            await _buildSnapshot('display-left', const Color(0xFF135D66), const ScreenshotRect(x: 0, y: 0, width: 200, height: 120)),
            await _buildSnapshot('display-right', const Color(0xFF6A4C93), const ScreenshotRect(x: 200, y: 0, width: 200, height: 120)),
          ],
          nativeSelection:
              (_) async => const ScreenshotNativeSelectionResult(
                wasHandled: true,
                selection: ScreenshotRect(x: 80, y: 16, width: 140, height: 72),
                editorVisibleBounds: ScreenshotRect(x: 0, y: 0, width: 400, height: 120),
              ),
          presentation:
              (nativeWorkspaceBounds) async =>
                  const ScreenshotWorkspacePresentation(workspaceBounds: ScreenshotRect(x: 0, y: 0, width: 400, height: 120), workspaceScale: 1, presentedByPlatform: false),
          dismissNativeOverlays: () async {
            dismissCalls += 1;
          },
        ),
      );

      // The native overlays should be dismissed automatically after Flutter paints
      // its first frame, not kept alive as a passive backdrop.
      final sessionFuture = screenshotController.startCaptureSession('smoke-native-dismiss-order', _defaultRequest());
      await pumpUntil(tester, () => screenshotController.stage.value == ScreenshotSessionStage.annotating, timeout: const Duration(seconds: 15));

      // Bug fix: stage switches to annotating before the native overlay dismiss
      // runs at the end of the next frame. Wait for the actual dismiss callback
      // instead of assuming both state transitions happen in the same pump.
      await pumpUntil(tester, () => dismissCalls == 1, timeout: const Duration(seconds: 15));
      // Native overlays should have been dismissed during the handoff after the first frame.
      expect(dismissCalls, equals(1));

      await screenshotController.cancelSession('smoke-native-dismiss-order-cancel');
      await tester.pump(const Duration(milliseconds: 250));
      final result = (await sessionFuture).toJson();

      expect(result['status'], equals('cancelled'));
      // dismissNativeSelectionOverlays is also called in _restoreWindowState, so total is 2.
      expect(dismissCalls, equals(2));
      await pumpUntil(tester, () => find.byType(WoxLauncherView).evaluate().isNotEmpty, timeout: const Duration(seconds: 15));
    });
  });
}

CaptureScreenshotRequest _defaultRequest() {
  return CaptureScreenshotRequest(
    sessionId: 'smoke-session',
    trigger: 'plugin',
    scope: 'all_displays',
    output: 'clipboard',
    tools: const ['rect', 'ellipse', 'arrow', 'text'],
    hideAnnotationToolbar: false,
    autoConfirm: false,
    // Direct controller smoke tests bypass the Go UI layer that normally
    // reserves timestamped screenshot paths, so mirror that production naming
    // contract here instead of exporting every test to one fixed temp file.
    exportFilePath: _smokeScreenshotExportPath(),
  );
}

String _smokeScreenshotExportPath() {
  final now = DateTime.now();
  final timestamp =
      '${now.year.toString().padLeft(4, '0')}${now.month.toString().padLeft(2, '0')}${now.day.toString().padLeft(2, '0')}_'
      '${now.hour.toString().padLeft(2, '0')}${now.minute.toString().padLeft(2, '0')}${now.second.toString().padLeft(2, '0')}';
  return '${Directory.systemTemp.path}${Platform.pathSeparator}${timestamp}_wox_snapshots_${now.microsecondsSinceEpoch}.png';
}

Future<DisplaySnapshot> _buildSnapshot(String id, Color color, ScreenshotRect logicalBounds) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  final rect = Rect.fromLTWH(0, 0, logicalBounds.width, logicalBounds.height);
  canvas.drawRect(rect, Paint()..color = color);
  canvas.drawRect(rect.deflate(18), Paint()..color = color.withValues(alpha: 0.78));

  final picture = recorder.endRecording();
  final image = await picture.toImage(logicalBounds.width.toInt(), logicalBounds.height.toInt());
  final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
  return DisplaySnapshot(
    displayId: id,
    logicalBounds: logicalBounds,
    pixelBounds: logicalBounds,
    scale: 1,
    rotation: 0,
    imageBytesBase64: base64Encode(bytes!.buffer.asUint8List()),
  );
}

String _screenshotFileNameForPath(String screenshotPath) {
  final normalized = screenshotPath.replaceAll('\\', '/');
  final slashIndex = normalized.lastIndexOf('/');
  if (slashIndex == -1) {
    return normalized;
  }

  return normalized.substring(slashIndex + 1);
}
