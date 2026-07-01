import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_app_selector.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_window_manager.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxWindowManagerGroupsSetting extends StatefulWidget {
  final String value;
  final double labelWidth;
  final Future<String?> Function(String key, String value) onUpdate;

  const WoxWindowManagerGroupsSetting({super.key, required this.value, required this.labelWidth, required this.onUpdate});

  @override
  State<WoxWindowManagerGroupsSetting> createState() => _WoxWindowManagerGroupsSettingState();
}

class _WoxWindowManagerGroupsSettingState extends State<WoxWindowManagerGroupsSetting> {
  static const String _settingKey = 'windowGroups';

  late final WoxSettingController controller;

  List<WindowManagerDisplay> _displays = <WindowManagerDisplay>[];
  bool _isLoadingDisplays = false;
  String _displayError = '';

  String tr(String key) => controller.tr(key);

  @override
  void initState() {
    super.initState();
    controller = Get.find<WoxSettingController>();
    _loadDisplays();
  }

  Future<void> _loadDisplays() async {
    final traceId = const UuidV4().generate();
    setState(() {
      _isLoadingDisplays = true;
      _displayError = '';
    });

    try {
      final displays = await WoxApi.instance.getWindowManagerDisplays(traceId);
      if (!mounted) {
        return;
      }
      setState(() {
        _displays = displays;
        _isLoadingDisplays = false;
      });
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to load window manager displays: $e');
      if (!mounted) {
        return;
      }
      setState(() {
        _isLoadingDisplays = false;
        _displayError = e.toString();
      });
    }
  }

  Future<void> _ensureDisplaysLoaded() async {
    while (_isLoadingDisplays && mounted) {
      await Future.delayed(const Duration(milliseconds: 50));
    }

    if (_displays.isNotEmpty) {
      return;
    }

    await _loadDisplays();
  }

  _WindowManagerGroup _newGroup() {
    final group = _WindowManagerGroup(id: const UuidV4().generate(), name: '', screens: <_WindowManagerGroupScreen>[]);
    _populateGroupScreens(group);
    return group;
  }

  void _populateGroupScreens(_WindowManagerGroup group) {
    for (final entry in _displays.asMap().entries) {
      group.screenFor(entry.value, entry.key);
    }
  }

  Future<void> _openCreateGroupDialog(
    BuildContext dialogContext,
    Future<String?> Function(Map<String, dynamic> row) saveRow, {
    Map<String, dynamic> initialRow = const <String, dynamic>{},
  }) async {
    await _ensureDisplaysLoaded();
    if (!dialogContext.mounted) {
      return;
    }

    final group = initialRow.isEmpty ? _newGroup() : _WindowManagerGroup.fromJson(initialRow).copy();
    if (group.id.trim().isEmpty) {
      group.id = const UuidV4().generate();
    }
    _populateGroupScreens(group);

    final saved = await _showGroupDialog(dialogContext, group, isEditing: false);
    if (saved == null) {
      return;
    }

    await saveRow(saved.toJson());
  }

  Future<void> _openEditGroupDialog(BuildContext dialogContext, Map<String, dynamic> row, Future<String?> Function(Map<String, dynamic> row) saveRow) async {
    await _ensureDisplaysLoaded();
    if (!dialogContext.mounted) {
      return;
    }

    final group = _WindowManagerGroup.fromJson(row).copy();
    _populateGroupScreens(group);

    final saved = await _showGroupDialog(dialogContext, group, isEditing: true);
    if (saved == null) {
      return;
    }

    await saveRow(saved.toJson());
  }

  Future<_WindowManagerGroup?> _showGroupDialog(BuildContext dialogContext, _WindowManagerGroup group, {required bool isEditing}) async {
    return await showDialog<_WindowManagerGroup>(
      context: dialogContext,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _WindowGroupDialog(group: group, displays: _displays, tr: tr, isEditing: isEditing),
    );
  }

