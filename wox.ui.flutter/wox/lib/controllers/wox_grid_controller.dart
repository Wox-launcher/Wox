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

  WoxGridController({super.onItemExecuted, super.onItemActive, super.onFilterBoxEscPressed, super.onFilterBoxLostFocus, super.onItemsEmpty});

  void updateGridParams(GridLayoutParams params) {
    gridLayoutParams = params;
  }

  void updateRowHeight(double height) {
    rowHeight = height;
  }

  /// Calculate total height needed for grid view content
  /// Returns the height for all rows and group headers, capped at maxRowCount
  double calculateGridHeight() {
    if (items.isEmpty || gridLayoutParams.columns <= 0 || rowHeight <= 0) {
      return 0;
    }

    const groupHeaderHeight = 32.0;
    double totalHeight = 0;
    int i = 0;

    while (i < items.length) {
      if (items[i].value.isGroup) {
        totalHeight += groupHeaderHeight;
        i++;
      } else {
        // Count items in this row (same group, up to columns count)
        final currentGroup = _getItemGroup(items[i].value);
        int itemsInRow = 0;
        while (i < items.length && !items[i].value.isGroup && _getItemGroup(items[i].value) == currentGroup && itemsInRow < gridLayoutParams.columns) {
          itemsInRow++;
          i++;
        }
        if (itemsInRow > 0) {
          totalHeight += rowHeight;
        }
      }
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
    const groupHeaderHeight = 32.0;

    double activeItemOffset = 0;
    double precedingGroupHeaderOffset = 0; // offset of the group header before active item's row
    int visualRowIndex = 0;
    bool found = false;

    int itemIndex = 0;
    while (itemIndex < items.length && !found) {
      if (items[itemIndex].value.isGroup) {
        precedingGroupHeaderOffset = activeItemOffset;
        activeItemOffset += groupHeaderHeight;
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

    if (activeItemOffset >= currentOffset && activeItemOffset + actualRowHeight <= currentOffset + viewportHeight) {
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
      newOffset = activeItemOffset + actualRowHeight - viewportHeight;
    }

    newOffset = newOffset.clamp(0.0, maxOffset);
    scrollController.jumpTo(newOffset);
  }
}
