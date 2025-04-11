import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  List<WoxQueryResultTail> getHotkeyTails(WoxResultAction action) {
    var tails = <WoxQueryResultTail>[];
    if (action.hotkey != "") {
      var hotkey = WoxHotkey.parseHotkeyFromString(action.hotkey);
      if (hotkey != null) {
        tails.add(WoxQueryResultTail.hotkey(hotkey));
      }
    }
    return tails;
  }

  Widget getActionPanelView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action panel view container");

    return Obx(
      () => controller.isShowActionPanel.value
          ? Positioned(
              right: 10,
              bottom: 10,
              child: Container(
                padding: EdgeInsets.only(
                  top: controller.woxTheme.value.actionContainerPaddingTop.toDouble(),
                  bottom: controller.woxTheme.value.actionContainerPaddingBottom.toDouble(),
                  left: controller.woxTheme.value.actionContainerPaddingLeft.toDouble(),
                  right: controller.woxTheme.value.actionContainerPaddingRight.toDouble(),
                ),
                decoration: BoxDecoration(
                  color: fromCssColor(controller.woxTheme.value.actionContainerBackgroundColor),
                  borderRadius: BorderRadius.circular(controller.woxTheme.value.actionQueryBoxBorderRadius.toDouble()),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withValues(alpha: 0.1),
                      spreadRadius: 2,
                      blurRadius: 8,
                      offset: const Offset(0, 3),
                    ),
                  ],
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.start,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(controller.actionsTitle.value, style: TextStyle(color: fromCssColor(controller.woxTheme.value.actionContainerHeaderFontColor), fontSize: 16.0)),
                    const Divider(),
                    Obx(() => WoxListView(
                          controller: controller.actionListViewController,
                          maxHeight: 320,
                          items: controller.actions
                              .map((action) => WoxListItem(
                                    id: action.id,
                                    icon: action.icon.value,
                                    title: action.name.value,
                                    hotkey: action.hotkey,
                                    tails: getHotkeyTails(action),
                                    subTitle: "",
                                    isGroup: false,
                                  ))
                              .toList(),
                          woxTheme: controller.woxTheme.value,
                          listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code,
                          onFilterEscPressed: () {
                            controller.hideActionPanel(const UuidV4().generate());
                          },
                        )),
                  ],
                ),
              ),
            )
          : const SizedBox(),
    );
  }

  Widget getResultContainer() {
    return Container(
      padding: EdgeInsets.only(
        top: controller.woxTheme.value.resultContainerPaddingTop.toDouble(),
        right: controller.woxTheme.value.resultContainerPaddingRight.toDouble(),
        bottom: controller.woxTheme.value.resultContainerPaddingBottom.toDouble(),
        left: controller.woxTheme.value.resultContainerPaddingLeft.toDouble(),
      ),
      child: WoxListView(
        controller: controller.resultListViewController,
        listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code,
        maxHeight: WoxThemeUtil.instance.getMaxResultListViewHeight(),
        showFilter: false,
        items: controller.results
            .map((result) => WoxListItem(
                  id: result.id,
                  icon: result.icon.value,
                  title: result.title.value,
                  tails: result.tails,
                  subTitle: result.subTitle.value,
                  isGroup: result.isGroup,
                ))
            .toList(),
        woxTheme: controller.woxTheme.value,
        onItemExecuted: (item) {
          controller.onEnter(const UuidV4().generate());
        },
        onItemActive: (item) {
          controller.onResultItemActivated(const UuidV4().generate(), item);
        },
      ),
    );
  }

  Widget getResultView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: result view container");

    return Obx(
      () => controller.results.isNotEmpty
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
                            woxTheme: controller.woxTheme.value,
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
                      woxTheme: controller.woxTheme.value,
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
              getActionPanelView()
            ],
          )),
    );
  }
}
