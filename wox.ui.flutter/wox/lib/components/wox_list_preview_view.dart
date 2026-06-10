import 'package:flutter/material.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_preview_list.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxListPreviewView extends StatelessWidget {
  final WoxPreviewListData data;
  final WoxTheme woxTheme;

  const WoxListPreviewView({super.key, required this.data, required this.woxTheme});

  @override
  Widget build(BuildContext context) {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final splitLineColor = safeFromCssColor(woxTheme.previewSplitLineColor);

    // A generic list preview can represent selected files, compression
    // progress, or other status rows. Rendering only row data here prevents the
    // preview from leaking file-specific assumptions back into plugin payloads.
    if (data.items.isEmpty) {
      return Center(child: Text("No items", style: TextStyle(color: fontColor.withValues(alpha: 0.62), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize)));
    }

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12), vertical: WoxInterfaceSizeUtil.instance.current.scaledSpacing(10)),
      child: Column(
        children: [
          for (var index = 0; index < data.items.length; index++) ...[
            _ListPreviewRow(item: data.items[index], woxTheme: woxTheme),
            if (index != data.items.length - 1) Divider(height: 1, color: splitLineColor.withValues(alpha: 0.28)),
          ],
        ],
      ),
    );
  }
}

class _ListPreviewRow extends StatelessWidget {
  final WoxPreviewListItem item;
  final WoxTheme woxTheme;

  const _ListPreviewRow({required this.item, required this.woxTheme});

  @override
  Widget build(BuildContext context) {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final splitLineColor = safeFromCssColor(woxTheme.previewSplitLineColor);
    final propertyColor = safeFromCssColor(woxTheme.previewPropertyContentColor);
    final metrics = WoxInterfaceSizeUtil.instance.current;

    // List preview rows belong to the launcher preview surface, so text, icon,
    // and chip sizes follow density while semantic colors and borders stay local
    // to the preview styling.
    return Padding(
      padding: EdgeInsets.symmetric(vertical: metrics.scaledSpacing(10)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Container(
            width: metrics.scaledSpacing(34),
            height: metrics.scaledSpacing(34),
            decoration: BoxDecoration(
              color: propertyColor.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: splitLineColor.withValues(alpha: 0.38)),
            ),
            child: _buildIcon(propertyColor),
          ),
          SizedBox(width: metrics.scaledSpacing(12)),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                WoxTooltip(
                  message: item.title,
                  child: Text(
                    item.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: fontColor.withValues(alpha: 0.92), fontSize: metrics.resultTitleFontSize, height: 1.2),
                  ),
                ),
                if (item.subtitle.isNotEmpty) ...[
                  SizedBox(height: metrics.scaledSpacing(4)),
                  WoxTooltip(
                    message: item.subtitle,
                    child: Text(
                      item.subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: fontColor.withValues(alpha: 0.56), fontSize: metrics.resultSubtitleFontSize, height: 1.2),
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (item.tails.isNotEmpty) ...[
            SizedBox(width: metrics.scaledSpacing(10)),
            ConstrainedBox(
              constraints: BoxConstraints(maxWidth: metrics.scaledSpacing(150)),
              child: SingleChildScrollView(scrollDirection: Axis.horizontal, child: Row(children: item.tails.map((tail) => _buildTail(tail, fontColor, splitLineColor)).toList())),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildIcon(Color color) {
    final icon = item.icon;
    if (icon == null || icon.imageData.isEmpty) {
      return Icon(Icons.list_alt_outlined, color: color.withValues(alpha: 0.88), size: WoxInterfaceSizeUtil.instance.current.scaledSpacing(18));
    }

    return Center(child: WoxImageView(woxImage: icon, width: WoxInterfaceSizeUtil.instance.current.tailImageSize, height: WoxInterfaceSizeUtil.instance.current.tailImageSize));
  }

  Widget _buildTail(WoxListItemTail tail, Color fontColor, Color splitLineColor) {
    if (tail.type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_IMAGE.code && tail.image != null && tail.image!.imageData.isNotEmpty) {
      return Padding(
        padding: EdgeInsets.only(left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(6)),
        child: WoxImageView(
          woxImage: tail.image!,
          width: tail.imageWidth ?? WoxInterfaceSizeUtil.instance.current.tailImageSize,
          height: tail.imageHeight ?? WoxInterfaceSizeUtil.instance.current.tailImageSize,
        ),
      );
    }

    if (tail.type != WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code || tail.text == null) {
      return const SizedBox.shrink();
    }

    final style = _tailStyle(tail.textCategory, fontColor, splitLineColor);
    return Padding(
      padding: EdgeInsets.only(left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(6)),
      child: Container(
        constraints: BoxConstraints(maxWidth: WoxInterfaceSizeUtil.instance.current.scaledSpacing(92)),
        padding: EdgeInsets.symmetric(horizontal: WoxInterfaceSizeUtil.instance.current.scaledSpacing(8), vertical: WoxInterfaceSizeUtil.instance.current.scaledSpacing(4)),
        decoration: BoxDecoration(color: style.backgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: style.borderColor)),
        child: Text(
          tail.text!,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: style.textColor, fontSize: WoxInterfaceSizeUtil.instance.current.tailHotkeyFontSize, height: 1.1),
        ),
      ),
    );
  }

  _TailStyle _tailStyle(String textCategory, Color fontColor, Color splitLineColor) {
    final normalizedCategory = WoxListItemTailTextCategoryEnum.ensureCode(textCategory);
    if (normalizedCategory == woxListItemTailTextCategoryDefault) {
      // Default preview tails keep the quiet theme-owned treatment because they
      // are metadata, not status badges, and should not compete with semantic
      // category tails.
      final textColor = fontColor.withValues(alpha: 0.62);
      return _TailStyle(textColor: textColor, backgroundColor: textColor.withValues(alpha: 0.035), borderColor: splitLineColor.withValues(alpha: 0.42));
    }

    final semanticColor = switch (normalizedCategory) {
      woxListItemTailTextCategoryDanger => const Color(0xFFB42318),
      woxListItemTailTextCategoryWarning => const Color(0xFFB54708),
      woxListItemTailTextCategorySuccess => const Color(0xFF027A48),
      _ => fontColor.withValues(alpha: 0.62),
    };

    // Preview category tails match result-row category tails: a solid semantic
    // chip with white text. The old text-only tint was too close to some active
    // and panel backgrounds, so the solid fill keeps status tails legible.
    return _TailStyle(textColor: Colors.white, backgroundColor: semanticColor, borderColor: semanticColor.withValues(alpha: 0.72));
  }
}

class _TailStyle {
  final Color textColor;
  final Color backgroundColor;
  final Color borderColor;

  const _TailStyle({required this.textColor, required this.backgroundColor, required this.borderColor});
}
