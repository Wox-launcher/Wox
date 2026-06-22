import 'package:wox/controllers/wox_base_list_controller.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/utils/log.dart';

/// Controller for grid view. Handles grid-specific navigation and scrolling.
class WoxGridController<T> extends WoxBaseListController<T> {
  GridLayoutParams gridLayoutParams = GridLayoutParams.empty();

  // Row height calculated by the view based on available width and columns
  double rowHeight = 0;
  // Bug fix: grid scroll offsets need the same group-header height and final
  // spacer that the widget paints. Keeping them here avoids controller-only
  // constants drifting from density-scaled rendering.
  double groupHeaderHeight = 32.0;
  double viewportBottomPadding = 0.0;

  WoxGridController({super.onItemExecuted, super.onItemActive, super.onFilterBoxEscPressed, super.onFilterBoxLostFocus, super.onItemsEmpty});

  void updateGridParams(GridLayoutParams params) {
    gridLayoutParams = params;
  }

  bool updateLayoutMetrics({required double rowHeight, required double groupHeaderHeight, required double viewportBottomPadding}) {
    var changed = false;
    if ((this.rowHeight - rowHeight).abs() >= 0.5) {
      this.rowHeight = rowHeight;
      changed = true;
    }
    if ((this.groupHeaderHeight - groupHeaderHeight).abs() >= 0.5) {
      this.groupHeaderHeight = groupHeaderHeight;
      changed = true;
    }
    if ((this.viewportBottomPadding - viewportBottomPadding).abs() >= 0.5) {
      this.viewportBottomPadding = viewportBottomPadding;
      changed = true;
    }
    return changed;
  }

  /// Calculate total height needed for grid view content
  /// Returns the height for all rows and group headers, capped at maxRowCount
  double calculateGridHeight() {
    return _calculateGridHeightFor(items.length, (index) => items[index].value);
  }

  /// Bug fix: the launcher needs the next grid height before new results are
  /// committed. Estimating from the incoming snapshot lets the window grow
  /// before the first frame paints those results into the old height.
  double calculateGridHeightForItems(List<WoxListItem<T>> incomingItems) {
    return _calculateGridHeightFor(incomingItems.length, (index) => incomingItems[index]);
  }

  double _calculateGridHeightFor(int itemCount, WoxListItem<T> Function(int index) itemAt) {
    if (itemCount == 0 || gridLayoutParams.columns <= 0 || rowHeight <= 0) {
      return 0;
    }

    double totalHeight = 0;
    int i = 0;

    while (i < itemCount) {
      final item = itemAt(i);
      if (item.isGroup) {
        totalHeight += groupHeaderHeight;
        i++;
      } else {
        // Count items in this row (same group, up to columns count)
        final currentGroup = _getItemGroup(item);
        int itemsInRow = 0;
        while (i < itemCount && !itemAt(i).isGroup && _getItemGroup(itemAt(i)) == currentGroup && itemsInRow < gridLayoutParams.columns) {
          itemsInRow++;
          i++;
        }
        if (itemsInRow > 0) {
          totalHeight += rowHeight;
        }
      }
    }

    if (totalHeight > 0) {
      // Bug fix: grid rows can be scrolled to the bottom of a capped launcher
      // result area. The previous height model ended exactly at the final row,
      // so the toolbar separator could visually cover the active outline. The
      // view renders the same trailing spacer, keeping window height, scroll
      // extent, and painted content in one layout contract.
      totalHeight += viewportBottomPadding;
    }

    return totalHeight;
  }

  @override
  void updateActiveIndexByDirection(String traceId, WoxDirection direction) {
    Logger.instance.debug(traceId, "updateActiveIndexByDirection start, direction: $direction, columns: ${gridLayoutParams.columns}, current activeIndex: ${activeIndex.value}");

    if (items.isEmpty) {
      Logger.instance.debug(traceId, "updateActiveIndexByDirection: items list is empty");
      return;
    }

    var newIndex = activeIndex.value;

    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      newIndex = _findNextRowIndex(newIndex);
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      newIndex = _findPrevRowIndex(newIndex);
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_LEFT.code) {
      newIndex = findPrevNonGroupIndex(newIndex);
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_RIGHT.code) {
      newIndex = findNextNonGroupIndex(newIndex);
    }

