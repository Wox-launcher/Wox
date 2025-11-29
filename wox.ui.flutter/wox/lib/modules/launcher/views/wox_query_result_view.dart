import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_grid_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  Widget getActionPanelView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action panel view container");

    return Obx(
      () => controller.isShowActionPanel.value
          ? Positioned(
              right: 10,
              bottom: 10,
              child: Container(
                padding: EdgeInsets.only(
                  top: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingTop.toDouble(),
                  bottom: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingBottom.toDouble(),
                  left: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingLeft.toDouble(),
                  right: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingRight.toDouble(),
                ),
                decoration: BoxDecoration(
                  color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionContainerBackgroundColor),
                  borderRadius: BorderRadius.circular(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxBorderRadius.toDouble()),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withValues(alpha: 0.1),
                      spreadRadius: 2,
                      blurRadius: 8,
                      offset: const Offset(0, 3),
                    ),
                  ],
                ),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 320),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.start,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(controller.tr("ui_actions"),
                          style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionContainerHeaderFontColor), fontSize: 16.0)),
                      const Divider(),
                      WoxListView<WoxResultAction>(
                        controller: controller.actionListViewController,
                        maxHeight: 400,
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
    return Obx(
      () {
        final action = controller.activeFormAction.value;
        if (!controller.isShowFormActionPanel.value || action == null) {
          return const SizedBox();
        }

        if (action.form.isEmpty) {
          return const SizedBox();
        }

        if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action form view container");

        return Positioned(
          right: 10,
          bottom: 10,
          child: Container(
            padding: EdgeInsets.only(
              top: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingTop.toDouble(),
              bottom: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingBottom.toDouble(),
              left: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingLeft.toDouble(),
              right: WoxThemeUtil.instance.currentTheme.value.actionContainerPaddingRight.toDouble(),
            ),
            decoration: BoxDecoration(
              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionContainerBackgroundColor),
              borderRadius: BorderRadius.circular(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxBorderRadius.toDouble()),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.1),
                  spreadRadius: 2,
                  blurRadius: 8,
                  offset: const Offset(0, 3),
                ),
              ],
            ),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 360, maxHeight: 400),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Flexible(
                        child: Text(
                          action.name,
                          style: TextStyle(color: getThemeTextColor(), fontSize: 16, fontWeight: FontWeight.w600),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      WoxButton.text(
                        text: controller.tr("ui_cancel"),
                        icon: Icon(Icons.close, size: 14, color: getThemeTextColor()),
                        onPressed: () => controller.hideFormActionPanel(const UuidV4().generate()),
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      ),
                    ],
                  ),
                  const Divider(),
                  ...action.form.map((item) {
                    switch (item.type) {
                      case "textbox":
                        final textbox = item.value as PluginSettingValueTextBox;
                        return WoxSettingPluginTextBox(
                          value: controller.formActionValues[textbox.key] ?? textbox.defaultValue,
                          item: textbox,
                          onUpdate: (key, value) {
                            controller.formActionValues[key] = value;
                          },
                        );
                      case "checkbox":
                        final checkbox = item.value as PluginSettingValueCheckBox;
                        return WoxSettingPluginCheckbox(
                          value: controller.formActionValues[checkbox.key] ?? checkbox.defaultValue,
                          item: checkbox,
                          onUpdate: (key, value) {
                            controller.formActionValues[key] = value;
                          },
                        );
                      case "select":
                        final select = item.value as PluginSettingValueSelect;
                        return WoxSettingPluginSelect(
                          value: controller.formActionValues[select.key] ?? select.defaultValue,
                          item: select,
                          onUpdate: (key, value) {
                            controller.formActionValues[key] = value;
                          },
                        );
                      case "selectAIModel":
                        final select = item.value as PluginSettingValueSelectAIModel;
                        return WoxSettingPluginSelectAIModel(
                          value: controller.formActionValues[select.key] ?? select.defaultValue,
                          item: select,
                          onUpdate: (key, value) {
                            controller.formActionValues[key] = value;
                          },
                        );
                      case "table":
                        final table = item.value as PluginSettingValueTable;
                        return WoxSettingPluginTable(
                          value: controller.formActionValues[table.key] ?? table.defaultValue,
                          item: table,
                          onUpdate: (key, value) {
                            controller.formActionValues[key] = value;
                          },
                        );
                      case "head":
                        return WoxSettingPluginHead(
                          value: "",
                          item: item.value as PluginSettingValueHead,
                          onUpdate: (_, __) {},
                        );
                      case "label":
                        return WoxSettingPluginLabel(
                          value: "",
                          item: item.value as PluginSettingValueLabel,
                          onUpdate: (_, __) {},
                        );
                      case "newline":
                        return WoxSettingPluginNewLine(
                          value: "",
                          item: item.value as PluginSettingValueNewLine,
                          onUpdate: (_, __) {},
                        );
                      default:
                        return Padding(
                          padding: const EdgeInsets.only(bottom: 8),
                          child: Text(
                            controller.tr("ui_not_supported_field"),
                            style: TextStyle(color: getThemeTextColor(), fontSize: 12),
                          ),
                        );
                    }
                  }).toList(),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.end,
                    children: [
                      TextButton(
                        onPressed: () => controller.hideFormActionPanel(const UuidV4().generate()),
                        child: Text(controller.tr("ui_cancel")),
                      ),
                      const SizedBox(width: 8),
                      ElevatedButton(
                        onPressed: () => controller.submitFormAction(const UuidV4().generate()),
                        child: Text(controller.tr("ui_save")),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget getResultContainer() {
    return Container(
      padding: EdgeInsets.only(
        top: WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop.toDouble(),
        right: WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingRight.toDouble(),
        bottom: WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom.toDouble(),
        left: WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingLeft.toDouble(),
      ),
      child: Obx(() {
        if (controller.isGridLayout.value) {
          return WoxGridView(
            controller: controller.resultGridViewController,
            maxHeight: WoxThemeUtil.instance.getMaxResultListViewHeight(),
            onItemTapped: () {
              controller.hideActionPanel(const UuidV4().generate());
            },
          );
        }

        return WoxListView<WoxQueryResult>(
          controller: controller.resultListViewController,
          listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code,
          showFilter: false,
          maxHeight: WoxThemeUtil.instance.getMaxResultListViewHeight(),
          onItemTapped: () {
            controller.hideActionPanel(const UuidV4().generate());
          },
        );
      }),
    );
  }

  Widget getResultView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: result view container");

    return Obx(
      () => controller.resultListViewController.items.isNotEmpty
          ? controller.isShowPreviewPanel.value
              ? Flexible(
                  flex: (controller.resultPreviewRatio.value * 100).toInt(),
                  child: getResultContainer(),
                )
              : Expanded(
                  child: getResultContainer(),
                )
          : const SizedBox(),
    );
  }

  Widget getPreviewView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: preview view container");

    return Obx(
      () => controller.isShowPreviewPanel.value
          ? Flexible(
              flex: (100 - controller.resultPreviewRatio.value * 100).toInt(),
              child: controller.currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code
                  ? FutureBuilder(
                      future: controller.currentPreview.value.unWrap(),
                      builder: (context, snapshot) {
                        if (snapshot.hasData) {
                          return WoxPreviewView(
                            woxPreview: snapshot.data!,
                            woxTheme: WoxThemeUtil.instance.currentTheme.value,
                          );
                        } else if (snapshot.hasError) {
                          return Text("${snapshot.error}");
                        } else {
                          return const SizedBox();
                        }
                      },
                    )
                  : WoxPreviewView(
                      woxPreview: controller.currentPreview.value,
                      woxTheme: WoxThemeUtil.instance.currentTheme.value,
                    ),
            )
          : const SizedBox(),
    );
  }

  @override
  Widget build(BuildContext context) {
    return ConstrainedBox(
      constraints: BoxConstraints(maxHeight: WoxThemeUtil.instance.getMaxResultContainerHeight()),
      child: Obx(() => Stack(
            fit: controller.isShowActionPanel.value || controller.isShowPreviewPanel.value ? StackFit.expand : StackFit.loose,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  getResultView(),
                  getPreviewView(),
                ],
              ),
              getActionPanelView(),
              getActionFormView(),
            ],
          )),
    );
  }
}
