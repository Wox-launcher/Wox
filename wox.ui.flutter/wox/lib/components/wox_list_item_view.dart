import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';

import 'wox_hotkey_view.dart';

class WoxListItemView extends StatelessWidget {
  final WoxListItem item;
  final WoxTheme woxTheme;
  final bool isActive;
  final bool isHovered;
  final WoxListViewType listViewType;

  // Static const values for performance
  static const _tailPadding = EdgeInsets.only(left: 10.0, right: 5.0);
  static const _tailItemPadding = EdgeInsets.only(left: 10.0);
  static const _iconPadding = EdgeInsets.only(left: 5.0, right: 10.0);
  static const _subtitlePadding = EdgeInsets.only(top: 2.0);
  static const _quickSelectPadding = EdgeInsets.only(left: 10.0, right: 5.0);
  static const _quickSelectBorderRadius = BorderRadius.all(Radius.circular(4));
  static const _strutStyle = StrutStyle(forceStrutHeight: true);
  static const _iconSize = 30.0;
  static const _quickSelectSize = 24.0;
  static const _tailImageSize = 20.0;

  const WoxListItemView({super.key, required this.item, required this.woxTheme, required this.isActive, required this.isHovered, required this.listViewType});

  Widget buildQuickSelectNumber() {
    final tailColor = isActive ? woxTheme.resultItemActiveTailTextColorParsed : woxTheme.resultItemTailTextColorParsed;
    final bgColor = isActive ? woxTheme.resultItemActiveBackgroundColorParsed : woxTheme.appBackgroundColorParsed;

    return Padding(
      padding: _quickSelectPadding,
      child: Container(
        width: _quickSelectSize,
        height: _quickSelectSize,
        decoration: BoxDecoration(color: tailColor, borderRadius: _quickSelectBorderRadius, border: Border.all(color: tailColor.withValues(alpha: 0.3), width: 1)),
        child: Center(child: Text(item.quickSelectNumber, style: TextStyle(color: bgColor, fontSize: 12, fontWeight: FontWeight.bold))),
      ),
    );
  }

