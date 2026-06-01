import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

/// Renders the launcher Glance pill and keeps hover state across Glance refreshes.
class WoxGlanceItemView extends StatefulWidget {
  WoxGlanceItemView({Key? key, required this.currentTheme, required this.item, required this.controller, required this.getItemWidth})
    : super(key: key ?? ValueKey('glance-item-${item.pluginId}\x00${item.id}'));

  final dynamic currentTheme;
  final GlanceItem item;
  final WoxLauncherController controller;
  final double Function(BuildContext, GlanceItem, TextStyle) getItemWidth;

  @override
  State<WoxGlanceItemView> createState() => _WoxGlanceItemViewState();
}

class _WoxGlanceItemViewState extends State<WoxGlanceItemView> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    final item = widget.item;
    final controller = widget.controller;
    final baseTextColor = safeFromCssColor(widget.currentTheme.queryBoxFontColor);
    // Glance now has no status field in v1; keeping one quiet opacity preserves
    // the auxiliary feel without exposing unused state semantics in the API.
    const textAlpha = 0.8;
    final textColor = baseTextColor.withValues(alpha: textAlpha);

    // Glance is auxiliary status, so the default state is fully transparent and
    // visually merges with the query box; hover is only a light affordance.
    // Glance items are the launcher chrome shown in the screenshot. Using
    // WoxTooltip here keeps plugin/system glance hints visually aligned with
    // other launcher overlays instead of falling back to Material Tooltip.
    return WoxTooltip(
      message: item.tooltip.isNotEmpty ? item.tooltip : item.text,
      preferSide: WoxTooltipSide.top,
      child: Builder(
        builder: (context) {
          final metrics = WoxInterfaceSizeUtil.instance.current;
          final textStyle = TextStyle(color: textColor, fontSize: metrics.scaledSpacing(15));
          final itemWidth = widget.getItemWidth(context, item, textStyle);
          final iconVisible = controller.shouldShowGlanceIcon(item);

          return MouseRegion(
            cursor: item.action == null ? SystemMouseCursors.basic : SystemMouseCursors.click,
            onEnter: (_) => setState(() => _isHovered = true),
            onExit: (_) => setState(() => _isHovered = false),
            child: GestureDetector(
              onTap: item.action == null ? null : () => controller.executeGlanceDefaultAction(const UuidV4().generate(), item),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 120),
                width: itemWidth,
                height: metrics.scaledSpacing(30),
                padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(8)),
                decoration: BoxDecoration(
                  color: _isHovered ? baseTextColor.withValues(alpha: 0.10) : Colors.transparent,
                  borderRadius: BorderRadius.circular(5),
                  border: Border.all(color: _isHovered ? baseTextColor.withValues(alpha: 0.08) : Colors.transparent),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (iconVisible) ...[
                      Opacity(
                        opacity: textAlpha * 0.9,
                        child: WoxImageView(woxImage: item.icon, width: metrics.scaledSpacing(16), height: metrics.scaledSpacing(16), svgColor: textColor),
                      ),
                      SizedBox(width: metrics.scaledSpacing(5)),
                    ],
                    Flexible(child: Text(item.text, overflow: TextOverflow.ellipsis, maxLines: 1, style: textStyle)),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