  @override
  Widget build(BuildContext context) {
    return WoxSettingPluginTable(
      value: widget.value,
      item: _buildTableDefinition(),
      labelWidth: widget.labelWidth,
      showCloneAction: false,
      titleActions: [_buildDemoTitleAction()],
      trailingActions: _buildTrailingActions(),
      customCreateDialogBuilder: _openCreateGroupDialog,
      customEditDialogBuilder: _openEditGroupDialog,
      customCellBuilder: _buildGroupCell,
      onUpdate: widget.onUpdate,
    );
  }

  Widget _buildDemoTitleAction() {
    final foreground = getThemeTextColor();

    return WoxDemoPopover(
      key: const ValueKey('window-manager-layouts-demo-trigger'),
      popoverKey: const ValueKey('wox-demo-popover-window-manager-layouts'),
      demo: WoxWindowManagerLayoutsDemo(accent: const Color(0xFF14B8A6), tr: tr),
      width: 700,
      height: 460,
      child: Semantics(
        label: tr('ui_demo_preview'),
        button: true,
        child: MouseRegion(
          cursor: SystemMouseCursors.help,
          child: SizedBox(width: 22, height: 22, child: Icon(Icons.play_circle_outline_rounded, color: foreground.withValues(alpha: 0.88), size: 18)),
        ),
      ),
    );
  }

  PluginSettingValueTable _buildTableDefinition() {
    return PluginSettingValueTable.fromJson(<String, dynamic>{
      'Key': _settingKey,
      'Title': tr('plugin_window_manager_setting_groups'),
      'Tooltip': 'i18n:plugin_window_manager_setting_groups_tooltip',
      'MaxHeight': 240,
      'Columns': <Map<String, dynamic>>[
        {
          'Key': 'Name',
          'Label': 'i18n:plugin_window_manager_group_name',
          'Tooltip': '',
          'Width': 0,
          'Type': PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          'TextMaxLines': 1,
        },
        {
          'Key': 'AppCount',
          'Label': 'i18n:plugin_window_manager_group_app_count',
          'Tooltip': '',
          'Width': 90,
          'Type': PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          'TextMaxLines': 1,
        },
        {
          'Key': 'DisplayCount',
          'Label': 'i18n:plugin_window_manager_group_display_count',
          'Tooltip': '',
          'Width': 90,
          'Type': PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          'TextMaxLines': 1,
        },
      ],
    });
  }

  List<Widget> _buildTrailingActions() {
    if (_isLoadingDisplays) {
      return [Text(tr('plugin_window_manager_group_loading_displays'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))];
    }

    if (_displayError.isEmpty) {
      return const <Widget>[];
    }

    return [
      WoxButton.secondary(text: tr('plugin_window_manager_group_retry'), height: 30, padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6), onPressed: _loadDisplays),
    ];
  }

  Widget? _buildGroupCell(PluginSettingValueTableColumn column, Map<String, dynamic> row) {
    final group = _WindowManagerGroup.fromJson(row);
    final textStyle = TextStyle(overflow: TextOverflow.ellipsis, color: getThemeTextColor(), fontSize: 13);

    return switch (column.key) {
      'Name' => Text(group.name.trim().isEmpty ? group.id : group.name, maxLines: 1, overflow: TextOverflow.ellipsis, style: textStyle.copyWith(fontWeight: FontWeight.w600)),
      'AppCount' => Text('${group.appCount}', maxLines: 1, overflow: TextOverflow.ellipsis, style: textStyle),
      'DisplayCount' => Text('${group.screens.length}', maxLines: 1, overflow: TextOverflow.ellipsis, style: textStyle),
      _ => null,
    };
  }
}

class _WindowGroupDialog extends StatefulWidget {
  final _WindowManagerGroup group;
  final List<WindowManagerDisplay> displays;
  final String Function(String key) tr;
  final bool isEditing;

