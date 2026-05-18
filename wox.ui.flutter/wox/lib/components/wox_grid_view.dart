import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_grid_controller.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxGridView extends StatelessWidget {
  final WoxGridController<WoxQueryResult> controller;
  final GridLayoutParams gridLayoutParams;
  final double maxHeight;
  final VoidCallback? onItemTapped;
  final void Function(String traceId, WoxListItem<WoxQueryResult> item)? onItemSecondaryTapped;
  final VoidCallback? onRowHeightChanged;

  static const double focusFrameWidth = 4.0;
  static const double viewportBottomPadding = focusFrameWidth;

  const WoxGridView({
    super.key,
    required this.controller,
    required this.gridLayoutParams,
    required this.maxHeight,
    this.onItemTapped,
    this.onItemSecondaryTapped,
    this.onRowHeightChanged,
  });

  void _scrollByPointerDelta(double deltaY) {
    if (!controller.scrollController.hasClients) {
      return;
    }

    final position = controller.scrollController.position;
    final targetOffset = (position.pixels + deltaY).clamp(position.minScrollExtent, position.maxScrollExtent).toDouble();
    if ((targetOffset - position.pixels).abs() < 0.01) {
      return;
    }

    // Grid results share the same pointer-scroll contract as list results. The previous
    // selection-step handling ignored pointer distance, so using the raw offset preserves
    // smooth native scrolling while keyboard and click selection still drive active items.
    controller.scrollController.jumpTo(targetOffset);
  }

  void _handlePointerSignal(PointerSignalEvent event) {
    if (event is PointerScrollEvent) {
      _scrollByPointerDelta(event.scrollDelta.dy);
    }
  }

  // Handle pinch-zoom scroll events for trackpads and touchscreens. The PointerPanZoomUpdateEvent provides a panDelta that represents the scroll distance, which we can use to scroll the grid view accordingly.
  void _handlePointerPanZoomUpdate(PointerPanZoomUpdateEvent event) {
    _scrollByPointerDelta(-event.panDelta.dy);
  }

  @override
  Widget build(BuildContext context) {
    final columns = gridLayoutParams.columns;
    final showTitle = gridLayoutParams.showTitle;
    final itemPadding = gridLayoutParams.itemPadding;
    final itemMargin = gridLayoutParams.itemMargin;
    final aspectRatio = gridLayoutParams.aspectRatio;
    final metrics = WoxInterfaceSizeUtil.instance.metrics.value;

    return LayoutBuilder(
      builder: (context, constraints) {
        // Feature change: grid items are no longer always square. The old
        // icon-only math made wallpaper thumbnails float inside large square
        // cells, so the row height now comes from the declared visual ratio
        // while the default 1.0 ratio preserves emoji and app grids.
        final availableWidth = constraints.maxWidth;
        final cellWidth = columns > 0 ? (availableWidth / columns).floorToDouble() : 48.0;
        final contentWidth = (cellWidth - (itemPadding + itemMargin) * 2).clamp(1.0, double.infinity).toDouble();
        final contentHeight = contentWidth / aspectRatio;
        // Cell height includes visual content + padding/margin, and title height if showing title.
        final titleHeight = metrics.gridTitleHeight;
        final cellHeight = contentHeight + (itemPadding + itemMargin) * 2 + (showTitle ? titleHeight : 0);

        // Bug fix: the controller owns window height and keyboard scroll math,
        // while this widget owns the actual rendered grid. Keep row height,
        // group-header height, and the final paint spacer synchronized here so
        // the last row is not hidden by the toolbar when the grid reaches its
        // capped result height.
        final layoutMetricsChanged = controller.updateLayoutMetrics(
          rowHeight: cellHeight,
          groupHeaderHeight: metrics.gridGroupHeaderHeight,
          viewportBottomPadding: viewportBottomPadding,
        );
        if (layoutMetricsChanged) {
          WidgetsBinding.instance.addPostFrameCallback((_) => onRowHeightChanged?.call());
        }

        return ConstrainedBox(
          constraints: BoxConstraints(maxHeight: maxHeight),
          child: Scrollbar(
            thumbVisibility: true,
            controller: controller.scrollController,
            child: Listener(
              onPointerSignal: _handlePointerSignal,
              onPointerPanZoomUpdate: _handlePointerPanZoomUpdate,
              child: Obx(() => _buildGridWithGroups(cellHeight, contentWidth, contentHeight, columns, showTitle, itemPadding, itemMargin)),
            ),
          ),
        );
      },
    );
  }

  Widget _buildGridWithGroups(double cellSize, double contentWidth, double contentHeight, int columns, bool showTitle, double itemPadding, double itemMargin) {
    final items = controller.items;
    // Always keep scrollController attached so Scrollbar never loses its ScrollPosition.
    if (items.isEmpty) return SingleChildScrollView(controller: controller.scrollController, child: const SizedBox.shrink());

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
        rows.add(_buildGridRow(rowIndices, cellSize, contentWidth, contentHeight, showTitle, columns, itemPadding, itemMargin));
      }
    }

    rows.add(const SizedBox(height: viewportBottomPadding));

    return SingleChildScrollView(
      controller: controller.scrollController,
      physics: const NeverScrollableScrollPhysics(),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: rows),
    );
  }

  Widget _buildGroupHeader(WoxListItem<WoxQueryResult> item, int index) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return SizedBox(
      height: metrics.gridGroupHeaderHeight,
      child: Padding(
        padding: EdgeInsets.only(left: metrics.gridGroupHeaderPaddingLeft, top: metrics.gridGroupHeaderPaddingTop, bottom: metrics.gridGroupHeaderPaddingBottom),
        // Bug fix: the grid controller uses this fixed density-aware header
        // height for both total-height and active-row offset calculations. The
        // previous controller-side 32px constant could drift from rendered
        // padding/font metrics and leave bottom rows slightly under-scrolled.
        child: Text(
          item.title,
          style: TextStyle(
            fontSize: metrics.gridGroupHeaderFontSize,
            fontWeight: FontWeight.w500,
            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
          ),
          overflow: TextOverflow.ellipsis,
          maxLines: 1,
        ),
      ),
    );
  }

  Widget _buildGridRow(List<int> indices, double cellSize, double contentWidth, double contentHeight, bool showTitle, int columns, double itemPadding, double itemMargin) {
    return Row(
      children: [
        for (int i = 0; i < columns; i++)
          Expanded(
            child:
                i < indices.length
                    ? SizedBox(height: cellSize, child: _buildGridItemWidget(indices[i], contentWidth, contentHeight, showTitle, itemPadding, itemMargin))
                    : SizedBox(height: cellSize),
          ),
      ],
    );
  }

  Widget _buildGridItemWidget(int index, double contentWidth, double contentHeight, bool showTitle, double itemPadding, double itemMargin) {
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
        onTapDown: (_) {
          final traceId = const UuidV4().generate();
          // Bug fix: grid selection used to wait for onTap, which fires after
          // Flutter accepts the full tap gesture. List rows activate on tap
          // down, so selecting grid cells on tap down gives both layouts the
          // same immediate active-state feedback while keeping double-click
          // execution on the existing onDoubleTap path.
          controller.updateActiveIndex(traceId, index);
          onItemTapped?.call();
        },
        onDoubleTap: () {
          final traceId = const UuidV4().generate();
          controller.onItemExecuted?.call(traceId, item.value);
        },
        onSecondaryTapDown: (_) {
          final traceId = const UuidV4().generate();
          controller.updateActiveIndex(traceId, index);
          controller.onItemActive?.call(traceId, item.value);
          onItemSecondaryTapped?.call(traceId, item.value);
        },
        child: Obx(() => _buildGridItem(item.value, index, contentWidth, contentHeight, showTitle, itemPadding, itemMargin)),
      ),
    );
  }

  Widget _buildGridItem(WoxListItem<WoxQueryResult> item, int index, double contentWidth, double contentHeight, bool showTitle, double itemPadding, double itemMargin) {
    final isActive = controller.activeIndex.value == index;
    final isHovered = controller.hoveredIndex.value == index;
    final activeColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemActiveBackgroundColor);
    final frameColor =
        isActive
            ? activeColor
            : isHovered
            // Keep grid hover visually below active selection even when the
            // theme active token is translucent, as glass themes use low-alpha
            // active colors that were previously overwritten by a fixed hover alpha.
            ? getHoverColorFromActiveColor(activeColor)
            : Colors.transparent;
    const frameWidth = focusFrameWidth;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          margin: EdgeInsets.all(itemMargin),
          padding: EdgeInsets.all(itemPadding),
          // Bug fix: the focus frame is paint-only and must not be part of the
          // item box model. The previous version reserved focusFrameWidth in
          // layout, so ItemPadding=0 still looked padded and emoji thumbnails
          // shrank when selection moved. Painting the frame outside the padded
          // content keeps ItemPadding as the only content gap while preserving a
          // stable active/hover outline.
          child: Stack(
            clipBehavior: Clip.none,
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(6),
                child: WoxImageView(
                  woxImage: item.icon,
                  width: contentWidth,
                  height: contentHeight,
                  fit: (gridLayoutParams.aspectRatio - 1.0).abs() < 0.01 ? BoxFit.contain : BoxFit.cover,
                ),
              ),
              Positioned(
                left: -itemPadding,
                top: -itemPadding,
                right: -itemPadding,
                bottom: -itemPadding,
                child: IgnorePointer(
                  child: DecoratedBox(
                    decoration: ShapeDecoration(
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                        side: BorderSide(color: frameColor, width: frameWidth, strokeAlign: BorderSide.strokeAlignOutside),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        if (showTitle)
          Padding(
            padding: EdgeInsets.only(left: itemMargin, right: itemMargin),
            child: Text(
              item.title,
              // Grid result captions are launcher result text, so they follow
              // the density font bucket while grid image ratios and theme frame
              // styling remain unchanged.
              style: TextStyle(
                fontSize: WoxInterfaceSizeUtil.instance.current.gridItemTitleFontSize,
                color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor),
              ),
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
            ),
          ),
      ],
    );
  }
}
