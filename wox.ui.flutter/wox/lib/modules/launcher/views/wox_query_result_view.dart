import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_form_action_view.dart';
import 'package:wox/components/wox_grid_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  Widget getActionPanelView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action panel view container");

    return Obx(
      () =>
          controller.isShowActionPanel.value
              ? Positioned(
                right: WoxInterfaceSizeUtil.instance.current.actionPanelOffsetRight,
                bottom: WoxInterfaceSizeUtil.instance.current.actionPanelOffsetBottom,
                child: Container(
                  padding: EdgeInsets.only(
                    top: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingTop.toDouble()),
                    bottom: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingBottom.toDouble()),
                    left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingLeft.toDouble()),
                    right: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingRight.toDouble()),
                  ),
                  decoration: BoxDecoration(
                    color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionContainerBackgroundColor),
                    borderRadius: BorderRadius.circular(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxBorderRadius.toDouble()),
                    boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.1), spreadRadius: 2, blurRadius: 8, offset: const Offset(0, 3))],
                  ),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: WoxInterfaceSizeUtil.instance.current.actionPanelMaxWidth),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.start,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          controller.tr("ui_actions"),
                          style: TextStyle(
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionContainerHeaderFontColor),
                            fontSize: WoxInterfaceSizeUtil.instance.current.actionHeaderFontSize,
                          ),
                        ),
                        const Divider(),
                        WoxListView<WoxResultAction>(
                          controller: controller.actionListViewController,
                          // Action panel capacity follows density so compact,
                          // normal, and comfortable modes show the same number
                          // of action rows without reusing the old 40px math.
                          maxHeight: WoxInterfaceSizeUtil.instance.current.actionItemBaseHeight * 8,
                          listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code,
                          onFilteHotkeyPressed: (traceId, hotkey) {
                            if (controller.isActionHotkey(hotkey)) {
                              controller.hideActionPanel(traceId);
                              return true;
                            }
                            return false;
                          },
                        ),
                      ],
                    ),
                  ),
                ),
              )
              : const SizedBox(),
    );
  }

  Widget getActionFormView() {
    return Obx(() {
      final action = controller.activeFormAction.value;
      if (!controller.isShowFormActionPanel.value || action == null) {
        return const SizedBox();
      }

      if (action.form.isEmpty) {
        return const SizedBox();
      }

      if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action form view container");

      return Positioned(
        right: WoxInterfaceSizeUtil.instance.current.actionPanelOffsetRight,
        bottom: WoxInterfaceSizeUtil.instance.current.actionPanelOffsetBottom,
        child: Container(
          padding: EdgeInsets.only(
            top: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingTop.toDouble()),
            bottom: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingBottom.toDouble()),
            left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingLeft.toDouble()),
            right: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingRight.toDouble()),
          ),
          decoration: BoxDecoration(
            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionContainerBackgroundColor),
            borderRadius: BorderRadius.circular(
              WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxBorderRadius.toDouble()),
            ),
            boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.1), spreadRadius: 2, blurRadius: 8, offset: const Offset(0, 3))],
          ),
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: WoxInterfaceSizeUtil.instance.current.actionFormMaxWidth, maxHeight: WoxInterfaceSizeUtil.instance.current.actionFormMaxHeight),
            child: WoxFormActionView(
              action: action,
              initialValues: controller.formActionValues,
              translate: controller.tr,
              onSave: (values) => controller.submitFormAction(const UuidV4().generate(), values),
              onCancel: () => controller.hideFormActionPanel(const UuidV4().generate(), reason: "form cancel button"),
            ),
          ),
        ),
      );
    });
  }

  Widget getResultContainer() {
    return Container(
      padding: EdgeInsets.only(
        top: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop.toDouble()),
        right: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingRight.toDouble()),
        bottom: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom.toDouble()),
        left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingLeft.toDouble()),
      ),
      child: Obx(() {
        if (controller.isGridLayout.value) {
          final gridLayoutParams = controller.gridLayoutParams.value;
          return WoxGridView(
            controller: controller.resultGridViewController,
            gridLayoutParams: gridLayoutParams,
            maxHeight: controller.getMaxResultListViewHeight(),
            onItemTapped: () {
              controller.hideActionPanel(const UuidV4().generate());
              controller.hideFormActionPanel(const UuidV4().generate(), reason: "grid result item tapped");
            },
            onItemSecondaryTapped: (traceId, item) {
              controller.openActionPanelForActiveResult(traceId);
            },
            onItemIconLoaded: controller.updateLazyLoadedResultIcon,
            onRowHeightChanged: () => controller.resizeHeight(traceId: const UuidV4().generate(), reason: "grid row height changed"),
          );
        }

        return WoxListView<WoxQueryResult>(
          controller: controller.resultListViewController,
          listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code,
          showFilter: false,
          maxHeight: controller.getMaxResultListViewHeight(),
          onItemTapped: () {
            controller.hideActionPanel(const UuidV4().generate());
            controller.hideFormActionPanel(const UuidV4().generate(), reason: "list result item tapped");
          },
          onItemSecondaryTapped: (traceId, item) {
            controller.openActionPanelForActiveResult(traceId);
          },
          onItemIconLoaded: controller.updateLazyLoadedResultIcon,
        );
      }),
    );
  }

  Widget getResultView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: result view container");

    return Obx(
      () =>
          controller.resultListViewController.items.isNotEmpty
              ? controller.isShowPreviewPanel.value
                  ? controller.resultPreviewRatio.value == 0
                      ? SizedBox()
                      : Flexible(flex: (controller.resultPreviewRatio.value * 100).toInt(), child: getResultContainer())
                  : Expanded(child: getResultContainer())
              : const SizedBox(),
    );
  }

  Widget getPreviewView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: preview view container");

    return Obx(
      () =>
          controller.isShowPreviewPanel.value
              ? Flexible(
                flex: (100 - controller.resultPreviewRatio.value * 100).toInt(),
                child:
                    controller.currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code
                        ? FutureBuilder(
                          future: controller.currentPreview.value.unWrap(const UuidV4().generate()),
                          builder: (context, snapshot) {
                            if (snapshot.hasData) {
                              return WoxPreviewView(woxPreview: snapshot.data!, woxTheme: WoxThemeUtil.instance.currentTheme.value);
                            } else if (snapshot.hasError) {
                              return Text("${snapshot.error}");
                            } else {
                              return const SizedBox();
                            }
                          },
                        )
                        : WoxPreviewView(woxPreview: controller.currentPreview.value, woxTheme: WoxThemeUtil.instance.currentTheme.value),
              )
              : const SizedBox(),
    );
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final boundedHeight = constraints.hasBoundedHeight ? constraints.maxHeight : null;
        return SizedBox(
          height: boundedHeight,
          child: ConstrainedBox(
            constraints: BoxConstraints(maxHeight: controller.getMaxResultContainerHeight()),
            child: Obx(
              () => Stack(
                fit: controller.isShowActionPanel.value || controller.isShowPreviewPanel.value ? StackFit.expand : StackFit.loose,
                children: [
                  Row(crossAxisAlignment: CrossAxisAlignment.start, children: [getResultView(), getPreviewView()]),
                  getActionPanelView(),
                  getActionFormView(),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