  Widget buildTails() {
    final tailTextColor = isActive ? woxTheme.resultItemActiveTailTextColorParsed : woxTheme.resultItemTailTextColorParsed;
    final activeBgColor = woxTheme.resultItemActiveBackgroundColorParsed;
    final actionBgColor = woxTheme.actionContainerBackgroundColorParsed;

    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: WoxSettingUtil.instance.currentSetting.appWidth / 2),
      child: Padding(
        padding: _tailPadding,
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              for (final tail in item.tails)
                if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code && tail.text != null)
                  Padding(
                    padding: _tailItemPadding,
                    child: Text(tail.text!, style: TextStyle(color: tailTextColor, fontSize: 12), maxLines: 1, overflow: TextOverflow.ellipsis, strutStyle: _strutStyle),
                  )
                else if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_HOTKEY.code && tail.hotkey != null)
                  Padding(
                    padding: _tailItemPadding,
                    child: WoxHotkeyView(hotkey: tail.hotkey!, backgroundColor: isActive ? activeBgColor : actionBgColor, borderColor: tailTextColor, textColor: tailTextColor),
                  )
                else if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_IMAGE.code && tail.image != null && tail.image!.imageData.isNotEmpty)
                  Padding(padding: _tailItemPadding, child: WoxImageView(woxImage: tail.image!, width: _tailImageSize, height: _tailImageSize)),
            ],
          ),
        ),
      ),
    );
  }

  Color getBackgroundColor() {
    final isActionType = listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code;

    if (isActive) {
      return isActionType ? woxTheme.actionItemActiveBackgroundColorParsed : woxTheme.resultItemActiveBackgroundColorParsed;
    }
    if (isHovered) {
      final activeColor = isActionType ? woxTheme.actionItemActiveBackgroundColorParsed : woxTheme.resultItemActiveBackgroundColorParsed;
      return activeColor.withValues(alpha: 0.3);
    }
    return Colors.transparent;
  }

  @override
  Widget build(BuildContext context) {
    final Stopwatch? buildStopwatch = LoggerSwitch.enableBuildTimeLog ? (Stopwatch()..start()) : null;
    int? checkpoint1, checkpoint2, checkpoint3;

    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: list item view ${item.title} - container");

    final bool isResultList = listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code;
    final bool isActionList = listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code;
    final BorderRadius borderRadius = isResultList && woxTheme.resultItemBorderRadius > 0 ? BorderRadius.circular(woxTheme.resultItemBorderRadius.toDouble()) : BorderRadius.zero;

    // Calculate the maximum border width to reserve space
    final double maxBorderWidth = isActionList ? 0 : math.max(woxTheme.resultItemBorderLeftWidth.toDouble(), woxTheme.resultItemActiveBorderLeftWidth.toDouble());

    // Calculate the actual border width for current state
    final double actualBorderWidth = isActionList ? 0 : (isActive ? woxTheme.resultItemActiveBorderLeftWidth.toDouble() : woxTheme.resultItemBorderLeftWidth.toDouble());

    // Pre-compute colors for title/subtitle
    final Color titleColor =
        isActionList
            ? (isActive ? woxTheme.actionItemActiveFontColorParsed : woxTheme.actionItemFontColorParsed)
            : (isActive ? woxTheme.resultItemActiveTitleColorParsed : woxTheme.resultItemTitleColorParsed);
    final Color subtitleColor = isActive ? woxTheme.resultItemActiveSubTitleColorParsed : woxTheme.resultItemSubTitleColorParsed;

    if (buildStopwatch != null) checkpoint1 = buildStopwatch.elapsedMicroseconds;

    // Build icon widget
    final Widget iconWidget =
        item.isGroup
            ? const SizedBox()
            : Padding(padding: _iconPadding, child: SizedBox(width: _iconSize, height: _iconSize, child: WoxImageView(woxImage: item.icon, width: _iconSize, height: _iconSize)));

    int? checkpointIcon;
    if (buildStopwatch != null) checkpointIcon = buildStopwatch.elapsedMicroseconds;

    // Build title/subtitle widget
    final Widget textWidget = Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(item.title, style: TextStyle(fontSize: 16, color: titleColor), maxLines: 1, overflow: TextOverflow.ellipsis, strutStyle: _strutStyle),
          if (item.subTitle.isNotEmpty)
            Padding(
              padding: _subtitlePadding,
              child: Text(item.subTitle, style: TextStyle(color: subtitleColor, fontSize: 13), maxLines: 1, overflow: TextOverflow.ellipsis, strutStyle: _strutStyle),
            ),
        ],
      ),
    );

    int? checkpointText;
    if (buildStopwatch != null) checkpointText = buildStopwatch.elapsedMicroseconds;

    // Build tails widget
    final Widget? tailsWidget = item.tails.isNotEmpty ? buildTails() : null;

    int? checkpointTails;
    if (buildStopwatch != null) checkpointTails = buildStopwatch.elapsedMicroseconds;

    // Build quick select widget
    final Widget? quickSelectWidget = (item.isShowQuickSelect && item.quickSelectNumber.isNotEmpty) ? buildQuickSelectNumber() : null;

    int? checkpointQuickSelect;
    if (buildStopwatch != null) checkpointQuickSelect = buildStopwatch.elapsedMicroseconds;

    Widget content = Container(
      decoration: BoxDecoration(color: getBackgroundColor()),
      padding:
          isResultList
              ? EdgeInsets.only(
                top: woxTheme.resultItemPaddingTop.toDouble(),
                right: woxTheme.resultItemPaddingRight.toDouble(),
                bottom: woxTheme.resultItemPaddingBottom.toDouble(),
                left: woxTheme.resultItemPaddingLeft.toDouble() + maxBorderWidth,
              )
              : EdgeInsets.only(left: maxBorderWidth),
      child: Row(children: [iconWidget, textWidget, if (tailsWidget != null) tailsWidget, if (quickSelectWidget != null) quickSelectWidget]),
    );

    if (buildStopwatch != null) checkpoint2 = buildStopwatch.elapsedMicroseconds;

    if (borderRadius != BorderRadius.zero) {
      content = ClipRRect(borderRadius: borderRadius, child: content);
    }

    // Use Stack to overlay the left border indicator without affecting layout
    if (actualBorderWidth > 0) {
      content = Stack(
        children: [
          content,
          Positioned(
            left: 0,
            top: 0,
            bottom: 0,
            child: Container(
              width: actualBorderWidth,
              decoration: BoxDecoration(
                color: woxTheme.resultItemActiveBackgroundColorParsed,
                borderRadius: borderRadius != BorderRadius.zero ? BorderRadius.only(topLeft: borderRadius.topLeft, bottomLeft: borderRadius.bottomLeft) : BorderRadius.zero,
              ),
            ),
          ),
        ],
      );
    }

    if (buildStopwatch != null) {
      checkpoint3 = buildStopwatch.elapsedMicroseconds;
      buildStopwatch.stop();
      final iconTime = checkpointIcon! - checkpoint1!;
      final textTime = checkpointText! - checkpointIcon;
      final tailsTime = checkpointTails! - checkpointText;
      final quickSelectTime = checkpointQuickSelect! - checkpointTails;
      final containerTime = checkpoint2! - checkpointQuickSelect;
      Logger.instance.debug(
        const UuidV4().generate(),
        "flutter build metric: list item ${item.title} - total:${buildStopwatch.elapsedMicroseconds}μs, prep:${checkpoint1}μs, icon:${iconTime}μs, text:${textTime}μs, tails:${tailsTime}μs, qs:${quickSelectTime}μs, container:${containerTime}μs, wrap:${checkpoint3! - checkpoint2}μs",
      );
    }

    return content;
  }
}
