import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_setting_util.dart';

import 'wox_hotkey_view.dart';
import 'wox_tooltip.dart';

class WoxListItemView extends StatelessWidget {
  final WoxListItem item;
  final WoxTheme woxTheme;
  final bool isActive;
  final bool isHovered;
  final WoxListViewType listViewType;

  static const _quickSelectBorderRadius = BorderRadius.all(Radius.circular(4));
  static const _textTailBorderRadius = BorderRadius.all(Radius.circular(999));
  static const _dangerTailColor = Color(0xFFB42318);
  static const _warningTailColor = Color(0xFFB54708);
  static const _successTailColor = Color(0xFF027A48);

  const WoxListItemView({super.key, required this.item, required this.woxTheme, required this.isActive, required this.isHovered, required this.listViewType});

  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;
  EdgeInsets get _tailPadding => EdgeInsets.only(left: _metrics.resultItemTailPaddingLeft, right: _metrics.resultItemTailPaddingRight);
  EdgeInsets get _tailItemPadding => EdgeInsets.only(left: _metrics.resultItemTailItemPaddingLeft);
  EdgeInsets get _iconPadding => EdgeInsets.only(left: _metrics.resultItemIconPaddingLeft, right: _metrics.resultItemIconPaddingRight);
  EdgeInsets get _subtitlePadding => EdgeInsets.only(top: _metrics.resultItemSubtitlePaddingTop);
  EdgeInsets get _quickSelectPadding => EdgeInsets.only(left: _metrics.resultItemQuickSelectPaddingLeft, right: _metrics.resultItemQuickSelectPaddingRight);
  EdgeInsets get _textTailPadding => EdgeInsets.symmetric(horizontal: _metrics.resultItemTextTailHPadding, vertical: _metrics.resultItemTextTailVPadding);

  Widget buildQuickSelectNumber() {
    final metrics = _metrics;
    final tailColor = isActive ? woxTheme.resultItemActiveTailTextColorParsed : woxTheme.resultItemTailTextColorParsed;
    final bgColor = isActive ? woxTheme.resultItemActiveBackgroundColorParsed : woxTheme.appBackgroundColorParsed;

    return Padding(
      padding: _quickSelectPadding,
      child: Container(
        width: metrics.quickSelectSize,
        height: metrics.quickSelectSize,
        decoration: BoxDecoration(color: tailColor, borderRadius: _quickSelectBorderRadius, border: Border.all(color: tailColor.withValues(alpha: 0.3), width: 1)),
        child: Center(child: Text(item.quickSelectNumber, style: TextStyle(color: bgColor, fontSize: metrics.tailHotkeyFontSize, fontWeight: FontWeight.bold))),
      ),
    );
  }

