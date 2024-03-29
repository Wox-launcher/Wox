import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/log.dart';

class WoxListItemView extends StatelessWidget {
  final bool isActive;
  final Rx<WoxImage> icon;
  final Rx<String> title;
  final Rx<String> subTitle;
  final WoxTheme woxTheme;
  final WoxListViewType listViewType;

  const WoxListItemView({
    super.key,
    required this.woxTheme,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.isActive,
    required this.listViewType,
  });

  bool isAction() {
    return listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code;
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
          Padding(
              padding: const EdgeInsets.only(left: 5.0, right: 10.0),
              child: Obx(() {
                if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: list item view $key - icon");

                return WoxImageView(
                  woxImage: icon.value,
                  width: 30,
                  height: 30,
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
        ],
      ),
    );
  }
}
