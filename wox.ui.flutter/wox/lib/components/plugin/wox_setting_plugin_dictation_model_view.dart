import 'dart:async';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
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
  Timer? _pollTimer;

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    _options = List.from(widget.item.options);
    _selectedId = widget.value.isNotEmpty ? widget.value : _findFirstDownloaded();
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

  void _startPollingIfDownloading() {
    final hasDownloading = _options.any((o) => o.status == DictationModelStatus.downloading);
    if (hasDownloading && _pollTimer == null) {
      _pollTimer = Timer.periodic(const Duration(seconds: 1), (_) async {
        await _refreshStatus();
      });
    } else if (!hasDownloading && _pollTimer != null) {
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
          final idx = _options.indexWhere((o) => o.id == s['ID']);
          if (idx >= 0) {
            _options[idx] = DictationModelOption.fromJson(s);
          }
        }
        // Auto-select the first downloaded model if nothing is selected yet.
        if (_selectedId.isEmpty) {
          _selectedId = _findFirstDownloaded();
          if (_selectedId.isNotEmpty) {
            widget.onUpdate(widget.item.key, _selectedId);
          }
        }
      });
      _startPollingIfDownloading();
    } catch (e) {
      Logger.instance.error('dictation_model_status', 'Failed to refresh model status: $e');
    }
  }

  Future<void> _downloadModel(String modelId) async {
    final traceId = DateTime.now().millisecondsSinceEpoch.toString();
    try {
      await WoxApi.instance.dictationModelDownload(traceId, modelId);
      setState(() {
        final idx = _options.indexWhere((o) => o.id == modelId);
        if (idx >= 0) {
          _options[idx] = DictationModelOption.fromJson({
            'ID': _options[idx].id,
            'DisplayName': _options[idx].displayName,
            'Status': 'downloading',
            'DownloadProgress': 0,
            'SizeMB': _options[idx].sizeMB,
            'Error': '',
          });
        }
      });
      _startPollingIfDownloading();
    } catch (e) {
      if (!mounted) return;
      Get.snackbar('Error', 'Failed to start download: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return layout(
      label: widget.item.label,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: _options.map((opt) => _buildModelRow(opt)).toList(),
      ),
      style: widget.item.style,
      tooltip: widget.item.tooltip,
    );
  }

  Widget _buildModelRow(DictationModelOption opt) {
    final isSelected = _selectedId == opt.id;
    final isDownloaded = opt.status == DictationModelStatus.downloaded;
    final isDownloading = opt.status == DictationModelStatus.downloading;
    final isFailed = opt.status == DictationModelStatus.failed;
    final accentColor = getThemeActiveBackgroundColor();

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        decoration: BoxDecoration(
          border: Border.all(
            color: isDownloaded && isSelected ? accentColor.withValues(alpha: 0.4) : getThemeDividerColor(),
            width: 1,
          ),
          borderRadius: BorderRadius.circular(6),
          color: isDownloaded && isSelected ? accentColor.withValues(alpha: 0.06) : Colors.transparent,
        ),
        child: Row(
          children: [
            // Radio indicator
            Icon(
              isDownloaded && isSelected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
              size: 18,
              color: isDownloaded ? accentColor : getThemeSubTextColor(),
            ),
            const SizedBox(width: 10),
            // Model name and size
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    opt.displayName,
                    style: TextStyle(
                      color: isDownloaded ? getThemeTextColor() : getThemeSubTextColor(),
                      fontSize: 13,
                      fontWeight: isDownloaded && isSelected ? FontWeight.w600 : FontWeight.normal,
                    ),
                  ),
                  if (opt.sizeMB > 0)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(
                        '${opt.sizeMB}MB',
                        style: TextStyle(color: getThemeSubTextColor(), fontSize: 11),
                      ),
                    ),
                ],
              ),
            ),
            // Status / action
            _buildStatusAction(opt, isDownloading, isDownloaded, isFailed, accentColor),
          ],
        ),
      ),
    );
  }

  Widget _buildStatusAction(DictationModelOption opt, bool isDownloading, bool isDownloaded, bool isFailed, Color accentColor) {
    if (isDownloaded) {
      return GestureDetector(
        onTap: () {
          setState(() => _selectedId = opt.id);
          widget.onUpdate(widget.item.key, opt.id);
        },
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
          decoration: BoxDecoration(
            color: accentColor.withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(4),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.check_circle, size: 14, color: accentColor),
              const SizedBox(width: 4),
              Text(
                tr('plugin_dictation_model_ready'),
                style: TextStyle(color: accentColor, fontSize: 11, fontWeight: FontWeight.w600),
              ),
            ],
          ),
        ),
      );
    }
    if (isDownloading) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 80,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: LinearProgressIndicator(
                value: opt.downloadProgress / 100.0,
                backgroundColor: getThemeDividerColor(),
                valueColor: AlwaysStoppedAnimation(accentColor),
                minHeight: 6,
              ),
            ),
          ),
          const SizedBox(width: 8),
          SizedBox(
            width: 36,
            child: Text(
              '${opt.downloadProgress}%',
              style: TextStyle(color: getThemeSubTextColor(), fontSize: 11),
              textAlign: TextAlign.right,
            ),
          ),
        ],
      );
    }
    if (isFailed) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.error_outline, size: 16, color: Colors.red),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              opt.error,
              style: const TextStyle(color: Colors.red, fontSize: 11),
              overflow: TextOverflow.ellipsis,
            ),
          ),
          const SizedBox(width: 8),
          WoxButton.secondary(
            text: tr('plugin_dictation_model_retry'),
            onPressed: () => _downloadModel(opt.id),
            fontSize: 11,
          ),
        ],
      );
    }
    return WoxButton.secondary(
      text: tr('plugin_dictation_model_download'),
      icon: Icon(Icons.download, size: 14, color: accentColor),
      onPressed: () => _downloadModel(opt.id),
      fontSize: 11,
    );
  }
}