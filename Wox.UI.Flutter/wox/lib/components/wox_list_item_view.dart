import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';

import 'wox_hotkey_view.dart';

class WoxListItemView extends StatelessWidget {
  final bool isActive;
  final Rx<WoxImage> icon;
  final Rx<String> title;
  final Rx<String> subTitle;
  final RxList<WoxQueryResultTail> tails;
  final WoxTheme woxTheme;
  final WoxListViewType listViewType;
  final bool isGroup;

  const WoxListItemView({
    super.key,
    required this.woxTheme,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.tails,
    required this.isActive,
    required this.listViewType,
    required this.isGroup,
  });

  bool isAction() {
    return listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code;
  }

  double getImageSize(WoxImage img, double defaultSize) {
    if (img.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      return defaultSize - 10;
    } else {
      return defaultSize;
    }
  }

  Widget buildTails() {
    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: WoxSettingUtil.instance.currentSetting.appWidth / 2),
      child: Padding(
        padding: const EdgeInsets.only(left: 10.0, right: 5.0),
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              for (final tail in tails)
                if (tail.type == WoxQueryResultTailTypeEnum.WOX_QUERY_RESULT_TAIL_TYPE_TEXT.code && tail.text != null)
                  Padding(
                    padding: const EdgeInsets.only(left: 10.0),
                    child: Text(
                      tail.text!,
                      style: TextStyle(
                        color: fromCssColor(isActive ? woxTheme.resultItemActiveTailTextColor : woxTheme.resultItemTailTextColor),
                        fontSize: 12,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      strutStyle: const StrutStyle(
                        forceStrutHeight: true,
                      ),
                    ),
                  )
                else if (tail.type == WoxQueryResultTailTypeEnum.WOX_QUERY_RESULT_TAIL_TYPE_HOTKEY.code && tail.hotkey != null)
                  Padding(
                    padding: const EdgeInsets.only(left: 10.0),
                    child: WoxHotkeyView(
                      hotkey: tail.hotkey!,
                      backgroundColor: isActive ? fromCssColor(woxTheme.resultItemActiveBackgroundColor) : fromCssColor(woxTheme.actionContainerBackgroundColor),
                      borderColor: fromCssColor(isActive ? woxTheme.resultItemActiveTailTextColor : woxTheme.resultItemTailTextColor),
                      textColor: fromCssColor(isActive ? woxTheme.resultItemActiveTailTextColor : woxTheme.resultItemTailTextColor),
                    ),
                  )
                else if (tail.type == WoxQueryResultTailTypeEnum.WOX_QUERY_RESULT_TAIL_TYPE_IMAGE.code && tail.image != null && tail.image!.imageData.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(left: 10.0),
                    child: WoxImageView(
                      woxImage: tail.image!,
                      width: getImageSize(tail.image!, 24),
                      height: getImageSize(tail.image!, 24),
                    ),
                  ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: list item view $key - container");

    return Container(
      decoration: BoxDecoration(
        color: isActive ? fromCssColor(isAction() ? woxTheme.actionItemActiveBackgroundColor : woxTheme.resultItemActiveBackgroundColor) : Colors.transparent,
        borderRadius: BorderRadius.circular(isAction() ? 0.0 : woxTheme.resultItemBorderRadius.toDouble()),
        border: Border(
            left: isAction()
                ? BorderSide.none
                : BorderSide(
                    color: isActive ? fromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent,
                    width: isActive ? double.parse(woxTheme.resultItemActiveBorderLeft) : double.parse(woxTheme.resultItemBorderLeft),
                  )),
      ),
      padding: isAction()
          ? EdgeInsets.zero
          : EdgeInsets.only(
              top: woxTheme.resultItemPaddingTop.toDouble(),
              right: woxTheme.resultItemPaddingRight.toDouble(),
              bottom: woxTheme.resultItemPaddingBottom.toDouble(),
              left: woxTheme.resultItemPaddingLeft.toDouble(),
            ),
      child: Row(
        children: [
          isGroup
              ? const SizedBox()
              : Padding(
                  padding: const EdgeInsets.only(left: 5.0, right: 10.0),
                  child: Obx(() {
                    if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: list item view $key - icon");

                    return WoxImageView(
                      woxImage: icon.value,
                      width: getImageSize(icon.value, 30),
                      height: getImageSize(icon.value, 30),
                    );
                  })),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisAlignment: MainAxisAlignment.center, children: [
              Obx(() {
                if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: list item view $key - title");

                return Text(
                  title.value,
                  style: TextStyle(
                    fontSize: 16,
                    color: isAction()
                        ? fromCssColor(isActive ? woxTheme.actionItemActiveFontColor : woxTheme.actionItemFontColor)
                        : fromCssColor(isActive ? woxTheme.resultItemActiveTitleColor : woxTheme.resultItemTitleColor),
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  strutStyle: const StrutStyle(
                    forceStrutHeight: true,
                  ),
                );
              }),
              Obx(() {
                if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: list item view $key - subtitle");

                return subTitle.isNotEmpty
                    ? Padding(
                        padding: const EdgeInsets.only(top: 2.0),
                        child: Text(
                          subTitle.value,
                          style: TextStyle(
                            color: fromCssColor(isActive ? woxTheme.resultItemActiveSubTitleColor : woxTheme.resultItemSubTitleColor),
                            fontSize: 13,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          strutStyle: const StrutStyle(
                            forceStrutHeight: true,
                          ),
                        ),
                      )
                    : const SizedBox();
              }),
            ]),
          ),
          // Tails
          Obx(() {
            if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: list item view $key - tails");
            if (tails.isNotEmpty) {
              return buildTails();
            } else {
              return const SizedBox();
            }
          }),
        ],
      ),
    );
  }
}
