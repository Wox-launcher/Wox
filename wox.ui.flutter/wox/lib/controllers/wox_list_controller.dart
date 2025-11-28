import 'package:wox/controllers/wox_base_list_controller.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

/// Controller for list view. Handles list-specific navigation and scrolling.
class WoxListController<T> extends WoxBaseListController<T> {
  WoxListController({
    super.onItemExecuted,
    super.onItemActive,
    super.onFilterBoxEscPressed,
    super.onFilterBoxLostFocus,
    super.onItemsEmpty,
  });

  @override
  void updateActiveIndexByDirection(String traceId, WoxDirection direction) {
    Logger.instance.debug(traceId, "updateActiveIndexByDirection start, direction: $direction, current activeIndex: ${activeIndex.value}");

    if (items.isEmpty) {
      Logger.instance.debug(traceId, "updateActiveIndexByDirection: items list is empty");
      return;
    }

    var newIndex = activeIndex.value;

    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      newIndex++;
      if (newIndex >= items.length) {
        newIndex = 0;
      }

      int safetyCounter = 0;
      while (newIndex < items.length && items[newIndex].value.isGroup && safetyCounter < items.length) {
        newIndex++;
        safetyCounter++;
        if (newIndex >= items.length) {
          newIndex = 0;
          break;
        }
      }
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      newIndex--;
      if (newIndex < 0) {
        newIndex = items.length - 1;
      }

      int safetyCounter = 0;
      while (newIndex >= 0 && items[newIndex].value.isGroup && safetyCounter < items.length) {
        newIndex--;
        safetyCounter++;
        if (newIndex < 0) {
          newIndex = items.length - 1;
          break;
        }
      }
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_LEFT.code) {
      newIndex = findPrevNonGroupIndex(newIndex);
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_RIGHT.code) {
      newIndex = findNextNonGroupIndex(newIndex);
    }

    updateActiveIndex(traceId, newIndex);
  }

  @override
  void syncScrollPositionWithActiveIndex(String traceId) {
    Logger.instance.debug(traceId, "sync ScrollPosition, current activeIndex: ${activeIndex.value}");

    if (!scrollController.hasClients) {
      Logger.instance.debug(traceId, "ScrollController not attached to any scroll views yet");
      return;
    }

    if (items.isEmpty) {
      return;
    }

    final itemHeight = WoxThemeUtil.instance.getResultItemHeight();
    final viewportHeight = scrollController.position.viewportDimension;

    if (viewportHeight <= 0) {
      Logger.instance.debug(traceId, "Invalid viewport height: $viewportHeight, skipping scroll sync");
      return;
    }

    final visibleItemCount = (viewportHeight / itemHeight).floor();

    if (items.length <= visibleItemCount) {
      return;
    }

    final currentOffset = scrollController.offset;
    final firstVisibleItemIndex = (currentOffset / itemHeight).floor();
    final lastVisibleItemIndex = firstVisibleItemIndex + visibleItemCount - 1;

    Logger.instance.debug(traceId, "Visible range: $firstVisibleItemIndex - $lastVisibleItemIndex, active: ${activeIndex.value}, visibleItemCount: $visibleItemCount");

    if (activeIndex.value >= firstVisibleItemIndex && activeIndex.value <= lastVisibleItemIndex) {
      return;
    }

    // Find the group header before the active item
    int groupIndex = -1;
    if (activeIndex.value > 0) {
      for (int i = activeIndex.value - 1; i >= 0; i--) {
        if (items[i].value.isGroup) {
          groupIndex = i;
          break;
        }
      }
    } else if (activeIndex.value == 0 && items.length > 1) {
      for (int i = 0; i < items.length; i++) {
        if (items[i].value.isGroup) {
          scrollController.jumpTo(0);
          return;
        }
      }
    }

    if (activeIndex.value < firstVisibleItemIndex) {
      if (groupIndex != -1 && activeIndex.value - groupIndex <= 2) {
        final newOffset = groupIndex * itemHeight;
        scrollController.jumpTo(newOffset);
      } else {
        final newOffset = activeIndex.value * itemHeight;
        scrollController.jumpTo(newOffset);
      }
      return;
    }

    if (activeIndex.value > lastVisibleItemIndex) {
      final newOffset = (activeIndex.value - visibleItemCount + 1) * itemHeight;
      scrollController.jumpTo(newOffset);
      return;
    }
  }
}