  const _WindowGroupDialog({required this.group, required this.displays, required this.tr, required this.isEditing});

  @override
  State<_WindowGroupDialog> createState() => _WindowGroupDialogState();
}

class _WindowGroupDialogState extends State<_WindowGroupDialog> {
  static const List<int> _slotCounts = <int>[1, 2, 3, 4];

  late final TextEditingController _nameController;
  late _WindowManagerGroup _group;
  int _selectedDisplayIndex = 0;
  List<IgnoredHotkeyApp> _availableApps = <IgnoredHotkeyApp>[];
  String _nameError = '';

  String tr(String key) => widget.tr(key);

  @override
  void initState() {
    super.initState();
    _group = widget.group.copy();
    _nameController = TextEditingController(text: _group.name.trim());
    _nameController.addListener(() {
      _group.name = _nameController.text;
      if (_nameError.isNotEmpty && _nameController.text.trim().isNotEmpty) {
        setState(() {
          _nameError = '';
        });
      }
    });
    if (widget.displays.isNotEmpty) {
      _selectedDisplayIndex = 0;
      for (final entry in widget.displays.asMap().entries) {
        _group.screenFor(entry.value, entry.key);
      }
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  WindowManagerDisplay? get _selectedDisplay {
    if (widget.displays.isEmpty) {
      return null;
    }
    final index = _selectedDisplayIndex.clamp(0, widget.displays.length - 1).toInt();
    return widget.displays[index];
  }

  _WindowManagerGroupScreen? get _selectedScreen {
    final display = _selectedDisplay;
    if (display == null) {
      return null;
    }
    return _group.screenFor(display, _selectedDisplayIndex);
  }

  _WindowGroupLayout get _selectedLayout {
    final screen = _selectedScreen;
    return _WindowGroupLayouts.byId(screen?.layout ?? _WindowGroupLayouts.fullId);
  }

  Future<List<IgnoredHotkeyApp>> _loadAvailableApps() async {
    if (_availableApps.isNotEmpty) {
      return _availableApps;
    }

    final traceId = const UuidV4().generate();
    try {
      _availableApps = await WoxApi.instance.getHotkeyAppCandidates(traceId);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to load window group app candidates: $e');
      _availableApps = <IgnoredHotkeyApp>[];
    }
    return _availableApps;
  }

  Future<void> _openSlotAppSelector(_WindowGroupSlot slot) async {
    final screen = _selectedScreen;
    if (screen == null) {
      return;
    }

    final current = screen.assignmentFor(slot.id)?.app ?? IgnoredHotkeyApp.empty();
    final apps = await _loadAvailableApps();
    if (!mounted) {
      return;
    }

    final selectedApp = await showWoxAppSelectorDialog(
      context: context,
      selectedApp: current,
      initialApps: _mergeSelectedApp(current, apps),
      loadApps: () async => _mergeSelectedApp(current, await _loadAvailableApps()),
    );
    if (!mounted || selectedApp == null) {
      return;
    }

    setState(() {
      _setSlotApp(slot.id, selectedApp);
    });
  }

  void _setSlotApp(String slotId, IgnoredHotkeyApp app) {
    final identity = app.identity.trim().toLowerCase();
    if (identity.isNotEmpty) {
      for (final screen in _group.screens) {
        screen.assignments.removeWhere((assignment) => assignment.app.identity.trim().toLowerCase() == identity);
      }
    }

    final screen = _selectedScreen;
    if (screen == null) {
      return;
    }
    screen.assignments.removeWhere((assignment) => assignment.slot == slotId);
    if (identity.isNotEmpty) {
      screen.assignments.add(_WindowManagerAssignment(slot: slotId, app: app));
    }
  }

  List<IgnoredHotkeyApp> _mergeSelectedApp(IgnoredHotkeyApp selectedApp, List<IgnoredHotkeyApp> apps) {
    final merged = <IgnoredHotkeyApp>[];
    final seen = <String>{};
    final selectedIdentity = selectedApp.identity.trim().toLowerCase();
    if (selectedIdentity.isNotEmpty) {
      merged.add(selectedApp);
      seen.add(selectedIdentity);
    }
    for (final app in apps) {
      final identity = app.identity.trim().toLowerCase();
      if (identity.isEmpty || seen.contains(identity)) {
        continue;
      }
      seen.add(identity);
      merged.add(app);
    }
    return merged;
  }

  void _setSelectedLayout(_WindowGroupLayout layout) {
    final screen = _selectedScreen;
    if (screen == null) {
      return;
    }

    setState(() {
      screen.layout = layout.id;
      screen.assignments.removeWhere((assignment) => !layout.slots.any((slot) => slot.id == assignment.slot));
    });
  }

  void _submit() {
    final name = _nameController.text.trim();
    if (name.isEmpty) {
      setState(() {
        _nameError = tr('plugin_window_manager_group_name_required');
      });
      return;
    }

    _group.name = name;
    Navigator.pop(context, _group);
  }

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    return WoxDialog(
      title: Text(
        tr(widget.isEditing ? 'plugin_window_manager_group_edit_dialog_title' : 'plugin_window_manager_group_create_dialog_title'),
        style: TextStyle(color: textColor, fontSize: 16),
      ),
      titleTextStyle: TextStyle(color: textColor, fontSize: 16),
      insetPadding: const EdgeInsets.symmetric(horizontal: 28, vertical: 24),
      content: SizedBox(
        width: 1040,
        height: 650,
        child: Column(
          children: [
            Row(
              children: [
                SizedBox(
                  width: 360,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      WoxTextField(
                        controller: _nameController,
                        hintText: tr('plugin_window_manager_group_name'),
                        width: double.infinity,
                        contentPadding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
                      ),
                      if (_nameError.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 6), child: Text(_nameError, style: const TextStyle(color: Colors.red, fontSize: 12))),
                    ],
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(child: Text(tr('plugin_window_manager_group_select_display'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))),
              ],
            ),
            const SizedBox(height: 14),
            Expanded(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [Expanded(flex: 11, child: _buildDisplayArrangement()), const SizedBox(width: 18), SizedBox(width: 330, child: _buildLayoutPanel())],
              ),
            ),
          ],
        ),
      ),
      actions: [
        WoxButton.secondary(text: tr('ui_cancel'), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: () => Navigator.pop(context)),
        WoxButton.primary(text: tr('ui_save'), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: _submit),
      ],
    );
  }

  Widget _buildDisplayArrangement() {
    if (widget.displays.isEmpty) {
      return Container(
        decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)), borderRadius: BorderRadius.circular(6)),
        alignment: Alignment.center,
        child: Text(tr('plugin_window_manager_group_no_displays'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
      );
    }

    final rects = widget.displays.map(_displayRect).toList();
    final minX = rects.map((rect) => rect.left).reduce(math.min);
    final minY = rects.map((rect) => rect.top).reduce(math.min);
    final maxX = rects.map((rect) => rect.right).reduce(math.max);
    final maxY = rects.map((rect) => rect.bottom).reduce(math.max);
    final desktopWidth = math.max(1.0, maxX - minX);
    final desktopHeight = math.max(1.0, maxY - minY);

    return Container(
      decoration: BoxDecoration(
        color: getThemeSubTextColor().withValues(alpha: 0.06),
        border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)),
        borderRadius: BorderRadius.circular(6),
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          const padding = 24.0;
          final scale =
              math.min((constraints.maxWidth - padding * 2) / desktopWidth, (constraints.maxHeight - padding * 2) / desktopHeight).clamp(0.01, double.infinity).toDouble();
          final contentWidth = desktopWidth * scale;
          final contentHeight = desktopHeight * scale;
          final offsetX = (constraints.maxWidth - contentWidth) / 2;
          final offsetY = (constraints.maxHeight - contentHeight) / 2;

          return Stack(
            children: [
              for (final entry in widget.displays.asMap().entries)
                Positioned(
                  left: offsetX + (rects[entry.key].left - minX) * scale,
                  top: offsetY + (rects[entry.key].top - minY) * scale,
                  width: math.max(90.0, rects[entry.key].width * scale),
                  height: math.max(58.0, rects[entry.key].height * scale),
                  child: _buildDisplayTile(entry.value, entry.key),
                ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildDisplayTile(WindowManagerDisplay display, int displayIndex) {
    final selected = displayIndex == _selectedDisplayIndex;
    final layout = selected ? _selectedLayout : _WindowGroupLayouts.byId(_group.screenFor(display, displayIndex).layout);
    final screen = _group.screenFor(display, displayIndex);

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () => setState(() => _selectedDisplayIndex = displayIndex),
        child: Container(
          decoration: BoxDecoration(
            color: selected ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.26 : 0.18) : getThemePopupSurfaceColor(),
            border: Border.all(color: selected ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.4), width: selected ? 2 : 1),
            borderRadius: BorderRadius.circular(6),
          ),
          child: Stack(
            children: [
              Positioned.fill(
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    return Stack(
                      children:
                          layout.slots.map((slot) {
                            final assignment = screen.assignmentFor(slot.id);
                            return Positioned(
                              left: slot.left * constraints.maxWidth,
                              top: slot.top * constraints.maxHeight,
                              width: slot.width * constraints.maxWidth,
                              height: slot.height * constraints.maxHeight,
                              child: Padding(padding: const EdgeInsets.all(3), child: _buildSlotTile(slot, assignment, selected, displayIndex)),
                            );
                          }).toList(),
                    );
                  },
                ),
              ),
              if (display.isPrimary)
                Positioned(right: 8, top: 6, child: Text(tr('plugin_window_manager_group_display_primary'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 10))),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSlotTile(_WindowGroupSlot slot, _WindowManagerAssignment? assignment, bool selectedDisplay, int displayIndex) {
    final assignedApp = assignment?.app;
    final hasAssignment = assignedApp != null && assignedApp.identity.trim().isNotEmpty;
    final appName = hasAssignment && assignedApp.name.trim().isNotEmpty ? assignedApp.name.trim() : '';

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: selectedDisplay ? () => _openSlotAppSelector(slot) : () => setState(() => _selectedDisplayIndex = displayIndex),
        child: Container(
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(horizontal: 8),
          decoration: BoxDecoration(
            color:
                hasAssignment
                    ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.28 : 0.2)
                    : getThemeTextColor().withValues(alpha: isThemeDark() ? 0.035 : 0.055),
            border: Border.all(color: hasAssignment ? getThemeActiveBackgroundColor().withValues(alpha: 0.55) : getThemeSubTextColor().withValues(alpha: 0.38)),
            borderRadius: BorderRadius.circular(4),
          ),
          child:
              hasAssignment
                  ? Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (assignedApp.icon.imageData.isNotEmpty)
                        ClipRRect(borderRadius: BorderRadius.circular(4), child: WoxImageView(woxImage: assignedApp.icon, width: 18, height: 18))
                      else
                        Icon(Icons.apps_rounded, size: 18, color: getThemeTextColor().withValues(alpha: 0.78)),
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(appName, style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600), maxLines: 1, overflow: TextOverflow.ellipsis),
                      ),
                    ],
                  )
                  : Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.add_circle_outline_rounded, size: 15, color: getThemeSubTextColor()),
                      const SizedBox(width: 5),
                      Flexible(
                        child: Text(
                          tr('plugin_window_manager_group_slot_choose_app'),
                          style: TextStyle(color: getThemeSubTextColor(), fontSize: 11),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
        ),
      ),
    );
  }

  Widget _buildLayoutPanel() {
    return Container(
      decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)), borderRadius: BorderRadius.circular(6)),
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(tr('plugin_window_manager_group_layouts'), style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600)),
            const SizedBox(height: 4),
            Text(tr('plugin_window_manager_group_layouts_description'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 11, height: 1.35)),
            const SizedBox(height: 12),
            for (final count in _slotCounts) _buildLayoutGroup(count),
          ],
        ),
      ),
    );
  }

  Widget _buildLayoutGroup(int slotCount) {
    final layouts = _WindowGroupLayouts.bySlotCount(slotCount);
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(tr('plugin_window_manager_group_slot_count').replaceAll('{count}', '$slotCount'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          const SizedBox(height: 8),
          Wrap(spacing: 8, runSpacing: 8, children: layouts.map(_buildLayoutCard).toList()),
        ],
      ),
    );
  }

  Widget _buildLayoutCard(_WindowGroupLayout layout) {
    final selected = _selectedLayout.id == layout.id;
    return InkWell(
      borderRadius: BorderRadius.circular(4),
      onTap: () => _setSelectedLayout(layout),
      child: Container(
        width: 142,
        height: 82,
        padding: const EdgeInsets.all(7),
        decoration: BoxDecoration(
          color: selected ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.22 : 0.12) : Colors.transparent,
          borderRadius: BorderRadius.circular(4),
          border: Border.all(color: selected ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.35)),
        ),
        child: Column(
          children: [
            Expanded(child: _buildMiniLayout(layout)),
            const SizedBox(height: 5),
            Text(tr(layout.titleKey), style: TextStyle(color: getThemeTextColor(), fontSize: 10), maxLines: 1, overflow: TextOverflow.ellipsis),
          ],
        ),
      ),
    );
  }

  Widget _buildMiniLayout(_WindowGroupLayout layout) {
    return Container(
      decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.45)), borderRadius: BorderRadius.circular(3)),
      child: LayoutBuilder(
        builder: (context, constraints) {
          return Stack(
            children:
                layout.slots.map((slot) {
                  return Positioned(
                    left: slot.left * constraints.maxWidth,
                    top: slot.top * constraints.maxHeight,
                    width: slot.width * constraints.maxWidth,
                    height: slot.height * constraints.maxHeight,
                    child: Padding(
                      padding: const EdgeInsets.all(1.5),
                      child: Container(decoration: BoxDecoration(color: getThemeActiveBackgroundColor().withValues(alpha: 0.7), borderRadius: BorderRadius.circular(2))),
                    ),
                  );
                }).toList(),
          );
        },
      ),
    );
  }

  Rect _displayRect(WindowManagerDisplay display) {
    final rect = display.bounds.width > 0 && display.bounds.height > 0 ? display.bounds : display.workArea;
    return Rect.fromLTWH(rect.x.toDouble(), rect.y.toDouble(), math.max(1.0, rect.width.toDouble()), math.max(1.0, rect.height.toDouble()));
  }
}

