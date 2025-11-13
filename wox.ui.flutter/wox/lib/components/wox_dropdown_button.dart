import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Data model for dropdown items with optional tooltip
class WoxDropdownItem<T> {
  final T value;
  final String label;
  final String? tooltip;

  const WoxDropdownItem({
    required this.value,
    required this.label,
    this.tooltip,
  });
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

  @override
  void initState() {
    super.initState();
    _filteredItems = widget.items;
  }

  @override
  void didUpdateWidget(WoxDropdownButton<T> oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.items != widget.items) {
      _filteredItems = widget.items;
      _filterController.clear();
    }
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
        _filteredItems = widget.items.where((item) {
          return item.label.toLowerCase().contains(query.toLowerCase());
        }).toList();
      }
    });
    // Rebuild overlay with filtered items
    if (_overlayEntry != null) {
      _overlayEntry!.markNeedsBuild();
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
    final dropdownBg = widget.dropdownColor ?? getThemeActiveBackgroundColor().withAlpha(255);
    final borderColor = getThemeSubTextColor();

    final RenderBox renderBox = context.findRenderObject() as RenderBox;
    final size = renderBox.size;

    _overlayEntry = OverlayEntry(
      builder: (context) => GestureDetector(
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
                    constraints: BoxConstraints(
                      maxHeight: widget.menuMaxHeight ?? 300,
                    ),
                    decoration: BoxDecoration(
                      border: Border.all(color: borderColor),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        // Filter text field
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                          decoration: BoxDecoration(
                            border: Border(
                              bottom: BorderSide(color: borderColor),
                            ),
                          ),
                          child: TextField(
                            controller: _filterController,
                            focusNode: _filterFocusNode,
                            autofocus: true,
                            style: TextStyle(color: activeTextColor, fontSize: widget.fontSize),
                            decoration: InputDecoration(
                              hintText: 'Filter...',
                              hintStyle: TextStyle(color: activeTextColor.withValues(alpha: 0.5), fontSize: widget.fontSize),
                              border: InputBorder.none,
                              isDense: true,
                              contentPadding: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
                              prefixIcon: Icon(Icons.search, size: 16, color: activeTextColor.withValues(alpha: 0.7)),
                            ),
                            onChanged: _filterItems,
                          ),
                        ),
                        // Filtered items list
                        Flexible(
                          child: ListView.builder(
                            shrinkWrap: true,
                            padding: EdgeInsets.zero,
                            itemCount: _filteredItems.length,
                            itemBuilder: (context, index) {
                              final item = _filteredItems[index];
                              final isSelected = item.value == widget.value;
                              return InkWell(
                                onTap: () {
                                  widget.onChanged?.call(item.value);
                                  _removeOverlay();
                                },
                                child: Container(
                                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                                  color: isSelected ? activeTextColor.withValues(alpha: 0.1) : null,
                                  child: DefaultTextStyle(
                                    style: TextStyle(color: activeTextColor, fontSize: widget.fontSize),
                                    child: _buildDropdownMenuItem(item, activeTextColor),
                                  ),
                                ),
                              );
                            },
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

  // Build dropdown menu item with optional tooltip icon
  Widget _buildDropdownMenuItem(WoxDropdownItem<T> item, Color activeTextColor) {
    if (item.tooltip == null || item.tooltip!.isEmpty) {
      return Text(item.label);
    }

    return Row(
      children: [
        Expanded(child: Text(item.label)),
        Tooltip(
          message: item.tooltip!,
          child: Icon(Icons.info_outline, size: 16, color: activeTextColor),
        ),
      ],
    );
  }

  // Build selected item (without tooltip icon)
  Widget _buildSelectedItem(WoxDropdownItem<T> item, Color textColor) {
    return Align(
      alignment: widget.alignment,
      child: Text(item.label, style: TextStyle(color: textColor, fontSize: widget.fontSize)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final activeTextColor = getThemeActiveTextColor();
    final dropdownBg = widget.dropdownColor ?? getThemeActiveBackgroundColor().withAlpha(255);
    final borderColor = getThemeSubTextColor();

    if (!widget.enableFilter) {
      // Convert WoxDropdownItem to DropdownMenuItem
      final dropdownMenuItems = widget.items.map((item) {
        return DropdownMenuItem<T>(
          value: item.value,
          child: _buildDropdownMenuItem(item, activeTextColor),
        );
      }).toList();

      // Original non-filterable dropdown
      final dropdown = DropdownButtonHideUnderline(
        child: DropdownButton<T>(
          items: dropdownMenuItems,
          value: widget.value,
          onChanged: widget.onChanged,
          isExpanded: widget.isExpanded,
          style: TextStyle(color: activeTextColor, fontSize: widget.fontSize),
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
      );

      return SizedBox(
        width: widget.width ?? 300.0,
        child: Container(
          decoration: BoxDecoration(
            border: Border.all(color: borderColor),
            borderRadius: BorderRadius.circular(4),
          ),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
            child: dropdown,
          ),
        ),
      );
    }

    // Filterable dropdown with custom overlay
    final selectedItem = widget.items.firstWhere(
      (item) => item.value == widget.value,
      orElse: () => widget.items.first,
    );

    return CompositedTransformTarget(
      link: _layerLink,
      child: SizedBox(
        width: widget.width ?? 300.0,
        child: Container(
          decoration: BoxDecoration(
            border: Border.all(color: borderColor),
            borderRadius: BorderRadius.circular(4),
          ),
          child: InkWell(
            onTap: widget.onChanged != null ? _showFilterableMenu : null,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
              child: Row(
                children: [
                  Expanded(
                    child: DefaultTextStyle(
                      style: TextStyle(color: textColor, fontSize: widget.fontSize),
                      child: widget.value != null ? Text(selectedItem.label) : (widget.hint ?? const SizedBox.shrink()),
                    ),
                  ),
                  Icon(
                    Icons.arrow_drop_down,
                    color: widget.onChanged != null ? textColor : textColor.withValues(alpha: 0.5),
                    size: widget.iconSize ?? 24.0,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
