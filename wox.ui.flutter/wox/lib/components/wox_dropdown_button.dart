import 'package:chinese_font_library/chinese_font_library.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/utils/colors.dart';

/// Data model for dropdown items with optional tooltip
class WoxDropdownItem<T> {
  final T value;
  final String label;
  final String? tooltip;
  final Widget? leading;
  final bool isSelectAll;

  const WoxDropdownItem({required this.value, required this.label, this.tooltip, this.leading, this.isSelectAll = false});
}

/// Wox dropdown button with theme-aware styling
class WoxDropdownButton<T> extends StatefulWidget {
  final List<WoxDropdownItem<T>> items;
  final T? value;
  final ValueChanged<T?>? onChanged;
  final bool isExpanded;
  final double fontSize;
  final Color? dropdownColor;
  final double? menuMaxHeight;
  final Widget? hint;
  final Widget? icon;
  final double? iconSize;
  final AlignmentGeometry alignment;
  final double? itemHeight;
  final double? width;
  final Widget? underline;
  final bool enableFilter;
  final String? filterHintText;
  final FocusNode? focusNode;
  final bool autofocus;
  final bool multiSelect;
  final List<T> multiValues;
  final ValueChanged<List<T>>? onMultiChanged;

  const WoxDropdownButton({
    super.key,
    required this.items,
    required this.value,
    required this.onChanged,
    this.isExpanded = true,
    this.fontSize = 13,
    this.dropdownColor,
    this.menuMaxHeight,
    this.hint,
    this.icon,
    this.iconSize,
    this.alignment = AlignmentDirectional.centerStart,
    this.itemHeight,
    this.width,
    this.underline,
    this.enableFilter = false,
    this.filterHintText,
    this.focusNode,
    this.autofocus = false,
    this.multiSelect = false,
    this.multiValues = const [],
    this.onMultiChanged,
  });

  @override
  State<WoxDropdownButton<T>> createState() => _WoxDropdownButtonState<T>();
}

class _WoxDropdownButtonState<T> extends State<WoxDropdownButton<T>> {
  final TextEditingController _filterController = TextEditingController();
  final FocusNode _filterFocusNode = FocusNode();
  final LayerLink _layerLink = LayerLink();
  OverlayEntry? _overlayEntry;
  List<WoxDropdownItem<T>> _filteredItems = [];
  List<T> _multiValues = [];

  Widget _buildNoRippleInkWell({required Widget child, VoidCallback? onTap}) {
    return InkWell(
      onTap: onTap,
      splashFactory: NoSplash.splashFactory,
      overlayColor: WidgetStateProperty.all(Colors.transparent),
      splashColor: Colors.transparent,
      highlightColor: Colors.transparent,
      hoverColor: Colors.transparent,
      child: child,
    );
  }

  Color _getDropdownBackgroundColor() {
    final baseDropdownBg = widget.dropdownColor ?? getThemeActiveBackgroundColor();
    return baseDropdownBg.withAlpha(255);
  }

  double _contrastRatio(Color foreground, Color background) {
    final foregroundLuminance = foreground.computeLuminance();
    final backgroundLuminance = background.computeLuminance();
    final bright = foregroundLuminance > backgroundLuminance ? foregroundLuminance : backgroundLuminance;
    final dark = foregroundLuminance > backgroundLuminance ? backgroundLuminance : foregroundLuminance;
    return (bright + 0.05) / (dark + 0.05);
  }

  Color _getReadableTextColor(Color background) {
    const darkText = Colors.black87;
    const lightText = Colors.white;
    return _contrastRatio(lightText, background) >= _contrastRatio(darkText, background) ? lightText : darkText;
  }