class _WindowManagerGroup {
  String id;
  String name;
  List<_WindowManagerGroupScreen> screens;

  _WindowManagerGroup({required this.id, required this.name, required this.screens});

  int get appCount {
    var count = 0;
    for (final screen in screens) {
      count += screen.assignments.where((assignment) => assignment.app.identity.trim().isNotEmpty).length;
    }
    return count;
  }

  factory _WindowManagerGroup.fromJson(Map<String, dynamic> json) {
    return _WindowManagerGroup(
      id: json['Id'] ?? '',
      name: json['Name'] ?? '',
      screens:
          (json['Screens'] is List ? json['Screens'] as List : <dynamic>[])
              .whereType<Map>()
              .map((item) => _WindowManagerGroupScreen.fromJson(Map<String, dynamic>.from(item)))
              .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'Id': id, 'Name': name, 'Screens': screens.map((screen) => screen.toJson()).toList()};
  }

  _WindowManagerGroup copy() {
    return _WindowManagerGroup(id: id, name: name, screens: screens.map((screen) => screen.copy()).toList());
  }

  _WindowManagerGroupScreen screenFor(WindowManagerDisplay display, int displayIndex) {
    for (final screen in screens) {
      if (screen.matches(display, displayIndex)) {
        return screen;
      }
    }
    final screen = _WindowManagerGroupScreen(displayId: display.id, displayIndex: displayIndex, layout: _WindowGroupLayouts.full.id, assignments: <_WindowManagerAssignment>[]);
    screens.add(screen);
    return screen;
  }
}

