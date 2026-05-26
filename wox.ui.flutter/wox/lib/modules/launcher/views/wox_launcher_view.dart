import 'dart:async';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/refinement/wox_query_refinement_bar_view.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/modules/launcher/views/wox_query_box_view.dart';
import 'package:wox/modules/launcher/views/wox_query_result_view.dart';
import 'package:wox/modules/launcher/views/wox_query_toolbar_view.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxLauncherView extends GetView<WoxLauncherController> {
  const WoxLauncherView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final theme = WoxThemeUtil.instance.currentTheme.value;
      final isQueryBoxVisible = controller.isQueryBoxVisible.value;
      final isToolbarShowedWithoutResults = controller.isToolbarShowedWithoutResults;
      final isPreviewOnlyLayout = controller.isPreviewOnlyLayout;
      final interfaceMetrics = WoxInterfaceSizeUtil.instance.metrics.value;
      final queryBoxView = const WoxQueryBoxView();
      final refinementBarView = const WoxQueryRefinementBarView();
      final resultView = const WoxQueryResultView();
      final topPadding = isQueryBoxVisible ? theme.appPaddingTop.toDouble() : 0.0;

      double bottomPadding = theme.appPaddingBottom.toDouble();
      if (isQueryBoxVisible && isToolbarShowedWithoutResults) {
        bottomPadding = 0.0;
      }

      final contentPadding =
          isPreviewOnlyLayout
              ? EdgeInsets.zero
              : EdgeInsets.only(top: topPadding, right: theme.appPaddingRight.toDouble(), bottom: bottomPadding, left: theme.appPaddingLeft.toDouble());

      Widget content = resultView;
      if (isQueryBoxVisible) {
        final queryBoxHeight = controller.getQueryBoxInputHeight();
        final refinementBarHeight = controller.getQueryRefinementBarHeight();
        final topResultInset = queryBoxHeight + (controller.isQueryBoxAtBottom.value ? 0.0 : refinementBarHeight);
        final bottomResultInset = queryBoxHeight + (controller.isQueryBoxAtBottom.value ? refinementBarHeight : 0.0);
        content = Stack(
          children: [
            if (controller.isQueryBoxAtBottom.value)
              Positioned.fill(bottom: bottomResultInset, child: const WoxQueryResultView())
            else
              Positioned.fill(top: topResultInset, child: const WoxQueryResultView()),
            Positioned(
              top: controller.isQueryBoxAtBottom.value ? null : 0,
              bottom: controller.isQueryBoxAtBottom.value ? 0 : null,
              left: 0,
              right: 0,
              height: queryBoxHeight,
              child: queryBoxView,
            ),
            if (controller.shouldShowQueryRefinements)
              Positioned(
                top: controller.isQueryBoxAtBottom.value ? null : queryBoxHeight,
                bottom: controller.isQueryBoxAtBottom.value ? queryBoxHeight : null,
                left: 0,
                right: 0,
                height: refinementBarHeight,
                child: refinementBarView,
              ),
          ],
        );
      }

      return WoxPlatformFocus(
        focusNode: controller.launcherFocusNode,
        onKeyEvent: (node, event) {
          if (event is! KeyDownEvent || event.logicalKey != LogicalKeyboardKey.escape) {
            return KeyEventResult.ignored;
          }

          final traceId = const UuidV4().generate();
          if (!isQueryBoxVisible) {
            unawaited(controller.hideApp(traceId));
            return KeyEventResult.handled;
          }

          if (controller.queryBoxFocusNode.hasFocus) {
            return KeyEventResult.ignored;
          }

          controller.focusQueryBox();
          return KeyEventResult.handled;
        },
        child: Scaffold(
          backgroundColor: safeFromCssColor(theme.appBackgroundColor),
          body: DropTarget(
            onDragDone: (DropDoneDetails details) {
              controller.handleDropFiles(details);
            },
            child: Column(
              children: [
                if (!isQueryBoxVisible) const Offstage(offstage: true, child: WoxQueryBoxView()),
                Flexible(
                  fit: isQueryBoxVisible ? FlexFit.tight : FlexFit.loose,
                  child: Padding(
                    padding: contentPadding,
                    child: LayoutBuilder(
                      builder: (context, constraints) {
                        return SizedBox(width: constraints.maxWidth, height: constraints.maxHeight, child: content);
                      },
                    ),
                  ),
                ),
                if (controller.isToolbarVisible)
                  SizedBox(
                    // The parent reserves toolbar height before the toolbar child paints,
                    // so it must observe density metrics directly instead of relying on
                    // the old fixed 40px wrapper.
                    height: interfaceMetrics.toolbarHeight,
                    child: const WoxQueryToolbarView(),
                  ),
              ],
            ),
          ),
        ),
      );
    });
  }
}
