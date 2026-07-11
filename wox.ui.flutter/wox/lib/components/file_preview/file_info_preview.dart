import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_preview_icon_loader.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxFilePreviewProperty {
  final String label;
  final String value;

  const WoxFilePreviewProperty({required this.label, required this.value});
}

class WoxFileInfoPreview extends StatefulWidget {
  final IconData icon;
  final String? fileIconPath;
  final Color accent;
  final String title;
  final String subtitle;
  final List<WoxFilePreviewProperty> properties;
  final List<Widget> sections;

  const WoxFileInfoPreview({
    super.key,
    required this.icon,
    this.fileIconPath,
    required this.accent,
    required this.title,
    required this.subtitle,
    required this.properties,
    this.sections = const [],
  });

  @override
  State<WoxFileInfoPreview> createState() => _WoxFileInfoPreviewState();
}

class _WoxFileInfoPreviewState extends State<WoxFileInfoPreview> {
  Future<WoxImage?>? _fileIconFuture;

  @override
  void initState() {
    super.initState();
    _fileIconFuture = _createFileIconFuture();
  }

  @override
  void didUpdateWidget(covariant WoxFileInfoPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.fileIconPath != widget.fileIconPath) {
      _fileIconFuture = _createFileIconFuture();
    }
  }

  Future<WoxImage?>? _createFileIconFuture() {
    final fileIconPath = widget.fileIconPath?.trim() ?? "";
    return fileIconPath.isEmpty ? null : loadWoxFilePreviewIcon(fileIconPath);
  }

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final cardColor = getThemeCardBackgroundColor().withValues(alpha: isThemeDark() ? 0.38 : 0.72);
    final borderColor = getThemeDividerColor().withValues(alpha: 0.42);
    final iconBoxSize = metrics.scaledSpacing(52);
    final fileIconSize = metrics.scaledSpacing(40);

    Widget buildFallbackIcon() {
      return Icon(widget.icon, color: widget.accent, size: metrics.scaledSpacing(28));
    }

    Widget buildLoadedIcon(WoxImage? icon) {
      if (icon == null || icon.imageData.trim().isEmpty) {
        return buildFallbackIcon();
      }

      return ClipRRect(borderRadius: BorderRadius.circular(10), child: WoxImageView(woxImage: icon, width: fileIconSize, height: fileIconSize));
    }

    return Padding(
      padding: EdgeInsets.all(metrics.previewTextPadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Container(
                width: iconBoxSize,
                height: iconBoxSize,
                decoration: BoxDecoration(
                  color: widget.accent.withValues(alpha: 0.16),
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: widget.accent.withValues(alpha: 0.28)),
                ),
                child: Center(
                  child:
                      _fileIconFuture == null
                          ? buildFallbackIcon()
                          : FutureBuilder<WoxImage?>(
                            future: _fileIconFuture,
                            builder: (context, snapshot) {
                              return buildLoadedIcon(snapshot.data);
                            },
                          ),
                ),
              ),
              SizedBox(width: metrics.scaledSpacing(14)),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: textColor, fontSize: metrics.previewTextFontSize, fontWeight: FontWeight.w800),
                    ),
                    SizedBox(height: metrics.scaledSpacing(4)),
                    Text(widget.subtitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTextColor, fontSize: metrics.resultSubtitleFontSize)),
                  ],
                ),
              ),
            ],
          ),
          SizedBox(height: metrics.scaledSpacing(18)),
          Wrap(
            spacing: metrics.scaledSpacing(10),
            runSpacing: metrics.scaledSpacing(10),
            children: widget.properties.map((property) => _FilePropertyTile(property: property, cardColor: cardColor, borderColor: borderColor)).toList(),
          ),
          for (final section in widget.sections) ...[SizedBox(height: metrics.scaledSpacing(16)), section],
        ],
      ),
    );
  }
}

class WoxFilePreviewSection extends StatelessWidget {
  final String title;
  final Widget child;

  const WoxFilePreviewSection({super.key, required this.title, required this.child});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final borderColor = getThemeDividerColor().withValues(alpha: 0.42);

    return Container(
      decoration: BoxDecoration(border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(8)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: EdgeInsets.fromLTRB(metrics.scaledSpacing(12), metrics.scaledSpacing(10), metrics.scaledSpacing(12), metrics.scaledSpacing(8)),
            child: Text(
              title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: textColor, fontSize: metrics.resultSubtitleFontSize, fontWeight: FontWeight.w800),
            ),
          ),
          Divider(height: 1, color: borderColor),
          child,
        ],
      ),
    );
  }
}

class _FilePropertyTile extends StatelessWidget {
  final WoxFilePreviewProperty property;
  final Color cardColor;
  final Color borderColor;

  const _FilePropertyTile({required this.property, required this.cardColor, required this.borderColor});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;

    return Container(
      width: metrics.scaledSpacing(190),
      padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(12), vertical: metrics.scaledSpacing(10)),
      decoration: BoxDecoration(color: cardColor, border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(8)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            property.label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: getThemeSubTextColor(), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w600),
          ),
          SizedBox(height: metrics.scaledSpacing(4)),
          Text(
            property.value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: getThemeTextColor(), fontSize: metrics.resultSubtitleFontSize, height: 1.25, fontWeight: FontWeight.w700),
          ),
        ],
      ),
    );
  }
}

// Formats file sizes for compact preview metadata without pulling another
// formatting dependency into the launcher UI.
String formatWoxFilePreviewSize(int bytes) {
  if (bytes < 1024) {
    return "$bytes B";
  }

  const units = ["KB", "MB", "GB", "TB"];
  var value = bytes / 1024.0;
  var unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  return "${value >= 10 ? value.toStringAsFixed(1) : value.toStringAsFixed(2)} ${units[unitIndex]}";
}

String formatWoxFilePreviewDate(DateTime dateTime) {
  final local = dateTime.toLocal();
  String twoDigits(int value) => value.toString().padLeft(2, "0");
  return "${local.year}-${twoDigits(local.month)}-${twoDigits(local.day)} ${twoDigits(local.hour)}:${twoDigits(local.minute)}";
}

List<WoxFilePreviewProperty> buildWoxFilePreviewCommonProperties(File file, {required String typeLabel, required WoxFilePreviewTranslationFormatter tr}) {
  final stat = file.statSync();
  return [
    WoxFilePreviewProperty(label: tr("ui_file_preview_property_type"), value: typeLabel),
    WoxFilePreviewProperty(label: tr("ui_file_preview_property_size"), value: formatWoxFilePreviewSize(stat.size)),
    WoxFilePreviewProperty(label: tr("ui_file_preview_property_modified"), value: formatWoxFilePreviewDate(stat.modified)),
    WoxFilePreviewProperty(label: tr("ui_file_preview_property_location"), value: path.dirname(file.path)),
  ];
}

List<WoxPreviewTag> buildWoxFilePreviewCommonTags(File file, {required String typeLabel, required WoxFilePreviewTranslationFormatter tr}) {
  final stat = file.statSync();
  return [
    WoxPreviewTag(label: typeLabel, tooltip: tr("ui_file_preview_property_type")),
    WoxPreviewTag(label: formatWoxFilePreviewSize(stat.size), tooltip: tr("ui_file_preview_property_size")),
    WoxPreviewTag(label: formatWoxFilePreviewDate(stat.modified), tooltip: tr("ui_file_preview_property_modified")),
    WoxPreviewTag(label: path.dirname(file.path), tooltip: tr("ui_file_preview_property_location")),
  ];
}
