import 'dart:async';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_dictation_model.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxSettingPluginDictationModel extends StatefulWidget {
  final String value;
  final PluginSettingValueDictationModel item;
  final double labelWidth;
  final Future<String?> Function(String key, String value) onUpdate;

  const WoxSettingPluginDictationModel({super.key, required this.value, required this.item, required this.labelWidth, required this.onUpdate});

  @override
  State<WoxSettingPluginDictationModel> createState() => _WoxSettingPluginDictationModelState();
}

class _WoxSettingPluginDictationModelState extends State<WoxSettingPluginDictationModel> {
  late List<DictationModelOption> _options;
  String _selectedId = "";
  Timer? _pollTimer;

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

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
          final idx = _options.indexWhere((o) => o.id == s['id']);
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
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(tr(widget.item.label), style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500)),
          if (widget.item.tooltip.trim().isNotEmpty) ...[const SizedBox(height: 4), Text(tr(widget.item.tooltip), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))],
          const SizedBox(height: 8),
          ..._options.map((opt) => _buildModelRow(opt)),
        ],
      ),
    );
  }

  Widget _buildModelRow(DictationModelOption opt) {
    final isSelected = _selectedId == opt.id;
    final isDownloaded = opt.status == DictationModelStatus.downloaded;
    final isDownloading = opt.status == DictationModelStatus.downloading;
    final isFailed = opt.status == DictationModelStatus.failed;

    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          InkWell(
            onTap:
                isDownloaded
                    ? () {
                      setState(() => _selectedId = opt.id);
                      widget.onUpdate(widget.item.key, opt.id);
                    }
                    : null,
            child: Row(
              children: [
                Icon(
                  isDownloaded && isSelected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                  size: 18,
                  color: isDownloaded ? getThemeActiveBackgroundColor() : getThemeSubTextColor(),
                ),
                const SizedBox(width: 8),
                Text(opt.displayName, style: TextStyle(color: isDownloaded ? getThemeTextColor() : getThemeSubTextColor(), fontSize: 13)),
                if (opt.sizeMB > 0) ...[const SizedBox(width: 4), Text('(${opt.sizeMB}MB)', style: TextStyle(color: getThemeSubTextColor(), fontSize: 11))],
              ],
            ),
          ),
          const Spacer(),
          _buildStatusAction(opt, isDownloading, isDownloaded, isFailed),
        ],
      ),
    );
  }

  Widget _buildStatusAction(DictationModelOption opt, bool isDownloading, bool isDownloaded, bool isFailed) {
    if (isDownloaded) {
      return Icon(Icons.check_circle, size: 16, color: Colors.green);
    }
    if (isDownloading) {
      return Row(
        children: [
          SizedBox(
            width: 60,
            child: LinearProgressIndicator(
              value: opt.downloadProgress / 100.0,
              backgroundColor: getThemeDividerColor(),
              valueColor: AlwaysStoppedAnimation(getThemeActiveBackgroundColor()),
            ),
          ),
          const SizedBox(width: 4),
          Text('${opt.downloadProgress}%', style: TextStyle(color: getThemeSubTextColor(), fontSize: 11)),
        ],
      );
    }
    if (isFailed) {
      return Row(
        children: [
          Icon(Icons.error_outline, size: 16, color: Colors.red),
          const SizedBox(width: 4),
          Flexible(child: Text(opt.error, style: const TextStyle(color: Colors.red, fontSize: 11), overflow: TextOverflow.ellipsis)),
          const SizedBox(width: 8),
          WoxButton.secondary(text: 'Retry', onPressed: () => _downloadModel(opt.id), fontSize: 11),
        ],
      );
    }
    return WoxButton.secondary(
      text: 'Download',
      icon: Icon(Icons.download, size: 14, color: getThemeActiveBackgroundColor()),
      onPressed: () => _downloadModel(opt.id),
      fontSize: 11,
    );
  }
}
