import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_hotkey_display_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_setting_util.dart';

const _hotkeyOverviewAccentColor = Color(0xFF3B82F6);

class WoxHotkeyOverviewPreviewView extends StatefulWidget {
  final WoxTheme woxTheme;
  final String previewData;

  const WoxHotkeyOverviewPreviewView({super.key, required this.woxTheme, required this.previewData});

  @override
  State<WoxHotkeyOverviewPreviewView> createState() => _WoxHotkeyOverviewPreviewViewState();
}

class _WoxHotkeyOverviewPreviewViewState extends State<WoxHotkeyOverviewPreviewView> {
  final WoxLauncherController _launcherController = Get.find<WoxLauncherController>();

  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  String _tr(String key) => _launcherController.tr(key);

  @override
  Widget build(BuildContext context) {
    final sections = _buildSections();
    final search = _HotkeyOverviewPreviewData.fromPreviewData(widget.previewData).search.toLowerCase();
    final filteredSections =
        sections
            .map((section) => _HotkeyOverviewSection(title: section.title, entries: section.entries.where((entry) => entry.matches(search)).toList()))
            .where((section) => section.entries.isNotEmpty)
            .toList();
    final filteredCount = filteredSections.fold<int>(0, (count, section) => count + section.entries.length);

    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor);
    final mutedColor = safeFromCssColor(widget.woxTheme.previewPropertyContentColor, defaultColor: textColor).withValues(alpha: 0.72);
    const accentColor = _hotkeyOverviewAccentColor;