class _WindowManagerGroupScreen {
  String displayId;
  int displayIndex;
  String layout;
  List<_WindowManagerAssignment> assignments;

  _WindowManagerGroupScreen({required this.displayId, required this.displayIndex, required this.layout, required this.assignments});

  factory _WindowManagerGroupScreen.fromJson(Map<String, dynamic> json) {
    return _WindowManagerGroupScreen(
      displayId: json['DisplayId'] ?? '',
      displayIndex: json['DisplayIndex'] ?? 0,
      layout: json['Layout'] ?? _WindowGroupLayouts.fullId,
      assignments:
          (json['Assignments'] is List ? json['Assignments'] as List : <dynamic>[])
              .whereType<Map>()
              .map((item) => _WindowManagerAssignment.fromJson(Map<String, dynamic>.from(item)))
              .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'DisplayId': displayId, 'DisplayIndex': displayIndex, 'Layout': layout, 'Assignments': assignments.map((assignment) => assignment.toJson()).toList()};
  }

  _WindowManagerGroupScreen copy() {
    return _WindowManagerGroupScreen(displayId: displayId, displayIndex: displayIndex, layout: layout, assignments: assignments.map((assignment) => assignment.copy()).toList());
  }

  bool matches(WindowManagerDisplay display, int index) {
    return (displayId.trim().isNotEmpty && displayId == display.id) || (displayId.trim().isEmpty && displayIndex == index);
  }

