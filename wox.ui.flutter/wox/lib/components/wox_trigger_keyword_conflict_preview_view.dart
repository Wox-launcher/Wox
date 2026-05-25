import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_trigger_keyword_conflict_preview.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxTriggerKeywordConflictPreviewView extends StatefulWidget {
  final TriggerKeywordConflictPreviewData data;
  final WoxLauncherController launcherController;

  const WoxTriggerKeywordConflictPreviewView({super.key, required this.data, required this.launcherController});

  @override
  State<WoxTriggerKeywordConflictPreviewView> createState() => _WoxTriggerKeywordConflictPreviewViewState();
}

class _WoxTriggerKeywordConflictPreviewViewState extends State<WoxTriggerKeywordConflictPreviewView> {
  static const double _compactBreakpoint = 620;
  static const double _actionColumnWidth = 86;

  final Map<String, TextEditingController> _keywordControllers = {};
  final Map<String, bool> _savingByPlugin = {};
  final Map<String, String> _messageByPlugin = {};

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  void initState() {
    super.initState();
    _syncControllers();
  }

  @override
  void didUpdateWidget(covariant WoxTriggerKeywordConflictPreviewView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_dataSignature(oldWidget.data) != _dataSignature(widget.data)) {
      _syncControllers();
    }
  }

  @override
  void dispose() {
    for (final controller in _keywordControllers.values) {
      controller.dispose();
    }
    super.dispose();
  }

  void _syncControllers() {
    final activePluginIds = widget.data.plugins.map((plugin) => plugin.pluginId).toSet();
    final stalePluginIds = _keywordControllers.keys.where((pluginId) => !activePluginIds.contains(pluginId)).toList();
    for (final pluginId in stalePluginIds) {
      _keywordControllers.remove(pluginId)?.dispose();
      _savingByPlugin.remove(pluginId);
      _messageByPlugin.remove(pluginId);
    }

    for (final plugin in widget.data.plugins) {
      final value = plugin.triggerKeywords.join(", ");
      _keywordControllers.putIfAbsent(plugin.pluginId, () => TextEditingController(text: value));
    }
  }

  String _dataSignature(TriggerKeywordConflictPreviewData data) {
    return data.plugins.map((plugin) => "${plugin.pluginId}:${plugin.icon.imageType}:${plugin.icon.imageData}:${plugin.triggerKeywords.join(",")}").join("|");
  }

  List<String> _parseKeywords(String value) {
    return value.split(",").map((item) => item.trim()).where((item) => item.isNotEmpty).toList();
  }

  Future<void> _saveKeywords(TriggerKeywordConflictPreviewPlugin plugin) async {
    final keywords = _parseKeywords(_keywordControllers[plugin.pluginId]?.text ?? "");
    if (keywords.isEmpty) {
      setState(() {
        _messageByPlugin[plugin.pluginId] = tr("plugin_manager_trigger_keyword_conflict_empty");
      });
      return;
    }

    setState(() {
      _savingByPlugin[plugin.pluginId] = true;
      _messageByPlugin[plugin.pluginId] = "";
    });

    final traceId = const UuidV4().generate();
    try {
      // Trigger keyword conflicts are resolved from the launcher preview itself.
      // Saving through the existing plugin-setting endpoint keeps this path aligned
      // with the full settings page while avoiding another plugin-setting surface.
      await WoxApi.instance.updatePluginSetting(traceId, plugin.pluginId, "TriggerKeywords", keywords.join(","));
      if (!mounted) {
        return;
      }
      setState(() {
        _messageByPlugin[plugin.pluginId] = tr("plugin_manager_trigger_keyword_conflict_saved");
      });
      widget.launcherController.onRefreshQuery(traceId, false);
    } catch (e) {
      Logger.instance.error(traceId, "failed to save trigger keywords from conflict preview: $e");
      if (!mounted) {
        return;
      }
      setState(() {
        _messageByPlugin[plugin.pluginId] = e.toString();
      });
    } finally {
      if (mounted) {
        setState(() {
          _savingByPlugin[plugin.pluginId] = false;
        });
      }
    }
  }

  Color get _warningColor => isThemeDark() ? const Color(0xFFF3B75C) : const Color(0xFFB96D18);

  Color get _panelColor {
    // Visual fix: the first table surface was too close to black in dark themes.
    // A subtle text-color lift keeps the panel tied to the current theme while
    // separating it from the launcher background without adding a new palette.
    final baseColor = getThemeCardBackgroundColor();
    return isThemeDark() ? Color.alphaBlend(getThemeTextColor().withValues(alpha: 0.055), baseColor) : baseColor;
  }

  Color get _hairlineColor => getThemeDividerColor().withValues(alpha: isThemeDark() ? 0.62 : 0.44);

  String get _displayTitle {
    final keyword = widget.data.keyword.trim();
    var title = widget.data.title.trim();
    if (keyword.isEmpty || title.isEmpty) {
      return title;
    }

    for (final suffix in [": $keyword", ":$keyword", "：$keyword", "： $keyword"]) {
      if (title.endsWith(suffix)) {
        title = title.substring(0, title.length - suffix.length).trim();
        break;
      }
    }
    return title;
  }

  Widget _buildHeader() {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 34,
          height: 34,
          decoration: BoxDecoration(color: _warningColor.withValues(alpha: 0.14), border: Border.all(color: _warningColor.withValues(alpha: 0.36)), shape: BoxShape.circle),
          child: Icon(Icons.warning_amber_rounded, color: _warningColor, size: 20),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Flexible(
                    child: Text(
                      _displayTitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: getThemeTextColor(), fontSize: 16, fontWeight: FontWeight.w700),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 5),
              Text(
                widget.data.message,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.3, fontWeight: FontWeight.w500),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildKeywordChip(String keyword) {
    final isConflictKeyword = keyword == widget.data.keyword;
    final chipColor = isConflictKeyword ? _warningColor : getThemeSubTextColor();
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: isConflictKeyword ? chipColor.withValues(alpha: 0.14) : getThemeSubTextColor().withValues(alpha: 0.08),
        border: Border.all(color: isConflictKeyword ? chipColor.withValues(alpha: 0.52) : getThemeDividerColor().withValues(alpha: 0.66)),
        borderRadius: BorderRadius.circular(7),
      ),
      child: Text(keyword, style: TextStyle(color: isConflictKeyword ? chipColor : getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w700, fontFamily: "monospace")),
    );
  }

  Widget _buildCurrentKeywords(TriggerKeywordConflictPreviewPlugin plugin) {
    final currentKeywords = plugin.triggerKeywords.where((keyword) => keyword.trim().isNotEmpty).toList();
    if (currentKeywords.isEmpty) {
      return Text("-", style: TextStyle(color: getThemeSubTextColor(), fontSize: 12));
    }

    return Wrap(spacing: 6, runSpacing: 6, children: currentKeywords.map(_buildKeywordChip).toList());
  }

  String _pluginFallbackInitial(TriggerKeywordConflictPreviewPlugin plugin) {
    final name = plugin.pluginName.trim();
    if (name.isNotEmpty) {
      return String.fromCharCode(name.runes.first);
    }
    final pluginId = plugin.pluginId.trim();
    return pluginId.isNotEmpty ? String.fromCharCode(pluginId.runes.first).toUpperCase() : "?";
  }

  Widget _buildPluginIcon(TriggerKeywordConflictPreviewPlugin plugin) {
    final hasIcon = plugin.icon.imageData.trim().isNotEmpty;

    // Conflict rows now use the plugin's own WoxImage icon. The old generated
    // initial made every row feel like a temporary form card, while the real icon
    // matches the plugin list and helps users identify which route they are editing.
    return Container(
      width: 34,
      height: 34,
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(color: getThemeTextColor().withValues(alpha: isThemeDark() ? 0.045 : 0.065), borderRadius: BorderRadius.circular(7)),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(5),
        child:
            hasIcon
                ? WoxImageView(woxImage: plugin.icon, width: 26, height: 26)
                : Center(child: Text(_pluginFallbackInitial(plugin), style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w700))),
      ),
    );
  }

  Widget _buildPluginIdentity(TriggerKeywordConflictPreviewPlugin plugin) {
    final pluginName = plugin.pluginName.trim().isNotEmpty ? plugin.pluginName.trim() : plugin.pluginId;

    return Row(
      children: [
        _buildPluginIcon(plugin),
        const SizedBox(width: 10),
        Expanded(
          // Visual fix: plugin IDs add noise here because this preview is about
          // choosing which visible plugin name should keep the trigger keyword.
          child: Text(pluginName, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 13.5, fontWeight: FontWeight.w700)),
        ),
      ],
    );
  }

  Widget _buildKeywordEditor(TriggerKeywordConflictPreviewPlugin plugin) {
    final message = _messageByPlugin[plugin.pluginId] ?? "";

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        WoxTextField(
          controller: _keywordControllers[plugin.pluginId],
          hintText: tr("plugin_manager_trigger_keyword_conflict_keywords_hint"),
          width: double.infinity,
          contentPadding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
          style: TextStyle(color: getThemeTextColor(), fontSize: 12.5, fontWeight: FontWeight.w600, fontFamily: "monospace"),
        ),
        if (message.trim().isNotEmpty) Padding(padding: const EdgeInsets.only(top: 6), child: Text(message, style: TextStyle(color: getThemeSubTextColor(), fontSize: 11))),
      ],
    );
  }

  Widget _buildSaveButton(TriggerKeywordConflictPreviewPlugin plugin) {
    final isSaving = _savingByPlugin[plugin.pluginId] ?? false;

    final buttonBackground = getThemeActiveBackgroundColor();

    // Visual fix: query cursor color can be white in some themes, which made the
    // action button look like an inactive white block. Use the action active
    // background so this button matches the rest of Wox's actionable controls.
    return SizedBox(
      width: 76,
      height: 34,
      child: ElevatedButton(
        onPressed: isSaving ? null : () => _saveKeywords(plugin),
        style: ButtonStyle(
          backgroundColor: WidgetStateProperty.resolveWith<Color>((states) => states.contains(WidgetState.disabled) ? buttonBackground.withValues(alpha: 0.42) : buttonBackground),
          foregroundColor: WidgetStateProperty.all(getThemeActionItemActiveColor()),
          elevation: WidgetStateProperty.all(0),
          padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 12, vertical: 8)),
          textStyle: WidgetStateProperty.all(const TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
          shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(6))),
          minimumSize: WidgetStateProperty.all(Size.zero),
          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        ),
        child: Text(isSaving ? "${tr("ui_save")}..." : tr("ui_save"), maxLines: 1, overflow: TextOverflow.ellipsis),
      ),
    );
  }

  Widget _buildTableHeaderLabel(String text) {
    return Text(text, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeSubTextColor(), fontSize: 11.5, fontWeight: FontWeight.w700));
  }

  Widget _buildWideHeader() {
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 13, 18, 11),
      child: Row(
        children: [
          Expanded(flex: 4, child: _buildTableHeaderLabel(tr("plugin_manager_trigger_keyword_conflict_plugins"))),
          const SizedBox(width: 14),
          Expanded(flex: 3, child: _buildTableHeaderLabel(tr("plugin_manager_trigger_keyword_conflict_current_keywords"))),
          const SizedBox(width: 14),
          Expanded(flex: 5, child: _buildTableHeaderLabel(tr("plugin_manager_trigger_keyword_conflict_edit_keywords"))),
          const SizedBox(width: 14),
          SizedBox(width: _actionColumnWidth, child: _buildTableHeaderLabel(tr("plugin_manager_trigger_keyword_conflict_operation"))),
        ],
      ),
    );
  }

  Widget _buildWidePluginRow(TriggerKeywordConflictPreviewPlugin plugin) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 13),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(flex: 4, child: _buildPluginIdentity(plugin)),
          const SizedBox(width: 14),
          Expanded(flex: 3, child: Padding(padding: const EdgeInsets.only(top: 2), child: _buildCurrentKeywords(plugin))),
          const SizedBox(width: 14),
          Expanded(flex: 5, child: _buildKeywordEditor(plugin)),
          const SizedBox(width: 14),
          SizedBox(width: _actionColumnWidth, child: _buildSaveButton(plugin)),
        ],
      ),
    );
  }

  Widget _buildCompactPluginCard(TriggerKeywordConflictPreviewPlugin plugin) {
    // Compact preview uses one repeated item card because table columns would
    // squeeze the editor and make long plugin ids unreadable in narrow previews.
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: isThemeDark() ? 0.32 : 0.18),
        border: Border.all(color: _hairlineColor),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(crossAxisAlignment: CrossAxisAlignment.start, children: [Expanded(child: _buildPluginIdentity(plugin)), const SizedBox(width: 10), _buildSaveButton(plugin)]),
          const SizedBox(height: 12),
          Text(tr("plugin_manager_trigger_keyword_conflict_current_keywords"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 11.5, fontWeight: FontWeight.w700)),
          const SizedBox(height: 7),
          _buildCurrentKeywords(plugin),
          const SizedBox(height: 12),
          Text(tr("plugin_manager_trigger_keyword_conflict_edit_keywords"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 11.5, fontWeight: FontWeight.w700)),
          const SizedBox(height: 7),
          _buildKeywordEditor(plugin),
        ],
      ),
    );
  }

  Widget _buildWidePanel() {
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(color: _panelColor, border: Border.all(color: _hairlineColor), borderRadius: BorderRadius.circular(8)),
      child: Column(
        children: [
          _buildWideHeader(),
          Divider(height: 1, thickness: 1, color: _hairlineColor),
          Expanded(
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(horizontal: 18),
              itemBuilder: (context, index) => _buildWidePluginRow(widget.data.plugins[index]),
              separatorBuilder: (context, index) => Divider(height: 1, thickness: 1, color: _hairlineColor),
              itemCount: widget.data.plugins.length,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCompactPanel() {
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(color: _panelColor, border: Border.all(color: _hairlineColor), borderRadius: BorderRadius.circular(8)),
      child: Column(
        children: [
          Expanded(
            child: ListView.separated(
              padding: const EdgeInsets.all(12),
              itemBuilder: (context, index) => _buildCompactPluginCard(widget.data.plugins[index]),
              separatorBuilder: (context, index) => const SizedBox(height: 10),
              itemCount: widget.data.plugins.length,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildConflictPanel() {
    if (widget.data.plugins.isEmpty) {
      return Center(child: Text(widget.data.message, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)));
    }

    // Bug fix: the first redesign used a desktop-sized breakpoint, but Wox
    // preview constraints are logical pixels; a 1600px screenshot can still be
    // around 800 logical pixels. Keep the table layout available at that size so
    // the real launcher matches the approved design density instead of falling
    // back to oversized stacked cards.
    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < _compactBreakpoint) {
          return _buildCompactPanel();
        }
        return _buildWidePanel();
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(18),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [_buildHeader(), const SizedBox(height: 16), Expanded(child: _buildConflictPanel())]),
    );
  }
}
