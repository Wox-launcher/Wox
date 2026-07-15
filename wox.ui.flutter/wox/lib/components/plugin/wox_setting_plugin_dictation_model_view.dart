import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/entity/setting/wox_plugin_setting_dictation_model.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginDictationModel extends StatefulWidget {
  final String value;
  final PluginSettingValueDictationModel item;
  final double labelWidth;
  final Future<String?> Function(String key, String value) onUpdate;

  const WoxSettingPluginDictationModel({super.key, required this.value, required this.item, required this.labelWidth, required this.onUpdate});

  @override
  State<WoxSettingPluginDictationModel> createState() => _WoxSettingPluginDictationModelState();
}

class _WoxSettingPluginDictationModelState extends State<WoxSettingPluginDictationModel> with WoxSettingPluginItemMixin<WoxSettingPluginDictationModel> {
  late List<DictationModelOption> _options;
  String _selectedId = "";
  String _errorMessage = "";
  Timer? _pollTimer;

  // Engine (native lib) download state
  String _engineState = "done";
  int _engineProgress = 0;
  String _engineError = "";
  bool _engineReady = true;

  final GlobalKey _dropdownKey = GlobalKey();
  OverlayEntry? _overlayEntry;
  final LayerLink _layerLink = LayerLink();

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    _options = List.from(widget.item.options);
    _selectedId = widget.value.isNotEmpty ? widget.value : _findFirstDownloaded();
    if (_selectedId.isNotEmpty) {
      widget.onUpdate(widget.item.key, _selectedId);
    }
    _refreshEngineStatus();
    _refreshStatus();
    _startPollingIfDownloading();
  }

  @override
  void didUpdateWidget(covariant WoxSettingPluginDictationModel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.item.options != widget.item.options) {
      _options = List.from(widget.item.options);
    }
    if (oldWidget.value != widget.value) {
      _selectedId = widget.value.isNotEmpty ? widget.value : _findFirstDownloaded();
    }
    _startPollingIfDownloading();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _removeOverlay();
    super.dispose();
  }

  String _findFirstDownloaded() {
    for (final opt in _options) {
      if (opt.status == DictationModelStatus.downloaded) {
        return opt.id;
      }
    }
    return "";
  }

  // Polls native lib download status while the engine is being downloaded.
  Future<void> _refreshEngineStatus() async {
    try {
      final traceId = DateTime.now().millisecondsSinceEpoch.toString();
      final status = await WoxApi.instance.dictationNativeLibStatus(traceId);
      if (!mounted) return;
      if (status != null) {
        setState(() {
          _engineState = status['State'] ?? 'done';
          _engineProgress = (status['Progress'] ?? 0) as int;
          _engineError = status['Error'] ?? '';
          _engineReady = (status['Ready'] ?? true) as bool;
        });
      }
    } catch (e) {
      Logger.instance.error('dictation_engine_status', 'Failed to refresh engine status: $e');
    }
  }

  Future<void> _downloadEngine() async {
    final traceId = DateTime.now().millisecondsSinceEpoch.toString();
    try {
      await WoxApi.instance.dictationNativeLibDownload(traceId);
      setState(() {
        _engineState = 'downloading';
        _engineProgress = 0;
        _engineError = '';
      });
      _startPollingIfDownloading();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _engineError = e.toString().replaceFirst('Exception: ', '');
      });
    }
  }

  // Status polling can return partial rows; keep static model metadata from the original setting option.
  DictationModelOption _mergeStatus(DictationModelOption current, Map<String, dynamic> statusJson) {
    final merged = <String, dynamic>{
      'ID': current.id,
      'DisplayName': current.displayName,
      'Description': current.description,
      'Languages': current.languages,
      'Recommended': current.recommended,
      'Status': statusJson['Status'] ?? _statusValue(current.status),
      'DownloadProgress': statusJson['DownloadProgress'] ?? current.downloadProgress,
      'SizeMB': statusJson['SizeMB'] ?? current.sizeMB,
      'Error': statusJson['Error'] ?? current.error,
    };
    return DictationModelOption.fromJson(merged);
  }

  // Convert enum values back to backend status strings for partial refresh fallbacks.
  String _statusValue(DictationModelStatus status) {
    switch (status) {
      case DictationModelStatus.notDownloaded:
        return 'not_downloaded';
      case DictationModelStatus.downloading:
        return 'downloading';
      case DictationModelStatus.extracting:
        return 'extracting';
      case DictationModelStatus.finalizing:
        return 'finalizing';
      case DictationModelStatus.downloaded:
        return 'downloaded';
      case DictationModelStatus.failed:
        return 'failed';
    }
  }

  void _startPollingIfDownloading() {
    final hasModelActive = _options.any(
      (o) => o.status == DictationModelStatus.downloading || o.status == DictationModelStatus.extracting || o.status == DictationModelStatus.finalizing,
    );
    final hasEngineActive = _engineState == 'downloading' || _engineState == 'extracting';
    final hasActive = hasModelActive || hasEngineActive;
    if (hasActive && _pollTimer == null) {
      _pollTimer = Timer.periodic(const Duration(seconds: 1), (_) async {
        await _refreshStatus();
        await _refreshEngineStatus();
      });
    } else if (!hasActive && _pollTimer != null) {
      _pollTimer?.cancel();
      _pollTimer = null;
    }
  }

  Future<void> _refreshStatus() async {
    try {
      final traceId = DateTime.now().millisecondsSinceEpoch.toString();
      final statuses = await WoxApi.instance.dictationModelStatus(traceId);
      if (!mounted) return;
      setState(() {
        for (final s in statuses) {
          if (s is! Map) {
            continue;
          }
          final statusJson = Map<String, dynamic>.from(s);
          final idx = _options.indexWhere((o) => o.id == statusJson['ID']);
          if (idx >= 0) {
            _options[idx] = _mergeStatus(_options[idx], statusJson);
          }
        }
        if (_selectedId.isEmpty) {
          _selectedId = _findFirstDownloaded();
          if (_selectedId.isNotEmpty) {
            widget.onUpdate(widget.item.key, _selectedId);
          }
        }
      });
      _startPollingIfDownloading();
      _overlayEntry?.markNeedsBuild();
    } catch (e) {
      Logger.instance.error('dictation_model_status', 'Failed to refresh model status: $e');
    }
  }

  Future<void> _downloadModel(String modelId) async {
    final traceId = DateTime.now().millisecondsSinceEpoch.toString();
    try {
      await WoxApi.instance.dictationModelDownload(traceId, modelId);
      setState(() {
        _errorMessage = "";
        final idx = _options.indexWhere((o) => o.id == modelId);
        if (idx >= 0) {
          _options[idx] = DictationModelOption.fromJson({
            'ID': _options[idx].id,
            'DisplayName': _options[idx].displayName,
            'Description': _options[idx].description,
            'Languages': _options[idx].languages,
            'Recommended': _options[idx].recommended,
            'Status': 'downloading',
            'DownloadProgress': 0,
            'SizeMB': _options[idx].sizeMB,
            'Error': '',
          });
        }
      });
      _startPollingIfDownloading();
      _overlayEntry?.markNeedsBuild();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _errorMessage = e.toString().replaceFirst('Exception: ', '');
      });
      _overlayEntry?.markNeedsBuild();
    }
  }

  Future<void> _deleteModel(String modelId) async {
    final traceId = DateTime.now().millisecondsSinceEpoch.toString();
    try {
      await WoxApi.instance.dictationModelDelete(traceId, modelId);
      setState(() {
        _errorMessage = "";
        final idx = _options.indexWhere((o) => o.id == modelId);
        if (idx >= 0) {
          _options[idx] = DictationModelOption.fromJson({
            'ID': _options[idx].id,
            'DisplayName': _options[idx].displayName,
            'Description': _options[idx].description,
            'Languages': _options[idx].languages,
            'Recommended': _options[idx].recommended,
            'Status': 'not_downloaded',
            'DownloadProgress': 0,
            'SizeMB': _options[idx].sizeMB,
            'Error': '',
          });
        }
        // If the deleted model was selected, clear the selection.
        if (_selectedId == modelId) {
          _selectedId = _findFirstDownloaded();
          widget.onUpdate(widget.item.key, _selectedId);
        }
      });
      _overlayEntry?.markNeedsBuild();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _errorMessage = e.toString().replaceFirst('Exception: ', '');
      });
      _overlayEntry?.markNeedsBuild();
    }
  }

  void _selectModel(String modelId) {
    setState(() => _selectedId = modelId);
    widget.onUpdate(widget.item.key, modelId);
    _removeOverlay();
  }

  void _showDropdown() {
    _removeOverlay();
    final renderBox = _dropdownKey.currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return;
    final size = renderBox.size;

    _overlayEntry = OverlayEntry(builder: (context) => _buildDropdownOverlay(size));
    Overlay.of(context).insert(_overlayEntry!);
  }

  void _removeOverlay() {
    _overlayEntry?.remove();
    _overlayEntry = null;
  }

  @override
  Widget build(BuildContext context) {
    final children = <Widget>[];
    if (!_engineReady) {
      children.add(_buildEngineBanner());
    }
    children.add(_buildDropdownButton());
    children.add(validationMessage(_errorMessage));
    return layout(
      label: widget.item.label,
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: children),
      style: widget.item.style,
      tooltip: widget.item.tooltip,
    );
  }

  // Banner showing the native engine download state, displayed above the model
  // dropdown when the native libraries are not yet on disk.
  Widget _buildEngineBanner() {
    final accentColor = getThemeActiveBackgroundColor();
    final subTextColor = getThemeSubTextColor();
    final borderColor = subTextColor.withValues(alpha: 0.55);

    if (_engineState == 'downloading') {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Container(
          decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          child: Row(
            children: [
              SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(accentColor))),
              const SizedBox(width: 8),
              Expanded(child: Text('${tr('plugin_dictation_engine_downloading')} ($_engineProgress%)', style: TextStyle(color: subTextColor, fontSize: 11))),
            ],
          ),
        ),
      );
    }
    if (_engineState == 'extracting') {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Container(
          decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          child: Row(
            children: [
              SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(accentColor))),
              const SizedBox(width: 8),
              Text(tr('plugin_dictation_engine_extracting'), style: TextStyle(color: subTextColor, fontSize: 11)),
            ],
          ),
        ),
      );
    }
    if (_engineState == 'failed') {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Container(
          decoration: BoxDecoration(border: Border.all(color: Colors.red.withValues(alpha: 0.5)), borderRadius: BorderRadius.circular(4)),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          child: Row(
            children: [
              Icon(Icons.error_outline, size: 16, color: Colors.red),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  '${tr('plugin_dictation_engine_download_failed')}: $_engineError',
                  style: TextStyle(color: subTextColor, fontSize: 11),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: 6),
              WoxButton.secondary(text: tr('plugin_dictation_model_retry'), onPressed: _downloadEngine, fontSize: 11),
            ],
          ),
        ),
      );
    }
    // not_downloaded: show download button
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Container(
        decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        child: Row(
          children: [
            Icon(Icons.memory, size: 16, color: subTextColor),
            const SizedBox(width: 6),
            Expanded(child: Text(tr('plugin_dictation_engine_not_downloaded'), style: TextStyle(color: subTextColor, fontSize: 11))),
            const SizedBox(width: 6),
            WoxButton.secondary(text: tr('plugin_dictation_engine_download'), icon: Icon(Icons.download, size: 14, color: accentColor), onPressed: _downloadEngine, fontSize: 11),
          ],
        ),
      ),
    );
  }

  Widget _buildDropdownButton() {
    final selectedOpt = _options.where((o) => o.id == _selectedId).firstOrNull;
    final textColor = getThemeTextColor();
    final borderColor = getThemeSubTextColor().withValues(alpha: 0.55);

    return CompositedTransformTarget(
      link: _layerLink,
      child: GestureDetector(
        key: _dropdownKey,
        behavior: HitTestBehavior.translucent,
        onTap: _showDropdown,
        child: Container(
          decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
          padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  selectedOpt != null ? selectedOpt.displayName : tr('plugin_dictation_model_select_hint'),
                  style: TextStyle(color: selectedOpt != null ? textColor : textColor.withValues(alpha: 0.5), fontSize: 13),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              Icon(Icons.arrow_drop_down, color: textColor.withValues(alpha: 0.7), size: 20),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDropdownOverlay(Size buttonSize) {
    final dropdownBg = getThemePopupSurfaceColor();
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final accentColor = getThemeActiveBackgroundColor();
    final borderColor = getThemeSubTextColor().withValues(alpha: 0.55);

    return GestureDetector(
      behavior: HitTestBehavior.translucent,
      onTap: _removeOverlay,
      child: Stack(
        children: [
          Positioned(
            width: buttonSize.width,
            child: CompositedTransformFollower(
              link: _layerLink,
              showWhenUnlinked: false,
              offset: Offset(0, buttonSize.height),
              child: Material(
                elevation: 8,
                borderRadius: BorderRadius.circular(4),
                color: dropdownBg,
                child: Container(
                  clipBehavior: Clip.antiAlias,
                  constraints: const BoxConstraints(maxHeight: 300),
                  decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
                  child: Column(mainAxisSize: MainAxisSize.min, children: _options.map((opt) => _buildMenuItem(opt, textColor, subTextColor, accentColor, dropdownBg)).toList()),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMenuItem(DictationModelOption opt, Color textColor, Color subTextColor, Color accentColor, Color dropdownBg) {
    final isSelected = _selectedId == opt.id;
    final isDownloaded = opt.status == DictationModelStatus.downloaded;
    final isDownloading = opt.status == DictationModelStatus.downloading;
    final isExtracting = opt.status == DictationModelStatus.extracting;
    final isFinalizing = opt.status == DictationModelStatus.finalizing;
    final isFailed = opt.status == DictationModelStatus.failed;

    return InkWell(
      onTap: isDownloaded ? () => _selectModel(opt.id) : null,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        color: isSelected ? Theme.of(context).focusColor : null,
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Title row: model name + recommended badge + size
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          opt.displayName,
                          style: TextStyle(color: isDownloaded ? textColor : subTextColor, fontSize: 13, fontWeight: FontWeight.w600),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      if (opt.recommended) ...[
                        const SizedBox(width: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                          decoration: BoxDecoration(color: accentColor.withValues(alpha: 0.15), borderRadius: BorderRadius.circular(3)),
                          child: Text(tr('plugin_dictation_model_recommended'), style: TextStyle(color: accentColor, fontSize: 10, fontWeight: FontWeight.w600)),
                        ),
                      ],
                      if (opt.sizeMB > 0) ...[const SizedBox(width: 6), Text('~${opt.sizeMB}MB', style: TextStyle(color: subTextColor, fontSize: 11))],
                    ],
                  ),
                  // Languages
                  if (opt.languages.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(opt.languages, style: TextStyle(color: subTextColor, fontSize: 11), maxLines: 1, overflow: TextOverflow.ellipsis),
                    ),
                  // Description
                  if (opt.description.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 3),
                      child: Text(opt.description, style: TextStyle(color: subTextColor.withValues(alpha: 0.8), fontSize: 11), maxLines: 2, overflow: TextOverflow.ellipsis),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            _buildMenuTrailing(opt, isDownloading, isExtracting, isFinalizing, isDownloaded, isFailed, accentColor, subTextColor),
          ],
        ),
      ),
    );
  }

  Widget _buildMenuTrailing(
    DictationModelOption opt,
    bool isDownloading,
    bool isExtracting,
    bool isFinalizing,
    bool isDownloaded,
    bool isFailed,
    Color accentColor,
    Color subTextColor,
  ) {
    if (isDownloaded) {
      // Downloaded: show only the delete button (no checkmark).
      return GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () => _deleteModel(opt.id),
        child: WoxTooltip(
          message: tr('plugin_dictation_model_delete'),
          child: Padding(padding: const EdgeInsets.all(2), child: Icon(Icons.delete_outline, size: 16, color: subTextColor)),
        ),
      );
    }
    if (isDownloading) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 60,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: LinearProgressIndicator(
                value: opt.downloadProgress / 100.0,
                backgroundColor: getThemeDividerColor(),
                valueColor: AlwaysStoppedAnimation(accentColor),
                minHeight: 4,
              ),
            ),
          ),
          const SizedBox(width: 6),
          SizedBox(width: 30, child: Text('${opt.downloadProgress}%', style: TextStyle(color: subTextColor, fontSize: 11), textAlign: TextAlign.right)),
        ],
      );
    }
    if (isExtracting) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(accentColor))),
          const SizedBox(width: 6),
          Text(tr('plugin_dictation_model_extracting'), style: TextStyle(color: subTextColor, fontSize: 11)),
        ],
      );
    }
    if (isFinalizing) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(accentColor))),
          const SizedBox(width: 6),
          Text(tr('plugin_dictation_model_finalizing'), style: TextStyle(color: subTextColor, fontSize: 11)),
        ],
      );
    }
    if (isFailed) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.error_outline, size: 16, color: Colors.red),
          const SizedBox(width: 4),
          WoxButton.secondary(text: tr('plugin_dictation_model_retry'), onPressed: () => _downloadModel(opt.id), fontSize: 11),
        ],
      );
    }
    // Not downloaded: download button.
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _downloadModel(opt.id),
      child: WoxButton.secondary(
        text: tr('plugin_dictation_model_download'),
        icon: Icon(Icons.download, size: 14, color: accentColor),
        onPressed: () => _downloadModel(opt.id),
        fontSize: 11,
      ),
    );
  }
}
