import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/utils/wox_theme_util.dart';

class UpdatePreviewData {
  final String currentVersion;
  final String latestVersion;
  final String releaseNotes;
  final String downloadUrl;
  final String status;
  final bool hasUpdate;
  final String error;
  final bool autoUpdateEnabled;

  UpdatePreviewData({
    required this.currentVersion,
    required this.latestVersion,
    required this.releaseNotes,
    required this.downloadUrl,
    required this.status,
    required this.hasUpdate,
    required this.error,
    required this.autoUpdateEnabled,
  });

  factory UpdatePreviewData.fromJson(Map<String, dynamic> json) {
    return UpdatePreviewData(
      currentVersion: json['currentVersion'] ?? '',
      latestVersion: json['latestVersion'] ?? '',
      releaseNotes: json['releaseNotes'] ?? '',
      downloadUrl: json['downloadUrl'] ?? '',
      status: json['status'] ?? '',
      hasUpdate: json['hasUpdate'] ?? false,
      error: json['error'] ?? '',
      autoUpdateEnabled: json['autoUpdateEnabled'] ?? true,
    );
  }
}

class WoxUpdateView extends StatelessWidget {
  final UpdatePreviewData data;

  const WoxUpdateView({super.key, required this.data});

  String _tr(String key) => Get.find<WoxSettingController>().tr(key);

  Widget statusPill({required String text, required Color color}) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color.withValues(alpha: 0.4)),
      ),
      child: Center(
        child: Text(
          text,
          style: TextStyle(
            color: color,
            fontSize: 12,
            fontWeight: FontWeight.w600,
            height: 1.0,
          ),
        ),
      ),
    );
  }

  String _statusText() {
    if (!data.autoUpdateEnabled) {
      return _tr('plugin_update_status_auto_update_disabled');
    }

    if (!data.hasUpdate) {
      return _tr('plugin_update_status_none');
    }

    final current = data.currentVersion.isNotEmpty
        ? data.currentVersion
        : _tr('plugin_update_unknown');
    final latest = data.latestVersion.isNotEmpty
        ? data.latestVersion
        : _tr('plugin_update_unknown');
    return '$current → $latest';
  }

  Color _statusColor() {
    if (!data.autoUpdateEnabled) {
      return Colors.orange;
    }

    switch (data.status.toLowerCase()) {
      case 'error':
        return Colors.red;
      case 'downloading':
        return Colors.blue;
    }

    if (data.hasUpdate) {
      return Colors.orange;
    }

    return Colors.green;
  }

  Widget _infoRow({
    required WoxTheme theme,
    required String label,
    required String value,
  }) {
    final titleColor =
        safeFromCssColor(theme.previewFontColor).withValues(alpha: 0.75);
    final valueColor = safeFromCssColor(theme.previewFontColor);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: TextStyle(color: titleColor, fontSize: 12)),
        const SizedBox(width: 12),
        Flexible(
          child: Text(
            value,
            style: TextStyle(
              color: valueColor,
              fontSize: 12,
              fontWeight: FontWeight.w600,
            ),
            textAlign: TextAlign.right,
            overflow: TextOverflow.ellipsis,
            maxLines: 1,
          ),
        ),
      ],
    );
  }

  Widget versionPanel(WoxTheme theme) {
    final borderColor =
        safeFromCssColor(theme.previewSplitLineColor).withValues(alpha: 0.6);
    final bgColor =
        safeFromCssColor(theme.appBackgroundColor).withValues(alpha: 0.25);

    final current = data.currentVersion.isNotEmpty
        ? data.currentVersion
        : _tr('plugin_update_unknown');
    final latest = data.latestVersion.isNotEmpty
        ? data.latestVersion
        : _tr('plugin_update_unknown');
    final autoUpdateText = data.autoUpdateEnabled
        ? _tr('plugin_update_auto_update_enabled')
        : _tr('plugin_update_auto_update_disabled');

    return Container(
      width: 300,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: borderColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _infoRow(
            theme: theme,
            label: _tr('plugin_update_current_version'),
            value: current,
          ),
          const SizedBox(height: 8),
          _infoRow(
            theme: theme,
            label: _tr('plugin_update_latest_version'),
            value: latest,
          ),
          const SizedBox(height: 8),
          _infoRow(
            theme: theme,
            label: _tr('plugin_update_auto_update_label'),
            value: autoUpdateText,
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final launcherController = Get.find<WoxLauncherController>();
    final theme = WoxThemeUtil.instance.currentTheme.value;
    final fontColor = safeFromCssColor(theme.previewFontColor);

    final titleText = data.hasUpdate &&
            data.currentVersion.isNotEmpty &&
            data.latestVersion.isNotEmpty
        ? Strings.format(_tr('plugin_doctor_version_update_available'),
            [data.currentVersion, data.latestVersion])
        : _tr('plugin_update_title');

    final primaryActionText = !data.autoUpdateEnabled
        ? _tr('plugin_update_action_enable_auto_update')
        : (data.status.toLowerCase() == 'ready'
            ? _tr('plugin_update_action_apply')
            : _tr('plugin_update_action_check'));
    final primaryHotkey = 'enter';

    if (!data.autoUpdateEnabled) {
      const iconBox = 44.0;
      const iconGap = 14.0;
      return Container(
        padding: const EdgeInsets.all(20),
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 760),
            child: Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: safeFromCssColor(theme.appBackgroundColor)
                    .withValues(alpha: 0.35),
                borderRadius: BorderRadius.circular(14),
                border: Border.all(
                  color: safeFromCssColor(theme.previewSplitLineColor)
                      .withValues(alpha: 0.6),
                ),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(
                          color: Colors.orange.withValues(alpha: 0.15),
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(
                            color: Colors.orange.withValues(alpha: 0.35),
                          ),
                        ),
                        child: const Icon(Icons.update, color: Colors.orange),
                      ),
                      const SizedBox(width: 14),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              _tr('plugin_update_auto_update_disabled_title'),
                              style: TextStyle(
                                color: fontColor,
                                fontSize: 18,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                            const SizedBox(height: 8),
                            Text(
                              _tr('plugin_update_auto_update_disabled_desc'),
                              style: TextStyle(
                                color: fontColor.withValues(alpha: 0.8),
                                fontSize: 13,
                                height: 1.4,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  Padding(
                    padding: const EdgeInsets.only(left: iconBox + iconGap),
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: ElevatedButton(
                        onPressed: () {
                          launcherController.executeDefaultAction(
                            const UuidV4().generate(),
                          );
                        },
                        child: Text('$primaryActionText ($primaryHotkey)'),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      titleText,
                      style: TextStyle(
                        color: fontColor,
                        fontSize: 18,
                        fontWeight: FontWeight.w700,
                        height: 1.1,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (data.error.isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text(
                        data.error,
                        style: const TextStyle(
                          color: Colors.red,
                          fontSize: 12,
                        ),
                        overflow: TextOverflow.ellipsis,
                        maxLines: 2,
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 12),
              statusPill(text: _statusText(), color: _statusColor()),
            ],
          ),
          const SizedBox(height: 14),
          Divider(color: safeFromCssColor(theme.previewSplitLineColor)),
          const SizedBox(height: 12),
          Text(
            _tr('plugin_update_release_notes'),
            style: TextStyle(
              color: fontColor,
              fontSize: 14,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 8),
          Expanded(
            child: Scrollbar(
              thumbVisibility: true,
              child: SingleChildScrollView(
                child: WoxMarkdownView(
                  data: data.releaseNotes.isNotEmpty
                      ? data.releaseNotes
                      : _tr('plugin_update_no_release_notes'),
                  fontColor: fontColor,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