  _WindowManagerAssignment? assignmentFor(String slotId) {
    for (final assignment in assignments) {
      if (assignment.slot == slotId) {
        return assignment;
      }
    }
    return null;
  }
}

class _WindowManagerAssignment {
  String slot;
  IgnoredHotkeyApp app;

  _WindowManagerAssignment({required this.slot, required this.app});

  factory _WindowManagerAssignment.fromJson(Map<String, dynamic> json) {
    final rawApp = json['App'];
    return _WindowManagerAssignment(slot: json['Slot'] ?? '', app: rawApp is Map ? IgnoredHotkeyApp.fromJson(Map<String, dynamic>.from(rawApp)) : IgnoredHotkeyApp.empty());
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'Slot': slot, 'App': app.toJson()};
  }

  _WindowManagerAssignment copy() {
    return _WindowManagerAssignment(slot: slot, app: IgnoredHotkeyApp(name: app.name, identity: app.identity, path: app.path, icon: app.icon));
  }
}

class _WindowGroupLayouts {
  static const String fullId = 'full';
  static _WindowGroupLayout get full => layouts.first;

  static final List<_WindowGroupLayout> layouts = <_WindowGroupLayout>[
    _WindowGroupLayout(
      id: 'full',
      titleKey: 'plugin_window_manager_group_layout_full',
      slotCount: 1,
      slots: <_WindowGroupSlot>[_WindowGroupSlot(id: 'full', titleKey: 'plugin_window_manager_group_slot_full', left: 0, top: 0, width: 1, height: 1)],
    ),
    _WindowGroupLayout(
      id: 'halves-horizontal',
      titleKey: 'plugin_window_manager_group_layout_halves_horizontal',
      slotCount: 2,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'left', titleKey: 'plugin_window_manager_group_slot_left', left: 0, top: 0, width: 0.5, height: 1),
        _WindowGroupSlot(id: 'right', titleKey: 'plugin_window_manager_group_slot_right', left: 0.5, top: 0, width: 0.5, height: 1),
      ],
    ),
    _WindowGroupLayout(
      id: 'halves-vertical',
      titleKey: 'plugin_window_manager_group_layout_halves_vertical',
      slotCount: 2,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'top', titleKey: 'plugin_window_manager_group_slot_top', left: 0, top: 0, width: 1, height: 0.5),
        _WindowGroupSlot(id: 'bottom', titleKey: 'plugin_window_manager_group_slot_bottom', left: 0, top: 0.5, width: 1, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-left-main',
      titleKey: 'plugin_window_manager_group_layout_three_left_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'left', titleKey: 'plugin_window_manager_group_slot_left', left: 0, top: 0, width: 0.5, height: 1),
        _WindowGroupSlot(id: 'rightTop', titleKey: 'plugin_window_manager_group_slot_right_top', left: 0.5, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'rightBottom', titleKey: 'plugin_window_manager_group_slot_right_bottom', left: 0.5, top: 0.5, width: 0.5, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-right-main',
      titleKey: 'plugin_window_manager_group_layout_three_right_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'leftTop', titleKey: 'plugin_window_manager_group_slot_left_top', left: 0, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'leftBottom', titleKey: 'plugin_window_manager_group_slot_left_bottom', left: 0, top: 0.5, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'right', titleKey: 'plugin_window_manager_group_slot_right', left: 0.5, top: 0, width: 0.5, height: 1),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-top-main',
      titleKey: 'plugin_window_manager_group_layout_three_top_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'top', titleKey: 'plugin_window_manager_group_slot_top', left: 0, top: 0, width: 1, height: 0.5),
        _WindowGroupSlot(id: 'bottomLeft', titleKey: 'plugin_window_manager_group_slot_bottom_left', left: 0, top: 0.5, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottomRight', titleKey: 'plugin_window_manager_group_slot_bottom_right', left: 0.5, top: 0.5, width: 0.5, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-bottom-main',
      titleKey: 'plugin_window_manager_group_layout_three_bottom_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'topLeft', titleKey: 'plugin_window_manager_group_slot_top_left', left: 0, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'topRight', titleKey: 'plugin_window_manager_group_slot_top_right', left: 0.5, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottom', titleKey: 'plugin_window_manager_group_slot_bottom', left: 0, top: 0.5, width: 1, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'quarters',
      titleKey: 'plugin_window_manager_group_layout_quarters',
      slotCount: 4,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'topLeft', titleKey: 'plugin_window_manager_group_slot_top_left', left: 0, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'topRight', titleKey: 'plugin_window_manager_group_slot_top_right', left: 0.5, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottomLeft', titleKey: 'plugin_window_manager_group_slot_bottom_left', left: 0, top: 0.5, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottomRight', titleKey: 'plugin_window_manager_group_slot_bottom_right', left: 0.5, top: 0.5, width: 0.5, height: 0.5),
      ],
    ),
  ];

  static _WindowGroupLayout byId(String id) {
    for (final layout in layouts) {
      if (layout.id == id) {
        return layout;
      }
    }
    return full;
  }

  static List<_WindowGroupLayout> bySlotCount(int slotCount) {
    return layouts.where((layout) => layout.slotCount == slotCount).toList();
  }
}

class _WindowGroupLayout {
  final String id;
  final String titleKey;
  final int slotCount;
  final List<_WindowGroupSlot> slots;

  _WindowGroupLayout({required this.id, required this.titleKey, required this.slotCount, required this.slots});
}

class _WindowGroupSlot {
  final String id;
  final String titleKey;
  final double left;
  final double top;
  final double width;
  final double height;

  _WindowGroupSlot({required this.id, required this.titleKey, required this.left, required this.top, required this.width, required this.height});
}
