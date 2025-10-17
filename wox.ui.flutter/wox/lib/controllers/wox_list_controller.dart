import 'package:flutter/material.dart';
import 'package:fuzzywuzzy/fuzzywuzzy.dart';
import 'package:get/get.dart';
import 'package:lpinyin/lpinyin.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxListController<T> extends GetxController {
  final List<WoxListItem<T>> _originalItems = []; // original items for filter and restore
  final RxList<Rx<WoxListItem<T>>> _items = <Rx<WoxListItem<T>>>[].obs;

  final RxInt _activeIndex = 0.obs;
  final RxInt _hoveredIndex = (-1).obs;

  final Function(String traceId, WoxListItem<T> item)? onItemExecuted;
  final Function(String traceId, WoxListItem<T> item)? onItemActive;
  final Function(String traceId)? onFilterBoxEscPressed;
  final Function(String traceId)? onFilterBoxLostFocus;

  /// This flag is used to control whether the item is selected by mouse hover.
  /// This is used to prevent the item from being selected when the mouse is just hovering over the item in the result list.
  var isMouseMoved = false;

  final ScrollController scrollController = ScrollController();
  final FocusNode filterBoxFocusNode = FocusNode();
  final TextEditingController filterBoxController = TextEditingController();

  WoxListController({
    this.onItemExecuted,
    this.onItemActive,
    this.onFilterBoxEscPressed,
    this.onFilterBoxLostFocus,
  });

  RxList<Rx<WoxListItem<T>>> get items => _items;

  RxInt get hoveredIndex => _hoveredIndex;

  RxInt get activeIndex => _activeIndex;

  WoxListItem<T> get activeItem => _items[_activeIndex.value].value;

  void updateActiveIndexByDirection(String traceId, WoxDirection direction) {
    Logger.instance.debug(traceId, "updateActiveIndexByDirection start, direction: $direction, current activeIndex: ${_activeIndex.value}");

    if (_items.isEmpty) {
      Logger.instance.debug(traceId, "updateActiveIndexByDirection: items list is empty");
      return;
    }

    var newIndex = _activeIndex.value;
    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      newIndex++;
      if (newIndex >= _items.length) {
        newIndex = 0;
      }

      int safetyCounter = 0;
      while (newIndex < _items.length && _items[newIndex].value.isGroup && safetyCounter < _items.length) {
        newIndex++;
        safetyCounter++;
        if (newIndex >= _items.length) {
          newIndex = 0;
          break;
        }
      }
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      newIndex--;
      if (newIndex < 0) {
        newIndex = _items.length - 1;
      }

      int safetyCounter = 0;
      while (newIndex >= 0 && _items[newIndex].value.isGroup && safetyCounter < _items.length) {
        newIndex--;
        safetyCounter++;
        if (newIndex < 0) {
          newIndex = _items.length - 1;
          break;
        }
      }
    }

    updateActiveIndex(traceId, newIndex);
  }

  void updateActiveIndex(String traceId, int index, {bool silent = false}) {
    if (index < 0 || index >= _items.length) {
      return;
    }

    Logger.instance.debug(traceId, "update active index, before: ${_activeIndex.value}, after: $index");

    _activeIndex.value = index;
    _syncScrollPositionWithActiveIndex(traceId);

    if (!silent) {
      onItemActive?.call(traceId, _items[index].value);
    }
  }

  void _syncScrollPositionWithActiveIndex(String traceId) {
    Logger.instance.debug(traceId, "sync ScrollPosition, current activeIndex: ${_activeIndex.value}");

    if (!scrollController.hasClients) {
      Logger.instance.debug(traceId, "ScrollController not attached to any scroll views yet");
      return;
    }

    final itemHeight = WoxThemeUtil.instance.getResultItemHeight();

    // If there are too few items, no need to scroll
    if (_items.isEmpty) {
      return;
    }

    // Calculate how many items can be displayed in the current viewport
    final viewportHeight = scrollController.position.viewportDimension;

    // If viewport height is 0 or invalid, skip scrolling (this can happen during initialization)
    if (viewportHeight <= 0) {
      Logger.instance.debug(traceId, "Invalid viewport height: $viewportHeight, skipping scroll sync");
      return;
    }

    final visibleItemCount = (viewportHeight / itemHeight).floor();

    // If all items can be displayed in the viewport, no need to scroll
    if (_items.length <= visibleItemCount) {
      return;
    }

    // Calculate the range of currently visible items
    final currentOffset = scrollController.offset;
    final firstVisibleItemIndex = (currentOffset / itemHeight).floor();
    final lastVisibleItemIndex = firstVisibleItemIndex + visibleItemCount - 1;

    Logger.instance.debug(traceId, "Visible range: $firstVisibleItemIndex - $lastVisibleItemIndex, active: ${_activeIndex.value}, visibleItemCount: $visibleItemCount");

    // If the active item is already in the visible range, no need to scroll
    if (_activeIndex.value >= firstVisibleItemIndex && _activeIndex.value <= lastVisibleItemIndex) {
      return;
    }

    // Find the group header before the active item
    int groupIndex = -1;
    if (_activeIndex.value > 0) {
      // Search backward from the active item for the nearest group header
      for (int i = _activeIndex.value - 1; i >= 0; i--) {
        if (_items[i].value.isGroup) {
          groupIndex = i;
          break;
        }
      }
    } else if (_activeIndex.value == 0 && _items.length > 1) {
      // If the active item is the first item, check if there are any group headers in the list
      for (int i = 0; i < _items.length; i++) {
        if (_items[i].value.isGroup) {
          // If a group header is found, set the scroll position to 0 to show the first item
          scrollController.jumpTo(0);
          return;
        }
      }
    }

    // If the active item is before the visible range, scroll up to make it visible
    if (_activeIndex.value < firstVisibleItemIndex) {
      // If there's a group header before the active item and it's close enough, show the group header
      if (groupIndex != -1 && _activeIndex.value - groupIndex <= 2) {
        // Scroll the group header to the top of the visible area
        final newOffset = groupIndex * itemHeight;
        scrollController.jumpTo(newOffset);
      } else {
        // Scroll the active item to the top of the visible area
        final newOffset = _activeIndex.value * itemHeight;
        scrollController.jumpTo(newOffset);
      }
      return;
    }

    // If the active item is after the visible range, scroll down to make it visible
    if (_activeIndex.value > lastVisibleItemIndex) {
      // Scroll the active item to the bottom of the visible area
      final newOffset = (_activeIndex.value - visibleItemCount + 1) * itemHeight;
      scrollController.jumpTo(newOffset);
      return;
    }
  }

  void updateItem(String traceId, WoxListItem<T> item) {
    // update original items
    final originalIndex = _originalItems.indexWhere((element) => element.id == item.id);
    if (originalIndex != -1) {
      _originalItems[originalIndex] = item;
    }

    // Check if there's an active filter
    bool hasActiveFilter = filterBoxController.text.isNotEmpty;
    if (hasActiveFilter) {
      // If there's an active filter, reapply it to ensure the updated item is properly filtered
      filterItems(traceId, filterBoxController.text);
    } else {
      // No filter active, update items directly
      final index = _items.indexWhere((element) => element.value.id == item.id);
      if (index != -1) {
        _items[index] = item.obs;
      }
    }
  }

  void updateItems(String traceId, List<WoxListItem<T>> newItems, {bool silent = false}) {
    _originalItems.assignAll(newItems);
    filterItems(traceId, filterBoxController.text, silent: silent);
  }

  void updateHoveredIndex(int index) {
    _hoveredIndex.value = index;
  }

  void clearHoveredResult() {
    _hoveredIndex.value = -1;
  }

  bool isItemActive(String itemId) {
    final index = _items.indexWhere((element) => element.value.id == itemId);
    return index != -1 && index == _activeIndex.value;
  }

  void requestFocus() {
    filterBoxFocusNode.requestFocus();
  }

  /// check if the query text is fuzzy match with the filter text based on the setting
  bool isFuzzyMatch(String traceId, String queryText, String filterText) {
    if (WoxSettingUtil.instance.currentSetting.usePinYin) {
      queryText = transferChineseToPinYin(queryText).toLowerCase();
    } else {
      queryText = queryText.toLowerCase();
    }

    var score = weightedRatio(queryText, filterText.toLowerCase());
    // Logger.instance.debug(traceId, "calculate fuzzy match score, queryText: $queryText, filterText: $filterText, score: $score");
    return score > 60;
  }

  String transferChineseToPinYin(String str) {
    RegExp regExp = RegExp(r'[\u4e00-\u9fa5]');
    if (regExp.hasMatch(str)) {
      return PinyinHelper.getPinyin(str, separator: "", format: PinyinFormat.WITHOUT_TONE);
    }
    return str;
  }

  void filterItems(String traceId, String filterText, {bool silent = false}) {
    if (filterText.isEmpty) {
      _items.assignAll(_originalItems.map((item) => item.obs));
    } else {
      // Find all non-group items that match the filter text
      var matchedItems = _originalItems.where((element) => !element.isGroup && isFuzzyMatch(traceId, element.title, filterText)).toList();

      // Find all items to include (matched items and their parent groups)
      var filteredItems = _findItemsToInclude(matchedItems);

      _items.assignAll(filteredItems.map((item) => item.obs));
    }
    updateActiveIndex(traceId, 0, silent: silent);
  }

  /// Find all items to include in the filtered list (matched items and their parent groups)
  List<WoxListItem<T>> _findItemsToInclude(List<WoxListItem<T>> matchedItems) {
    // Create a set of IDs of matched items for faster lookup
    final matchedItemIds = matchedItems.map((item) => item.id).toSet();

    // Create a map to track which groups have matching children
    final groupsWithMatchingChildren = <String, bool>{};

    // Track the current group for each item
    String? currentGroupId;

    // Single pass through the original items to find groups with matching children
    for (var item in _originalItems) {
      if (item.isGroup) {
        // This is a group item, set as current group
        currentGroupId = item.id;
      } else if (matchedItemIds.contains(item.id) && currentGroupId != null) {
        // This is a matched item under a group, mark the group
        groupsWithMatchingChildren[currentGroupId] = true;
      }
    }

    // Create the final filtered list with both matched items and their parent groups
    final result = <WoxListItem<T>>[];

    for (var item in _originalItems) {
      if (item.isGroup) {
        // Include group if it has matching children
        if (groupsWithMatchingChildren.containsKey(item.id)) {
          result.add(item);
        }
      } else if (matchedItemIds.contains(item.id)) {
        // Include all matched items
        result.add(item);
      }
    }

    return result;
  }

  void clearFilter(String traceId) {
    filterBoxController.clear();
    updateActiveIndex(traceId, 0);
  }

  void clearItems() {
    _items.clear();
    _originalItems.clear();
    _activeIndex.value = 0;
    _hoveredIndex.value = -1;
  }

  @override
  void onClose() {
    super.onClose();

    scrollController.dispose();
  }
}