  void _markOverlayNeedsBuildSafely() {
    if (_overlayEntry == null) {
      return;
    }

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _overlayEntry == null) {
        return;
      }
      _overlayEntry!.markNeedsBuild();
    });
  }

  @override
  void initState() {
    super.initState();
    _filteredItems = widget.items;
    _multiValues = _normalizeMultiValues(widget.multiValues);
  }

  @override
  void didUpdateWidget(WoxDropdownButton<T> oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.items != widget.items) {
      _filteredItems = widget.items;
      _filterController.clear();
    }

    if (oldWidget.items != widget.items || !_isSameMultiValues(widget.multiValues, _multiValues)) {
      _multiValues = _normalizeMultiValues(widget.multiValues);
      _markOverlayNeedsBuildSafely();
    }
  }

  bool _isSameMultiValues(List<T> left, List<T> right) {
    if (identical(left, right)) {
      return true;
    }

    if (left.length != right.length) {
      return false;
    }

    for (var index = 0; index < left.length; index++) {
      if (left[index] != right[index]) {
        return false;
      }
    }

    return true;
  }

  WoxDropdownItem<T>? _getSelectAllItem() {
    for (final item in widget.items) {
      if (item.isSelectAll) {
        return item;
      }
    }
    return null;
  }

  List<T> _getRegularValues() {
    final regularValues = <T>[];
    for (final item in widget.items) {
      if (!item.isSelectAll) {
        regularValues.add(item.value);
      }
    }
    return regularValues;
  }

  bool _isSelectAllSelected([List<T>? values]) {
    final selectAllItem = _getSelectAllItem();
    if (selectAllItem == null) {
      return false;
    }

    final selectedValues = values ?? _multiValues;
    return selectedValues.contains(selectAllItem.value);
  }

  bool _isMultiMenuItemSelected(WoxDropdownItem<T> item) {
    if (item.isSelectAll) {
      return _isSelectAllSelected();
    }
    if (_isSelectAllSelected()) {
      return true;
    }

    return _multiValues.contains(item.value);
  }

  List<T> _normalizeMultiValues(List<T> values) {
    final normalized = <T>[];
    for (final item in widget.items) {
      if (values.contains(item.value) && !normalized.contains(item.value)) {
        normalized.add(item.value);
      }
    }

    final selectAllItem = _getSelectAllItem();
    if (selectAllItem == null) {
      return normalized;
    }

    if (normalized.contains(selectAllItem.value)) {
      return [selectAllItem.value];
    }

    final regularValues = _getRegularValues();
    if (regularValues.isNotEmpty && normalized.length == regularValues.length) {
      return [selectAllItem.value];
    }

    return normalized;
  }

  @override
  void dispose() {
    _removeOverlay();
    _filterController.dispose();
    _filterFocusNode.dispose();
    super.dispose();
  }

  void _filterItems(String query) {
    setState(() {
      if (query.isEmpty) {
        _filteredItems = widget.items;
      } else {
        _filteredItems =
            widget.items.where((item) {
              return item.label.toLowerCase().contains(query.toLowerCase());
            }).toList();
      }
    });
    // Rebuild overlay with filtered items
    if (_overlayEntry != null) {
      _markOverlayNeedsBuildSafely();
    }
  }

  void _removeOverlay() {
    _overlayEntry?.remove();
    _overlayEntry = null;
    _filterController.clear();
    _filteredItems = widget.items;
  }

  void _showFilterableMenu() {
    final activeTextColor = getThemeActiveTextColor();
    final dropdownBg = _getDropdownBackgroundColor();
    final searchBg =
        dropdownBg.computeLuminance() > 0.45
            ? Color.alphaBlend(Colors.black.withValues(alpha: 0.08), dropdownBg)
            : Color.alphaBlend(Colors.white.withValues(alpha: 0.08), dropdownBg);
    final searchTextColor = _getReadableTextColor(searchBg);
    final searchHintColor = searchTextColor.withValues(alpha: 0.55);
    final searchDividerColor = searchTextColor.withValues(alpha: 0.20);
    final borderColor = getThemeSubTextColor();

    final RenderBox renderBox = context.findRenderObject() as RenderBox;
    final size = renderBox.size;

    _overlayEntry = OverlayEntry(
      builder:
          (context) => GestureDetector(
            behavior: HitTestBehavior.translucent,
            onTap: _removeOverlay,
            child: Stack(
              children: [
                Positioned(
                  width: size.width,
                  child: CompositedTransformFollower(
                    link: _layerLink,
                    showWhenUnlinked: false,
                    offset: Offset(0, size.height),
                    child: Material(
                      elevation: 8,
                      borderRadius: BorderRadius.circular(4),
                      color: dropdownBg,
                      child: Container(
                        clipBehavior: Clip.antiAlias,
                        constraints: BoxConstraints(maxHeight: widget.menuMaxHeight ?? 300),
                        decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            // Filter text field
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                              decoration: BoxDecoration(
                                color: searchBg,
                                border: Border(bottom: BorderSide(color: searchDividerColor)),
                                borderRadius: const BorderRadius.vertical(top: Radius.circular(4)),
                              ),
                              child: Focus(
                                onKeyEvent: (node, event) {
                                  if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
                                    _removeOverlay();
                                    return KeyEventResult.handled;
                                  }
                                  return KeyEventResult.ignored;
                                },
                                child: TextField(
                                  controller: _filterController,
                                  focusNode: _filterFocusNode,
                                  autofocus: true,
                                  textAlignVertical: TextAlignVertical.center,
                                  style: TextStyle(color: searchTextColor, fontSize: widget.fontSize).useSystemChineseFont(),
                                  decoration: InputDecoration(
                                    hintText: widget.filterHintText ?? 'Filter...',
                                    hintStyle: TextStyle(color: searchHintColor, fontSize: widget.fontSize).useSystemChineseFont(),
                                    border: InputBorder.none,
                                    isDense: true,
                                    contentPadding: const EdgeInsets.symmetric(vertical: 8),
                                    prefixIcon: Padding(padding: const EdgeInsets.only(left: 4, right: 6), child: Icon(Icons.search, size: 16, color: searchHintColor)),
                                    prefixIconConstraints: const BoxConstraints(minWidth: 22, minHeight: 22),
                                  ),
                                  onChanged: _filterItems,
                                ),
                              ),
                            ),
                            // Filtered items list
                            Flexible(
                              child: Container(
                                color: dropdownBg,
                                child: ListView.builder(
                                  shrinkWrap: true,
                                  padding: EdgeInsets.zero,
                                  itemCount: _filteredItems.length,
                                  itemBuilder: (context, index) {
                                    final item = _filteredItems[index];
                                    final isSelected = item.value == widget.value;
                                    return _buildNoRippleInkWell(
                                      onTap: () {
                                        widget.onChanged?.call(item.value);
                                        _removeOverlay();
                                      },
                                      child: Container(
                                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                                        color: isSelected ? activeTextColor.withValues(alpha: 0.1) : null,
                                        child: DefaultTextStyle(
                                          style: TextStyle(color: activeTextColor, fontSize: widget.fontSize).useSystemChineseFont(),
                                          child: _buildDropdownMenuItem(item, activeTextColor),
                                        ),
                                      ),
                                    );
                                  },
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
    );

    Overlay.of(context).insert(_overlayEntry!);
    _filterFocusNode.requestFocus();
  }

  void _toggleMultiValue(T value) {
    final selectAllItem = _getSelectAllItem();
    List<T> selectedValues;

    if (selectAllItem != null && value == selectAllItem.value) {
      selectedValues = _isSelectAllSelected() ? <T>[] : <T>[selectAllItem.value];
    } else {
      selectedValues =
          _isSelectAllSelected()
              ? List<T>.from(_getRegularValues())
              : _multiValues.where((selectedValue) => selectAllItem == null || selectedValue != selectAllItem.value).toList();

      if (selectedValues.contains(value)) {
        selectedValues.remove(value);
      } else {
        selectedValues.add(value);
      }
    }

    final ordered = _normalizeMultiValues(selectedValues);
    setState(() {
      _multiValues = ordered;
    });
    widget.onMultiChanged?.call(ordered);

    _markOverlayNeedsBuildSafely();
  }

  void _showMultiSelectMenu() {
    final activeTextColor = getThemeActiveTextColor();
    final dropdownBg = _getDropdownBackgroundColor();
    final borderColor = getThemeSubTextColor();

    final RenderBox renderBox = context.findRenderObject() as RenderBox;
    final size = renderBox.size;

    _overlayEntry = OverlayEntry(
      builder:
          (context) => GestureDetector(
            behavior: HitTestBehavior.translucent,
            onTap: _removeOverlay,
            child: Stack(
              children: [
                Positioned(
                  width: size.width,
                  child: CompositedTransformFollower(
                    link: _layerLink,
                    showWhenUnlinked: false,
                    offset: Offset(0, size.height),
                    child: Material(
                      elevation: 8,
                      borderRadius: BorderRadius.circular(4),
                      color: dropdownBg,
                      child: Container(
                        constraints: BoxConstraints(maxHeight: widget.menuMaxHeight ?? 300),
                        decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
                        child: ListView.builder(
                          shrinkWrap: true,
                          padding: EdgeInsets.zero,
                          itemCount: widget.items.length,
                          itemBuilder: (context, index) {
                            final item = widget.items[index];
                            final isSelected = _isMultiMenuItemSelected(item);
                            return _buildNoRippleInkWell(
                              onTap: () => _toggleMultiValue(item.value),
                              child: Container(
                                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                                color: isSelected ? activeTextColor.withValues(alpha: 0.1) : null,
                                child: Row(
                                  children: [
                                    WoxCheckbox(value: isSelected, onChanged: (_) => _toggleMultiValue(item.value), size: 20),
                                    const SizedBox(width: 8),
                                    Expanded(
                                      child: DefaultTextStyle(
                                        style: TextStyle(color: activeTextColor, fontSize: widget.fontSize).useSystemChineseFont(),
                                        child: _buildDropdownMenuItem(item, activeTextColor),
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            );
                          },
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
    );

    Overlay.of(context).insert(_overlayEntry!);
  }

  KeyEventResult _handleFilterTriggerKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) {
      return KeyEventResult.ignored;
    }

    if (widget.onChanged == null) {
      return KeyEventResult.ignored;
    }

    final key = event.logicalKey;
    if (key == LogicalKeyboardKey.enter || key == LogicalKeyboardKey.space || key == LogicalKeyboardKey.arrowDown) {
      _showFilterableMenu();
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  KeyEventResult _handleMultiTriggerKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) {
      return KeyEventResult.ignored;
    }

    if (widget.onMultiChanged == null) {
      return KeyEventResult.ignored;
    }

    final key = event.logicalKey;
    if (key == LogicalKeyboardKey.enter || key == LogicalKeyboardKey.space || key == LogicalKeyboardKey.arrowDown) {
      _showMultiSelectMenu();
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  // Build dropdown menu item with optional tooltip icon
  Widget _buildDropdownMenuItem(WoxDropdownItem<T> item, Color activeTextColor) {
    final hasLeading = item.leading != null;
    final hasTooltip = item.tooltip != null && item.tooltip!.isNotEmpty;
    if (!hasLeading && !hasTooltip) {
      return Text(item.label);
    }

    return Row(
      children: [
        if (hasLeading) ...[SizedBox(width: 18, height: 18, child: item.leading!), const SizedBox(width: 8)],
        Expanded(child: Text(item.label)),
        if (hasTooltip) Tooltip(message: item.tooltip!, child: Icon(Icons.info_outline, size: 16, color: activeTextColor)),
      ],
    );
  }

  // Build selected item (without tooltip icon)
  Widget _buildSelectedItem(WoxDropdownItem<T> item, Color textColor) {
    return Align(
      alignment: widget.alignment,
      child: Row(
        children: [
          if (item.leading != null) ...[SizedBox(width: 18, height: 18, child: item.leading!), const SizedBox(width: 8)],
          Expanded(child: Text(item.label, style: TextStyle(color: textColor, fontSize: widget.fontSize).useSystemChineseFont())),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final activeTextColor = getThemeActiveTextColor();
    final dropdownBg = _getDropdownBackgroundColor();
    final borderColor = getThemeSubTextColor();

    if (widget.multiSelect) {
      final selectedItems =
          _isSelectAllSelected() ? widget.items.where((item) => item.isSelectAll).toList() : widget.items.where((item) => _multiValues.contains(item.value)).toList();
      final selectedText = selectedItems.map((item) => item.label).join(", ");

      return CompositedTransformTarget(
        link: _layerLink,
        child: SizedBox(
          width: widget.width ?? 300.0,
          child: Container(
            decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
            child: Focus(
              focusNode: widget.focusNode,
              autofocus: widget.autofocus,
              onKeyEvent: _handleMultiTriggerKey,
              child: _buildNoRippleInkWell(
                onTap: widget.onMultiChanged != null ? _showMultiSelectMenu : null,
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          selectedText.isNotEmpty ? selectedText : "",
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(color: textColor, fontSize: widget.fontSize).useSystemChineseFont(),
                        ),
                      ),
                      Icon(Icons.arrow_drop_down, color: widget.onMultiChanged != null ? textColor : textColor.withValues(alpha: 0.5), size: widget.iconSize ?? 24.0),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      );
    }

    if (!widget.enableFilter) {
      // Convert WoxDropdownItem to DropdownMenuItem
      final dropdownMenuItems =
          widget.items.map((item) {
            return DropdownMenuItem<T>(value: item.value, child: _buildDropdownMenuItem(item, activeTextColor));
          }).toList();

      // Original non-filterable dropdown
      final dropdown = Theme(
        data: Theme.of(context).copyWith(splashFactory: NoSplash.splashFactory, splashColor: Colors.transparent, highlightColor: Colors.transparent),
        child: DropdownButtonHideUnderline(
          child: DropdownButton<T>(
            items: dropdownMenuItems,
            value: widget.value,
            onChanged: widget.onChanged,
            focusNode: widget.focusNode,
            autofocus: widget.autofocus,
            isExpanded: widget.isExpanded,
            style: TextStyle(color: activeTextColor, fontSize: widget.fontSize).useSystemChineseFont(),
            selectedItemBuilder: (BuildContext context) {
              return widget.items.map<Widget>((item) {
                return _buildSelectedItem(item, textColor);
              }).toList();
            },
            dropdownColor: dropdownBg,
            iconEnabledColor: textColor,
            iconDisabledColor: textColor.withValues(alpha: 0.5),
            hint: widget.hint,
            icon: widget.icon,
            iconSize: widget.iconSize ?? 24.0,
            menuMaxHeight: widget.menuMaxHeight,
            alignment: widget.alignment,
            itemHeight: widget.itemHeight,
            underline: widget.underline ?? const SizedBox.shrink(),
            isDense: true,
            padding: EdgeInsets.zero,
          ),
        ),
      );

      return SizedBox(
        width: widget.width ?? 300.0,
        child: Container(
          decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
          child: Padding(padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0), child: dropdown),
        ),
      );
    }

    // Filterable dropdown with custom overlay
    final selectedItem = widget.items.firstWhere((item) => item.value == widget.value, orElse: () => widget.items.first);

    return CompositedTransformTarget(
      link: _layerLink,
      child: SizedBox(
        width: widget.width ?? 300.0,
        child: Container(
          decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
          child: Focus(
            focusNode: widget.focusNode,
            autofocus: widget.autofocus,
            onKeyEvent: _handleFilterTriggerKey,
            child: _buildNoRippleInkWell(
              onTap: widget.onChanged != null ? _showFilterableMenu : null,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
                child: Row(
                  children: [
                    Expanded(
                      child: DefaultTextStyle(
                        style: TextStyle(color: textColor, fontSize: widget.fontSize).useSystemChineseFont(),
                        child: widget.value != null ? Text(selectedItem.label) : (widget.hint ?? const SizedBox.shrink()),
                      ),
                    ),
                    Icon(Icons.arrow_drop_down, color: widget.onChanged != null ? textColor : textColor.withValues(alpha: 0.5), size: widget.iconSize ?? 24.0),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
