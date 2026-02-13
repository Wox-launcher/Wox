import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/utils/wox_fuzzy_match_util.dart';
import 'package:wox/utils/wox_setting_util.dart';

/// Base controller for list-like views (list and grid).
/// Contains shared logic for items management, active/hovered index, filtering, etc.
abstract class WoxBaseListController<T> extends GetxController {
  final List<WoxListItem<T>> _originalItems = [];
  final RxList<Rx<WoxListItem<T>>> _items = <Rx<WoxListItem<T>>>[].obs;

  final RxInt _activeIndex = 0.obs;
  final RxInt _hoveredIndex = (-1).obs;

  final Function(String traceId, WoxListItem<T> item)? onItemExecuted;
  final Function(String traceId, WoxListItem<T> item)? onItemActive;
  final Function(String traceId)? onFilterBoxEscPressed;
  final Function(String traceId)? onFilterBoxLostFocus;
  final Function(String traceId)? onItemsEmpty;

  /// Controls whether item is selected by mouse hover.
  /// Prevents selection when mouse is just hovering over the result list.
  var isMouseMoved = false;

  final ScrollController scrollController = ScrollController();
  final FocusNode filterBoxFocusNode = FocusNode();
  final TextEditingController filterBoxController = TextEditingController();

  WoxBaseListController({this.onItemExecuted, this.onItemActive, this.onFilterBoxEscPressed, this.onFilterBoxLostFocus, this.onItemsEmpty});

  @override
  void onInit() {
    super.onInit();
    filterBoxFocusNode.addListener(_onFilterBoxFocusChange);
  }

  void _onFilterBoxFocusChange() {
    if (!filterBoxFocusNode.hasFocus) {
      onFilterBoxLostFocus?.call('focus_change');
    }
  }

  RxList<Rx<WoxListItem<T>>> get items => _items;
  List<WoxListItem<T>> get originalItems => _originalItems;
  RxInt get hoveredIndex => _hoveredIndex;
  RxInt get activeIndex => _activeIndex;
  WoxListItem<T> get activeItem => _items[_activeIndex.value].value;

  String buildItemUpdateId(int index) => 'list_item_$index';

  void refreshItemByIndex(int index) {
    if (index < 0 || index >= _items.length) {
      return;
    }
    update([buildItemUpdateId(index)]);
  }

  /// Abstract method: subclasses implement direction-based navigation
  void updateActiveIndexByDirection(String traceId, WoxDirection direction);

  /// Abstract method: subclasses implement scroll sync logic
  void syncScrollPositionWithActiveIndex(String traceId);

  void updateActiveIndex(String traceId, int index, {bool silent = false}) {
    if (index < 0 || index >= _items.length) {
      return;
    }

    final previousIndex = _activeIndex.value;
    _activeIndex.value = index;
    if (previousIndex != index) {
      refreshItemByIndex(previousIndex);
      refreshItemByIndex(index);
    }
    syncScrollPositionWithActiveIndex(traceId);

    if (!silent) {
      onItemActive?.call(traceId, _items[index].value);
    }
  }

  void updateItem(String traceId, WoxListItem<T> item) {
    final originalIndex = _originalItems.indexWhere((element) => element.id == item.id);
    if (originalIndex != -1) {
      _originalItems[originalIndex] = item;
    }

    bool hasActiveFilter = filterBoxController.text.isNotEmpty;
    if (hasActiveFilter) {
      filterItems(traceId, filterBoxController.text);
    } else {
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
    if (index < 0 || index >= _items.length) {
      return;
    }
    if (_hoveredIndex.value == index) {
      return;
    }

    final previousIndex = _hoveredIndex.value;
    _hoveredIndex.value = index;
    refreshItemByIndex(previousIndex);
    refreshItemByIndex(index);
  }

  void clearHoveredResult() {
    if (_hoveredIndex.value == -1) {
      return;
    }
    final previousIndex = _hoveredIndex.value;
    _hoveredIndex.value = -1;
    refreshItemByIndex(previousIndex);
  }

  bool isItemActive(String itemId) {
    final index = _items.indexWhere((element) => element.value.id == itemId);
    return index != -1 && index == _activeIndex.value;
  }

  void requestFocus() {
    filterBoxFocusNode.requestFocus();
  }

  bool isFuzzyMatch(String queryText, String filterText) {
    return WoxFuzzyMatchUtil.isFuzzyMatch(text: queryText, pattern: filterText, usePinYin: WoxSettingUtil.instance.currentSetting.usePinYin);
  }

  void filterItems(String traceId, String filterText, {bool silent = false}) {
    if (filterText.isEmpty) {
      _items.assignAll(_originalItems.map((item) => item.obs));
    } else {
      var matchedItems = _originalItems.where((element) => !element.isGroup && isFuzzyMatch(element.title, filterText)).toList();
      var filteredItems = _findItemsToInclude(matchedItems);
      _items.assignAll(filteredItems.map((item) => item.obs));
    }

    if (_items.isEmpty) {
      onItemsEmpty?.call(traceId);
    } else {
      updateActiveIndex(traceId, 0, silent: silent);
    }
  }

  List<WoxListItem<T>> _findItemsToInclude(List<WoxListItem<T>> matchedItems) {
    final matchedItemIds = matchedItems.map((item) => item.id).toSet();
    final groupsWithMatchingChildren = <String, bool>{};

    String? currentGroupId;
    for (var item in _originalItems) {
      if (item.isGroup) {
        currentGroupId = item.id;
      } else if (matchedItemIds.contains(item.id) && currentGroupId != null) {
        groupsWithMatchingChildren[currentGroupId] = true;
      }
    }

    final result = <WoxListItem<T>>[];
    for (var item in _originalItems) {
      if (item.isGroup) {
        if (groupsWithMatchingChildren.containsKey(item.id)) {
          result.add(item);
        }
      } else if (matchedItemIds.contains(item.id)) {
        result.add(item);
      }
    }

    return result;
  }

  /// Find the next non-group item index (for left navigation)
  int findPrevNonGroupIndex(int currentIndex) {
    var newIndex = currentIndex - 1;
    if (newIndex < 0) {
      newIndex = _items.length - 1;
    }
    int safetyCounter = 0;
    while (_items[newIndex].value.isGroup && safetyCounter < _items.length) {
      newIndex--;
      safetyCounter++;
      if (newIndex < 0) {
        newIndex = _items.length - 1;
      }
    }
    return newIndex;
  }

  /// Find the next non-group item index (for right navigation)
  int findNextNonGroupIndex(int currentIndex) {
    var newIndex = currentIndex + 1;
    if (newIndex >= _items.length) {
      newIndex = 0;
    }
    int safetyCounter = 0;
    while (_items[newIndex].value.isGroup && safetyCounter < _items.length) {
      newIndex++;
      safetyCounter++;
      if (newIndex >= _items.length) {
        newIndex = 0;
      }
    }
    return newIndex;
  }

  void clearFilter(String traceId) {
    filterBoxController.clear();
    filterItems(traceId, '');
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
    filterBoxFocusNode.removeListener(_onFilterBoxFocusChange);
    scrollController.dispose();
  }
}