    updateActiveIndex(traceId, newIndex);
  }

  /// Find the index in the next row at the same column position
  int _findNextRowIndex(int currentIndex) {
    final rows = _buildGridRows();
    if (rows.isEmpty) return currentIndex;

    int currentRow = -1;
    int currentCol = -1;
    for (int r = 0; r < rows.length; r++) {
      final colIndex = rows[r].indexOf(currentIndex);
      if (colIndex != -1) {
        currentRow = r;
        currentCol = colIndex;
        break;
      }
    }

    if (currentRow == -1) return currentIndex;

    int nextRow = currentRow + 1;
    if (nextRow >= rows.length) {
      nextRow = 0;
    }

    final nextRowItems = rows[nextRow];
    if (nextRowItems.isEmpty) return currentIndex;

    if (currentCol < nextRowItems.length) {
      return nextRowItems[currentCol];
    } else {
      return nextRowItems.last;
    }
  }

  /// Find the index in the previous row at the same column position
  int _findPrevRowIndex(int currentIndex) {
    final rows = _buildGridRows();
    if (rows.isEmpty) return currentIndex;

    int currentRow = -1;
    int currentCol = -1;
    for (int r = 0; r < rows.length; r++) {
      final colIndex = rows[r].indexOf(currentIndex);
      if (colIndex != -1) {
        currentRow = r;
        currentCol = colIndex;
        break;
      }
    }

    if (currentRow == -1) return currentIndex;

    int prevRow = currentRow - 1;
    if (prevRow < 0) {
      prevRow = rows.length - 1;
    }

    final prevRowItems = rows[prevRow];
    if (prevRowItems.isEmpty) return currentIndex;

    if (currentCol < prevRowItems.length) {
      return prevRowItems[currentCol];
    } else {
      return prevRowItems.last;
    }
  }

  /// Get group value from item data
  String _getItemGroup(WoxListItem<T> item) {
    if (item.data is WoxQueryResult) {
      return (item.data as WoxQueryResult).group;
    }
    return '';
  }

  /// Build grid rows considering group headers
  List<List<int>> _buildGridRows() {
    List<List<int>> rows = [];
    int i = 0;

    while (i < items.length) {
      if (items[i].value.isGroup) {
        i++;
      } else {
        List<int> rowIndices = [];
        final currentGroup = _getItemGroup(items[i].value);
        while (i < items.length && !items[i].value.isGroup && _getItemGroup(items[i].value) == currentGroup && rowIndices.length < gridLayoutParams.columns) {
          rowIndices.add(i);
          i++;
        }
        if (rowIndices.isNotEmpty) {
          rows.add(rowIndices);
        }
      }
    }

    return rows;
  }

  @override
  void syncScrollPositionWithActiveIndex(String traceId) {
    if (!scrollController.hasClients) {
      return;
    }

    if (items.isEmpty || gridLayoutParams.columns <= 0 || rowHeight <= 0) {
      return;
    }

    final viewportHeight = scrollController.position.viewportDimension;
    if (viewportHeight <= 0) {
      return;
    }

    final gridRows = _buildGridRows();
    final actualRowHeight = rowHeight;
    final actualGroupHeaderHeight = groupHeaderHeight;

    double activeItemOffset = 0;
    double precedingGroupHeaderOffset = 0; // offset of the group header before active item's row
    int visualRowIndex = 0;
    bool found = false;

    int itemIndex = 0;
    while (itemIndex < items.length && !found) {
      if (items[itemIndex].value.isGroup) {
        precedingGroupHeaderOffset = activeItemOffset;
        activeItemOffset += actualGroupHeaderHeight;
        itemIndex++;
      } else {
        if (visualRowIndex < gridRows.length) {
          final row = gridRows[visualRowIndex];
          if (row.contains(activeIndex.value)) {
            found = true;
            break;
          }
          precedingGroupHeaderOffset = -1; // reset, no group header directly before this row
          activeItemOffset += actualRowHeight;
          itemIndex = row.isNotEmpty ? row.last + 1 : itemIndex + 1;
          visualRowIndex++;
        } else {
          break;
        }
      }
    }

    if (!found) return;

    final currentOffset = scrollController.offset;
    final maxOffset = scrollController.position.maxScrollExtent;
    final activeRowTrailingPadding = visualRowIndex == gridRows.length - 1 ? viewportBottomPadding : 0.0;
    final activeItemBottom = activeItemOffset + actualRowHeight + activeRowTrailingPadding;

    if (activeItemOffset >= currentOffset && activeItemBottom <= currentOffset + viewportHeight) {
      return;
    }

    double newOffset;
    if (activeItemOffset < currentOffset) {
      // Scrolling up: if there's a group header right before this row, show it too
      if (precedingGroupHeaderOffset >= 0) {
        newOffset = precedingGroupHeaderOffset;
      } else {
        newOffset = activeItemOffset;
      }
    } else {
      // Bug fix: when keyboard navigation wraps to the final grid row, scroll
      // far enough to include the same trailing spacer used by the rendered
      // scroll view. Without it the last active row was mathematically visible
      // but painted flush under the toolbar divider.
      newOffset = activeItemBottom - viewportHeight;
    }

    newOffset = newOffset.clamp(0.0, maxOffset);
    scrollController.jumpTo(newOffset);
  }
}