    return Container(
      padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(18), _metrics.scaledSpacing(16), _metrics.scaledSpacing(16), _metrics.scaledSpacing(14)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _buildHeader(textColor, mutedColor, accentColor, filteredCount),
          SizedBox(height: _metrics.scaledSpacing(14)),
          Expanded(
            child:
                filteredSections.isEmpty
                    ? Center(child: Text(_tr("ui_hotkey_overview_empty"), style: TextStyle(color: mutedColor, fontSize: _metrics.resultSubtitleFontSize)))
                    : ListView.separated(
                      itemCount: filteredSections.length,
                      separatorBuilder: (context, index) => SizedBox(height: _metrics.scaledSpacing(14)),
                      itemBuilder: (context, index) => _buildSection(filteredSections[index], textColor, mutedColor, accentColor),
                    ),
          ),
        ],
      ),
    );
  }

  Widget _buildHeader(Color textColor, Color mutedColor, Color accentColor, int totalCount) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: _metrics.scaledSpacing(34),
          height: _metrics.scaledSpacing(34),
          decoration: BoxDecoration(
            color: accentColor,
            borderRadius: BorderRadius.circular(8),
            boxShadow: [BoxShadow(color: accentColor.withValues(alpha: 0.28), blurRadius: _metrics.scaledSpacing(10), offset: Offset(0, _metrics.scaledSpacing(3)))],
          ),
          child: Icon(Icons.keyboard_alt_outlined, color: Colors.white, size: _metrics.scaledSpacing(20)),
        ),
        SizedBox(width: _metrics.scaledSpacing(12)),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      _tr("ui_hotkey_overview_title"),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: textColor, fontSize: _metrics.resultTitleFontSize + 2, fontWeight: FontWeight.w800),
                    ),
                  ),
                  SizedBox(width: _metrics.scaledSpacing(10)),
                  _buildCountPill(accentColor, totalCount),
                ],
              ),
              SizedBox(height: _metrics.scaledSpacing(3)),
              Text(
                _tr("ui_hotkey_overview_subtitle"),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: mutedColor, fontSize: _metrics.resultSubtitleFontSize, height: 1.25),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildCountPill(Color accentColor, int totalCount) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(10), vertical: _metrics.scaledSpacing(5)),
      decoration: BoxDecoration(
        color: accentColor,
        borderRadius: BorderRadius.circular(999),
        boxShadow: [BoxShadow(color: accentColor.withValues(alpha: 0.24), blurRadius: _metrics.scaledSpacing(9), offset: Offset(0, _metrics.scaledSpacing(2)))],
      ),
      child: Text(
        _tr("ui_hotkey_overview_count").replaceAll("{count}", totalCount.toString()),
        style: TextStyle(color: Colors.white, fontSize: _metrics.smallLabelFontSize, height: 1, fontWeight: FontWeight.w800),
      ),
    );
  }

  Widget _buildSection(_HotkeyOverviewSection section, Color textColor, Color mutedColor, Color accentColor) {
    final borderColor = safeFromCssColor(widget.woxTheme.previewSplitLineColor).withValues(alpha: 0.42);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                section.title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: textColor, fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w800),
              ),
            ),
          ],
        ),
        SizedBox(height: _metrics.scaledSpacing(7)),
        Container(
          decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(8)),
          child: Column(
            children: [
              for (var i = 0; i < section.entries.length; i++) ...[
                _buildEntryRow(section.entries[i], textColor, mutedColor, accentColor),
                if (i != section.entries.length - 1) Divider(height: 1, thickness: 1, color: borderColor),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildEntryRow(_HotkeyOverviewEntry entry, Color textColor, Color mutedColor, Color accentColor) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(10), vertical: _metrics.scaledSpacing(8)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          SizedBox(width: _metrics.scaledSpacing(220), child: _buildShortcutChips(entry, textColor, accentColor)),
          SizedBox(width: _metrics.scaledSpacing(10)),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  entry.action,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: textColor, fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w700, height: 1.2),
                ),
                if (entry.detail.isNotEmpty)
                  Padding(
                    padding: EdgeInsets.only(top: _metrics.scaledSpacing(2)),
                    child: Text(
                      entry.detail,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: mutedColor, fontSize: _metrics.smallLabelFontSize, height: 1.2),
                    ),
                  ),
              ],
            ),
          ),
          SizedBox(width: _metrics.scaledSpacing(10)),
          Text(entry.source, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: mutedColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }

  Widget _buildShortcutChips(_HotkeyOverviewEntry entry, Color textColor, Color accentColor) {
    final labels = entry.displayLabels;
    final chipWidgets = <Widget>[];
    for (final label in labels) {
      if (chipWidgets.isNotEmpty) {
        chipWidgets.add(SizedBox(width: _metrics.scaledSpacing(4)));
      }
      chipWidgets.add(_buildShortcutChip(label, textColor, accentColor, entry.isKeyboardHotkey));
    }

    // Keep one shortcut combination on a single visual line. Very long user
    // shortcuts stay horizontally scrollable instead of making the row taller.
    return SingleChildScrollView(scrollDirection: Axis.horizontal, child: Row(mainAxisSize: MainAxisSize.min, children: chipWidgets));
  }

  Widget _buildShortcutChip(String label, Color textColor, Color accentColor, bool isKeyboardHotkey) {
    return Container(
      constraints: BoxConstraints(minWidth: isKeyboardHotkey ? _metrics.scaledSpacing(28) : _metrics.scaledSpacing(34), minHeight: _metrics.scaledSpacing(22)),
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(7), vertical: _metrics.scaledSpacing(4)),
      decoration: BoxDecoration(
        color: isKeyboardHotkey ? textColor.withValues(alpha: 0.06) : accentColor.withValues(alpha: 0.11),
        borderRadius: BorderRadius.circular(5),
        border: Border.all(color: isKeyboardHotkey ? textColor.withValues(alpha: 0.18) : accentColor.withValues(alpha: 0.24)),
      ),
      child: Text(
        label,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        textAlign: TextAlign.center,
        style: _shortcutChipTextStyle(isKeyboardHotkey ? textColor.withValues(alpha: 0.9) : accentColor),
      ),
    );
  }

  TextStyle _shortcutChipTextStyle(Color color) {
    return TextStyle(color: color, fontSize: _metrics.smallLabelFontSize, height: 1, fontWeight: FontWeight.w800);
  }

  List<_HotkeyOverviewSection> _buildSections() {
    final setting = WoxSettingUtil.instance.currentSetting;
    return [
      _HotkeyOverviewSection(
        title: _tr("ui_hotkey_overview_global"),
        entries:
            [
              _keyboardEntry(setting.mainHotkey, _tr("ui_hotkey_overview_open_wox"), _tr("ui_hotkey_overview_global"), _tr("ui_hotkey_overview_source_setting")),
              _keyboardEntry(setting.selectionHotkey, _tr("ui_hotkey_overview_search_selection"), _tr("ui_hotkey_overview_global"), _tr("ui_hotkey_overview_source_setting")),
            ].where((entry) => entry.rawShortcut.isNotEmpty).toList(),
      ),
      _HotkeyOverviewSection(
        title: _tr("ui_hotkey_overview_launcher"),
        entries: [
          _keyboardEntry(
            _launcherController.moreActionsHotkey,
            _tr("ui_hotkey_overview_more_actions"),
            _tr("ui_hotkey_overview_launcher"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(
            _launcherController.queryRefinementToggleHotkey,
            _tr("ui_hotkey_overview_filters"),
            _tr("ui_hotkey_overview_launcher"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(_launcherController.attentionHotkey, _tr("ui_hotkey_overview_attention"), _tr("ui_hotkey_overview_launcher"), _tr("ui_hotkey_overview_source_builtin")),
        ],
      ),
      _HotkeyOverviewSection(
        title: _tr("ui_hotkey_overview_preview"),
        entries: [
          _keyboardEntry(
            _launcherController.previewFullscreenHotkey,
            _tr("ui_hotkey_overview_preview_fullscreen"),
            _tr("ui_hotkey_overview_preview"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(
            _launcherController.previewSearchHotkey,
            _tr("ui_hotkey_overview_preview_search"),
            _tr("ui_hotkey_overview_preview"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(
            _launcherController.previewRefreshHotkey,
            _tr("ui_hotkey_overview_webview_refresh"),
            _tr("ui_hotkey_overview_preview"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(
            _launcherController.previewBackHotkey,
            _tr("ui_hotkey_overview_webview_back"),
            _tr("ui_hotkey_overview_preview"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(
            _launcherController.previewForwardHotkey,
            _tr("ui_hotkey_overview_webview_forward"),
            _tr("ui_hotkey_overview_preview"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
          _keyboardEntry(
            _launcherController.previewInspectorHotkey,
            _tr("ui_hotkey_overview_webview_inspector"),
            _tr("ui_hotkey_overview_preview"),
            _tr("ui_hotkey_overview_source_builtin"),
          ),
        ],
      ),
      _HotkeyOverviewSection(title: _tr("ui_hotkey_overview_query_hotkeys"), entries: _buildQueryHotkeyEntries(setting.queryHotkeys)),
      _HotkeyOverviewSection(title: _tr("ui_hotkey_overview_query_shortcuts"), entries: _buildQueryShortcutEntries(setting.queryShortcuts)),
    ].where((section) => section.entries.isNotEmpty).toList();
  }

  List<_HotkeyOverviewEntry> _buildQueryHotkeyEntries(List<QueryHotkey> queryHotkeys) {
    return queryHotkeys
        .where((item) => !item.disabled && item.hotkey.trim().isNotEmpty && item.query.trim().isNotEmpty)
        .map(
          (item) => _keyboardEntry(
            item.hotkey,
            item.displayName,
            _tr("ui_hotkey_overview_query_hotkeys"),
            _tr("ui_hotkey_overview_source_user"),
            detail: item.query.trim() == item.displayName.trim() ? "" : item.query.trim(),
          ),
        )
        .toList();
  }

  List<_HotkeyOverviewEntry> _buildQueryShortcutEntries(List<QueryShortcut> queryShortcuts) {
    return queryShortcuts
        .where((item) => !item.disabled && item.shortcut.trim().isNotEmpty && item.query.trim().isNotEmpty)
        .map(
          (item) => _HotkeyOverviewEntry(
            rawShortcut: item.shortcut.trim(),
            action: item.query.trim(),
            scope: _tr("ui_hotkey_overview_query_shortcuts"),
            source: _tr("ui_hotkey_overview_source_user"),
            isKeyboardHotkey: false,
          ),
        )
        .toList();
  }

  _HotkeyOverviewEntry _keyboardEntry(String hotkey, String action, String scope, String source, {String detail = ""}) {
    return _HotkeyOverviewEntry(rawShortcut: hotkey.trim(), action: action, scope: scope, source: source, detail: detail, isKeyboardHotkey: true);
  }
}

class _HotkeyOverviewPreviewData {
  final String search;

  const _HotkeyOverviewPreviewData({required this.search});

  factory _HotkeyOverviewPreviewData.fromPreviewData(String previewData) {
    try {
      final decoded = jsonDecode(previewData);
      if (decoded is Map<String, dynamic>) {
        return _HotkeyOverviewPreviewData(search: (decoded["search"] ?? "").toString().trim());
      }
    } catch (_) {}
    return const _HotkeyOverviewPreviewData(search: "");
  }
}

class _HotkeyOverviewSection {
  final String title;
  final List<_HotkeyOverviewEntry> entries;

  const _HotkeyOverviewSection({required this.title, required this.entries});
}

class _HotkeyOverviewEntry {
  final String rawShortcut;
  final String action;
  final String scope;
  final String source;
  final String detail;
  final bool isKeyboardHotkey;

  const _HotkeyOverviewEntry({required this.rawShortcut, required this.action, required this.scope, required this.source, this.detail = "", required this.isKeyboardHotkey});

  List<String> get displayLabels {
    if (!isKeyboardHotkey) {
      return [rawShortcut];
    }

    final display = WoxHotkeyDisplayUtil.labelFromHotkeyString(rawShortcut);
    if (display.isEmpty) {
      return [rawShortcut];
    }
    return display.split("+").where((label) => label.trim().isNotEmpty).toList();
  }

  bool matches(String query) {
    if (query.isEmpty) {
      return true;
    }

    final values = [rawShortcut, displayLabels.join(" "), action, scope, source, detail];
    if (values.any((value) => value.toLowerCase().contains(query))) {
      return true;
    }

    final normalizedQuery = _normalizeSearchText(query);
    if (normalizedQuery.isEmpty) {
      return false;
    }
    return values.any((value) => _normalizeSearchText(value).contains(normalizedQuery));
  }

  String _normalizeSearchText(String value) => value.toLowerCase().replaceAll(RegExp(r"[\s_+\-]+"), "");
}
