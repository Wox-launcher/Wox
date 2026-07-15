import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/entity/setting/wox_plugin_setting_ocr_model.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginOCRModel extends StatefulWidget {
  final String value;
  final PluginSettingValueOCRModel item;
  final double labelWidth;
  final Future<String?> Function(String key, String value) onUpdate;

  const WoxSettingPluginOCRModel({super.key, required this.value, required this.item, required this.labelWidth, required this.onUpdate});

  @override
  State<WoxSettingPluginOCRModel> createState() => _WoxSettingPluginOCRModelState();
}

class _WoxSettingPluginOCRModelState extends State<WoxSettingPluginOCRModel> with WoxSettingPluginItemMixin<WoxSettingPluginOCRModel> {
  late List<OCRModelOption> _options;
  late String _selectedID;
  String _errorMessage = '';
  Timer? _pollTimer;
  String _engineState = 'done';
  int _engineProgress = 0;
  String _engineError = '';
  bool _engineReady = true;
  final GlobalKey _dropdownKey = GlobalKey();
  final LayerLink _layerLink = LayerLink();
  OverlayEntry? _overlayEntry;

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    _options = List.from(widget.item.options);
    _selectedID = widget.value.isNotEmpty ? widget.value : widget.item.defaultValue;
    _refreshEngineStatus();
    _refreshStatus();
    _startPollingIfNeeded();
  }

  @override
  void didUpdateWidget(covariant WoxSettingPluginOCRModel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.item.options != widget.item.options) {
      _options = List.from(widget.item.options);
    }
    if (oldWidget.value != widget.value && widget.value.isNotEmpty) {
      _selectedID = widget.value;
    }
    _startPollingIfNeeded();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _removeOverlay();
    super.dispose();
  }

  void _startPollingIfNeeded() {
    final modelDownloading = _options.any((option) => option.status == OCRModelStatus.downloading || option.status == OCRModelStatus.finalizing);
    final engineDownloading = _engineState == 'downloading' || _engineState == 'extracting';
    if ((modelDownloading || engineDownloading) && _pollTimer == null) {
      _pollTimer = Timer.periodic(const Duration(seconds: 1), (_) async {
        await _refreshStatus();
        await _refreshEngineStatus();
      });
    } else if (!modelDownloading && !engineDownloading && _pollTimer != null) {
      _pollTimer?.cancel();
      _pollTimer = null;
    }
  }

  Future<void> _refreshEngineStatus() async {
    try {
      final status = await WoxApi.instance.ocrEngineStatus(DateTime.now().millisecondsSinceEpoch.toString());
      if (!mounted || status == null) {
        return;
      }
      setState(() {
        _engineState = status['State'] ?? 'done';
        _engineProgress = (status['Progress'] ?? 0) as int;
        _engineError = status['Error'] ?? '';
        _engineReady = status['Ready'] == true;
      });
      _startPollingIfNeeded();
    } catch (error) {
      Logger.instance.error('ocr_engine_status', 'Failed to refresh OCR engine status: $error');
    }
  }

  Future<void> _downloadEngine() async {
    try {
      await WoxApi.instance.ocrEngineDownload(DateTime.now().millisecondsSinceEpoch.toString());
      if (!mounted) {
        return;
      }
      setState(() {
        _engineState = 'downloading';
        _engineProgress = 0;
        _engineError = '';
      });
      _startPollingIfNeeded();
    } catch (error) {
      if (!mounted) {
        return;
      }
      setState(() {
        _engineState = 'failed';
        _engineError = error.toString().replaceFirst('Exception: ', '');
      });
    }
  }

  Future<void> _refreshStatus() async {
    try {
      final statuses = await WoxApi.instance.ocrModelStatus(DateTime.now().millisecondsSinceEpoch.toString());
      if (!mounted) {
        return;
      }
      setState(() {
        for (final status in statuses) {
          if (status is! Map) {
            continue;
          }
          final current = Map<String, dynamic>.from(status);
          final index = _options.indexWhere((option) => option.id == current['ID']);
          if (index >= 0) {
            _options[index] = _mergeStatus(_options[index], current);
          }
        }
      });
      _startPollingIfNeeded();
      _overlayEntry?.markNeedsBuild();
    } catch (error) {
      Logger.instance.error('ocr_model_status', 'Failed to refresh OCR model status: $error');
    }
  }

  // Keep translated setting metadata; the status endpoint only supplies raw model data.
  OCRModelOption _mergeStatus(OCRModelOption current, Map<String, dynamic> status) {
    return OCRModelOption.fromJson({
      'ID': current.id,
      'DisplayName': current.displayName,
      'Description': current.description,
      'Languages': current.languages,
      'Recommended': current.recommended,
      'Available': current.available,
      'Status': status['Status'] ?? _statusValue(current.status),
      'DownloadProgress': status['DownloadProgress'] ?? current.downloadProgress,
      'SizeMB': status['SizeMB'] ?? current.sizeMB,
      'Error': status['Error'] ?? current.error,
    });
  }

  String _statusValue(OCRModelStatus status) {
    switch (status) {
      case OCRModelStatus.notDownloaded:
        return 'not_downloaded';
      case OCRModelStatus.downloading:
        return 'downloading';
      case OCRModelStatus.finalizing:
        return 'finalizing';
      case OCRModelStatus.downloaded:
        return 'downloaded';
      case OCRModelStatus.failed:
        return 'failed';
    }
  }

  Future<void> _download(OCRModelOption option) async {
    try {
      await WoxApi.instance.ocrModelDownload(DateTime.now().millisecondsSinceEpoch.toString(), option.id);
      if (!mounted) {
        return;
      }
      setState(() {
        _errorMessage = '';
        final index = _options.indexWhere((candidate) => candidate.id == option.id);
        if (index >= 0) {
          _options[index] = _mergeStatus(_options[index], {'Status': 'downloading', 'DownloadProgress': 0, 'Error': ''});
        }
      });
      _startPollingIfNeeded();
      _overlayEntry?.markNeedsBuild();
    } catch (error) {
      if (!mounted) {
        return;
      }
      setState(() => _errorMessage = error.toString().replaceFirst('Exception: ', ''));
      _overlayEntry?.markNeedsBuild();
    }
  }

  void _select(OCRModelOption option) {
    if (!option.available || option.status != OCRModelStatus.downloaded) {
      return;
    }
    setState(() => _selectedID = option.id);
    widget.onUpdate(widget.item.key, option.id);
    _removeOverlay();
  }

  void _showDropdown() {
    _removeOverlay();
    final renderBox = _dropdownKey.currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) {
      return;
    }
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
      tooltip: widget.item.tooltip,
      style: widget.item.style,
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: children),
    );
  }

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
              Expanded(child: Text('${tr('plugin_ocr_engine_downloading')} ($_engineProgress%)', style: TextStyle(color: subTextColor, fontSize: 11))),
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
              Text(tr('plugin_ocr_engine_extracting'), style: TextStyle(color: subTextColor, fontSize: 11)),
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
              const Icon(Icons.error_outline, size: 16, color: Colors.red),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  '${tr('plugin_ocr_engine_download_failed')}: $_engineError',
                  style: TextStyle(color: subTextColor, fontSize: 11),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: 6),
              WoxButton.secondary(text: tr('plugin_ocr_model_retry'), onPressed: _downloadEngine, fontSize: 11),
            ],
          ),
        ),
      );
    }
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Container(
        decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        child: Row(
          children: [
            Icon(Icons.memory, size: 16, color: subTextColor),
            const SizedBox(width: 6),
            Expanded(child: Text(tr('plugin_ocr_engine_not_downloaded'), style: TextStyle(color: subTextColor, fontSize: 11))),
            const SizedBox(width: 6),
            WoxButton.secondary(text: tr('plugin_ocr_engine_download'), icon: Icon(Icons.download, size: 14, color: accentColor), onPressed: _downloadEngine, fontSize: 11),
          ],
        ),
      ),
    );
  }

  Widget _buildDropdownButton() {
    final selectedOption = _options.where((option) => option.id == _selectedID).firstOrNull;
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
          padding: const EdgeInsets.fromLTRB(8, 4, 8, 4),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  selectedOption?.displayName ?? tr('plugin_dictation_model_select_hint'),
                  style: TextStyle(color: selectedOption?.available == false ? textColor.withValues(alpha: 0.5) : textColor, fontSize: 13),
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
    final dropdownBackground = getThemePopupSurfaceColor();
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final accentColor = getThemeActiveBackgroundColor();
    final borderColor = subTextColor.withValues(alpha: 0.55);

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
                color: dropdownBackground,
                child: Container(
                  clipBehavior: Clip.antiAlias,
                  constraints: const BoxConstraints(maxHeight: 300),
                  decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(4)),
                  child: Column(mainAxisSize: MainAxisSize.min, children: _options.map((option) => _buildMenuItem(option, textColor, subTextColor, accentColor)).toList()),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMenuItem(OCRModelOption option, Color textColor, Color subTextColor, Color accentColor) {
    final selected = _selectedID == option.id;
    final downloaded = option.status == OCRModelStatus.downloaded;
    final selectable = option.available && downloaded;

    return InkWell(
      onTap: selectable ? () => _select(option) : null,
      child: Container(
        color: selected ? Theme.of(context).focusColor : null,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          option.displayName,
                          style: TextStyle(color: selectable ? textColor : subTextColor, fontSize: 13, fontWeight: FontWeight.w600),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      if (option.recommended) ...[
                        const SizedBox(width: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                          decoration: BoxDecoration(color: accentColor.withValues(alpha: 0.15), borderRadius: BorderRadius.circular(3)),
                          child: Text(tr('plugin_dictation_model_recommended'), style: TextStyle(color: accentColor, fontSize: 10, fontWeight: FontWeight.w600)),
                        ),
                      ],
                      if (option.sizeMB > 0) ...[const SizedBox(width: 6), Text('~${option.sizeMB}MB', style: TextStyle(color: subTextColor, fontSize: 11))],
                    ],
                  ),
                  if (option.languages.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(option.languages, style: TextStyle(color: subTextColor, fontSize: 11), maxLines: 1, overflow: TextOverflow.ellipsis),
                    ),
                  if (option.description.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 3),
                      child: Text(option.description, style: TextStyle(color: subTextColor.withValues(alpha: 0.8), fontSize: 11), maxLines: 2, overflow: TextOverflow.ellipsis),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            _buildMenuTrailing(option, accentColor, subTextColor),
          ],
        ),
      ),
    );
  }

  Widget _buildMenuTrailing(OCRModelOption option, Color accentColor, Color subTextColor) {
    if (!option.available) {
      return Text(tr('plugin_ocr_model_unavailable'), style: TextStyle(color: subTextColor, fontSize: 11));
    }
    if (option.status == OCRModelStatus.downloading) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 60,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: LinearProgressIndicator(
                value: option.downloadProgress / 100,
                backgroundColor: getThemeDividerColor(),
                valueColor: AlwaysStoppedAnimation(accentColor),
                minHeight: 4,
              ),
            ),
          ),
          const SizedBox(width: 6),
          SizedBox(width: 30, child: Text('${option.downloadProgress}%', style: TextStyle(color: subTextColor, fontSize: 11), textAlign: TextAlign.right)),
        ],
      );
    }
    if (option.status == OCRModelStatus.finalizing) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(accentColor))),
          const SizedBox(width: 6),
          Text(tr('plugin_ocr_model_finalizing'), style: TextStyle(color: subTextColor, fontSize: 11)),
        ],
      );
    }
    if (option.status == OCRModelStatus.failed) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.error_outline, size: 16, color: Colors.red),
          const SizedBox(width: 4),
          WoxButton.secondary(text: tr('plugin_ocr_model_retry'), onPressed: () => _download(option), fontSize: 11),
        ],
      );
    }
    if (option.status == OCRModelStatus.notDownloaded) {
      return WoxButton.secondary(text: tr('plugin_ocr_model_download'), icon: Icon(Icons.download, size: 14, color: accentColor), onPressed: () => _download(option), fontSize: 11);
    }
    return const SizedBox.shrink();
  }
}
