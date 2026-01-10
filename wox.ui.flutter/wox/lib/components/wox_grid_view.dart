import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_grid_controller.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxGridView extends StatelessWidget {
  final WoxGridController<WoxQueryResult> controller;
  final double maxHeight;
  final VoidCallback? onItemTapped;
  final VoidCallback? onRowHeightChanged;

  // Height for title text (fontSize 12 + some extra)
  static const double titleHeight = 18.0;

  const WoxGridView({
    super.key,
    required this.controller,
    required this.maxHeight,
    this.onItemTapped,
    this.onRowHeightChanged,
  });

  @override
  Widget build(BuildContext context) {
    final columns = controller.gridLayoutParams.columns;
    final showTitle = controller.gridLayoutParams.showTitle;
    final itemPadding = controller.gridLayoutParams.itemPadding;
    final itemMargin = controller.gridLayoutParams.itemMargin;

    return LayoutBuilder(
      builder: (context, constraints) {
        // Calculate icon size based on available width, columns, and margins
        // Use floor to avoid floating point precision overflow
        final availableWidth = constraints.maxWidth;
        final cellWidth = columns > 0 ? (availableWidth / columns).floorToDouble() : 48.0;
        final iconSize = cellWidth - (itemPadding + itemMargin) * 2;
        // Cell height includes icon + padding/margin, and title height if showing title
        final cellHeight = cellWidth + (showTitle ? titleHeight : 0);

        // Update controller with the calculated row height for scroll calculations
        final heightChanged = controller.updateRowHeight(cellHeight);
        if (heightChanged) {
          WidgetsBinding.instance.addPostFrameCallback((_) => onRowHeightChanged?.call());
        }

        return ConstrainedBox(
          constraints: BoxConstraints(maxHeight: maxHeight),
          child: Scrollbar(
            thumbVisibility: true,
            controller: controller.scrollController,
            child: Listener(
              onPointerSignal: (event) {
                if (event is PointerScrollEvent) {
                  controller.updateActiveIndexByDirection(
                    const UuidV4().generate(),
                    event.scrollDelta.dy > 0 ? WoxDirectionEnum.WOX_DIRECTION_DOWN.code : WoxDirectionEnum.WOX_DIRECTION_UP.code,
                  );
                }
              },
              child: Obx(() => _buildGridWithGroups(cellHeight, iconSize, columns, showTitle, itemPadding, itemMargin)),
            ),
          ),
        );
      },
    );
  }

  Widget _buildGridWithGroups(double cellSize, double iconSize, int columns, bool showTitle, double itemPadding, double itemMargin) {
    final items = controller.items;
    if (items.isEmpty) return const SizedBox.shrink();

    List<Widget> rows = [];
    int i = 0;

    while (i < items.length) {
      final item = items[i];

      if (item.value.isGroup) {
        // Add group header
        rows.add(_buildGroupHeader(item.value, i));
        i++;
      } else {
        // Collect items for this row (up to columns count, stop at next group or group change)
        List<int> rowIndices = [];
        final currentGroup = items[i].value.data.group;
        while (i < items.length && !items[i].value.isGroup && items[i].value.data.group == currentGroup && rowIndices.length < columns) {
          rowIndices.add(i);
          i++;
        }

        // Build grid row
        rows.add(_buildGridRow(rowIndices, cellSize, iconSize, showTitle, columns, itemPadding, itemMargin));
      }
    }

    return SingleChildScrollView(
      controller: controller.scrollController,
      physics: const NeverScrollableScrollPhysics(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: rows,
      ),
    );
  }

  Widget _buildGroupHeader(WoxListItem<WoxQueryResult> item, int index) {
    return Padding(
      padding: const EdgeInsets.only(left: 8, top: 12, bottom: 4),
      child: Text(
        item.title,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w500,
          color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
        ),
      ),
    );
  }

  Widget _buildGridRow(List<int> indices, double cellSize, double iconSize, bool showTitle, int columns, double itemPadding, double itemMargin) {
    return Row(
      children: [
        for (int i = 0; i < columns; i++)
          Expanded(
            child: i < indices.length
                ? SizedBox(
                    height: cellSize,
                    child: _buildGridItemWidget(indices[i], iconSize, showTitle, itemPadding, itemMargin),
                  )
                : SizedBox(height: cellSize),
          ),
      ],
    );
  }

  Widget _buildGridItemWidget(int index, double iconSize, bool showTitle, double itemPadding, double itemMargin) {
    final item = controller.items[index];

    return MouseRegion(
      onEnter: (_) {
        if (controller.isMouseMoved && !item.value.isGroup) {
          controller.updateHoveredIndex(index);
        }
      },
      onHover: (_) {
        if (!controller.isMouseMoved && !item.value.isGroup) {
          controller.isMouseMoved = true;
          controller.updateHoveredIndex(index);
        }
      },
      onExit: (_) {
        if (!item.value.isGroup && controller.hoveredIndex.value == index) {
          controller.clearHoveredResult();
        }
      },
      child: GestureDetector(
        onTap: () {
          final traceId = const UuidV4().generate();
          controller.updateActiveIndex(traceId, index);
          onItemTapped?.call();
        },
        onDoubleTap: () {
          final traceId = const UuidV4().generate();
          controller.onItemExecuted?.call(traceId, item.value);
        },
        child: Obx(() => _buildGridItem(item.value, index, iconSize, showTitle, itemPadding, itemMargin)),
      ),
    );
  }

  Widget _buildGridItem(WoxListItem<WoxQueryResult> item, int index, double iconSize, bool showTitle, double itemPadding, double itemMargin) {
    final isActive = controller.activeIndex.value == index;
    final isHovered = controller.hoveredIndex.value == index;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          margin: EdgeInsets.all(itemMargin),
          padding: EdgeInsets.all(itemPadding),
          decoration: BoxDecoration(
            color: isActive
                ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemActiveBackgroundColor)
                : isHovered
                    ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemActiveBackgroundColor).withValues(alpha: 0.5)
                    : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
          ),
          child: WoxImageView(woxImage: item.icon, width: iconSize, height: iconSize),
        ),
        if (showTitle)
          Padding(
            padding: EdgeInsets.only(left: itemMargin, right: itemMargin),
            child: Text(
              item.title,
              style: TextStyle(fontSize: 12, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor)),
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
            ),
          ),
      ],
    );
  }
}