  Widget buildTails() {
    final metrics = _metrics;
    final tailTextColor = isActive ? woxTheme.resultItemActiveTailTextColorParsed : woxTheme.resultItemTailTextColorParsed;
    final activeBgColor = woxTheme.resultItemActiveBackgroundColorParsed;
    final actionBgColor = woxTheme.actionContainerBackgroundColorParsed;
    final maxTailWidth = WoxSettingUtil.instance.currentSetting.appWidth / 3;
    final maxTextTailWidth = math.max(0.0, maxTailWidth - _tailPadding.horizontal - _tailItemPadding.horizontal);

    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: maxTailWidth),
      child: Padding(
        padding: _tailPadding,
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              for (final tail in item.tails)
                if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code && tail.text != null)
                  Padding(padding: _tailItemPadding, child: buildTailTooltip(tail, buildTextTailTag(tail.text!, tail.textCategory, tailTextColor, maxTextTailWidth)))
                else if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_HOTKEY.code && tail.hotkey != null)
                  Padding(
                    padding: _tailItemPadding,
                    child: buildTailTooltip(
                      tail,
                      WoxHotkeyView(hotkey: tail.hotkey!, backgroundColor: isActive ? activeBgColor : actionBgColor, borderColor: tailTextColor, textColor: tailTextColor),
                    ),
                  )
                else if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_IMAGE.code && tail.image != null && tail.image!.imageData.isNotEmpty)
                  Padding(
                    padding: _tailItemPadding,
                    child: buildTailTooltip(
                      tail,
                      WoxImageView(woxImage: tail.image!, width: tail.imageWidth ?? metrics.tailImageSize, height: tail.imageHeight ?? metrics.tailImageSize),
                    ),
                  ),
            ],
          ),
        ),
      ),
    );
  }

  Widget buildTailTooltip(WoxListItemTail tail, Widget child) {
    final tooltip = tail.tooltip?.trim() ?? "";
    if (tooltip.isEmpty) {
      return child;
    }

    // Feature change: result tails can now be icon-only metadata, so the shared
    // tooltip wrapper gives compact tails an accessible label without changing row layout.
    return WoxTooltip(message: tooltip, child: child);
  }

  _TextTailStyle _getTextTailStyle(String textCategory, Color defaultTextColor) {
    final normalizedCategory = WoxListItemTailTextCategoryEnum.ensureCode(textCategory);

    switch (normalizedCategory) {
      case woxListItemTailTextCategoryDanger:
        return _buildSemanticTextTailStyle(_dangerTailColor);
      case woxListItemTailTextCategoryWarning:
        return _buildSemanticTextTailStyle(_warningTailColor);
      case woxListItemTailTextCategorySuccess:
        return _buildSemanticTextTailStyle(_successTailColor);
      default:
        return _TextTailStyle(textColor: defaultTextColor, backgroundColor: Colors.transparent, borderColor: defaultTextColor.withValues(alpha: isActive ? 0.34 : 0.2));
    }
  }

  _TextTailStyle _buildSemanticTextTailStyle(Color semanticColor) {
    // Category tails used to tint only the text, which became unreadable when a
    // theme's active row background was close to the semantic color. Solid chips
    // keep status categories recognizable and use white text for stable contrast
    // without requiring new theme fields or plugin API changes.
    return _TextTailStyle(textColor: Colors.white, backgroundColor: semanticColor, borderColor: semanticColor.withValues(alpha: 0.72));
  }

  Widget buildTextTailTag(String text, String textCategory, Color defaultTextColor, double maxTailWidth) {
    final tailStyle = _getTextTailStyle(textCategory, defaultTextColor);

    // Text tails live inside a horizontal scroll view, whose child receives unbounded
    // width. The old unconstrained Text made ellipsis impossible and clipped long
    // pill borders into a thin line, so each text tail is capped to the visible tail
    // area while the tail row can still scroll when there are multiple tail items.
    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: maxTailWidth),
      child: DecoratedBox(
        decoration: BoxDecoration(color: tailStyle.backgroundColor, borderRadius: _textTailBorderRadius, border: Border.all(color: tailStyle.borderColor)),
        child: Padding(
          padding: _textTailPadding,
          child: Center(
            widthFactor: 1,
            heightFactor: 1,
            child: Text(text, style: TextStyle(color: tailStyle.textColor, fontSize: _metrics.tailHotkeyFontSize), maxLines: 1, overflow: TextOverflow.ellipsis),
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
    if (LoggerSwitch.enablePaintLog) {
      Logger.instance.debug(const UuidV4().generate(), "repaint: list item view ${item.title} - container");
    }

    final bool isResultList = listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code;
    final bool isActionList = listViewType == WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code;
    // Bug fix: action rows share the same active-row visual language as result rows, but
    // the old radius guard clipped only result rows and left action selection rectangular.
    // Reusing ResultItemBorderRadius keeps the action panel aligned with the result list
    // without introducing a duplicate theme field for the same row-surface shape.
    final BorderRadius borderRadius =
        (isResultList || isActionList) && woxTheme.resultItemBorderRadius > 0 ? BorderRadius.circular(woxTheme.resultItemBorderRadius.toDouble()) : BorderRadius.zero;

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

    // Build icon widget
    final metrics = _metrics;

    // Action and result rows have different base heights (40px vs 50px), so
    // icon and title sizes are sourced from type-specific metric fields to keep
    // content proportional to the row without hard-coding per-list values here.
    final double iconSize = isActionList ? metrics.actionIconSize : metrics.resultIconSize;
    final double titleFontSize = isActionList ? metrics.actionTitleFontSize : metrics.resultTitleFontSize;

    final Widget iconWidget =
        item.isGroup
            ? const SizedBox()
            : Padding(padding: _iconPadding, child: SizedBox(width: iconSize, height: iconSize, child: WoxImageView(woxImage: item.icon, width: iconSize, height: iconSize)));

    // Build title/subtitle widget
    final Widget textWidget = Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(item.title, style: TextStyle(fontSize: titleFontSize, color: titleColor), maxLines: 1, overflow: TextOverflow.ellipsis),
          if (item.subTitle.isNotEmpty)
            Padding(
              padding: _subtitlePadding,
              child: Text(item.subTitle, style: TextStyle(color: subtitleColor, fontSize: metrics.resultSubtitleFontSize), maxLines: 1, overflow: TextOverflow.ellipsis),
            ),
        ],
      ),
    );

    // Build tails widget
    final Widget? tailsWidget = item.tails.isNotEmpty ? buildTails() : null;

    // Build quick select widget
    final Widget? quickSelectWidget = (item.isShowQuickSelect && item.quickSelectNumber.isNotEmpty) ? buildQuickSelectNumber() : null;

    Widget content = Container(
      decoration: BoxDecoration(color: getBackgroundColor()),
      padding:
          isResultList
              ? EdgeInsets.only(
                top: WoxInterfaceSizeUtil.instance.current.scaledSpacing(woxTheme.resultItemPaddingTop.toDouble()),
                right: WoxInterfaceSizeUtil.instance.current.scaledSpacing(woxTheme.resultItemPaddingRight.toDouble()),
                bottom: WoxInterfaceSizeUtil.instance.current.scaledSpacing(woxTheme.resultItemPaddingBottom.toDouble()),
                left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(woxTheme.resultItemPaddingLeft.toDouble() + maxBorderWidth),
              )
              : EdgeInsets.only(left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(maxBorderWidth)),
      child: Row(children: [iconWidget, textWidget, if (tailsWidget != null) tailsWidget, if (quickSelectWidget != null) quickSelectWidget]),
    );

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

    return content;
  }
}

class _TextTailStyle {
  final Color textColor;
  final Color backgroundColor;
  final Color borderColor;

  const _TextTailStyle({required this.textColor, required this.backgroundColor, required this.borderColor});
}
