import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

class UpdatePreviewData {
  final String currentVersion;
  final String latestVersion;
  final String releaseChannel;
  final String releaseNotes;
  final String downloadUrl;
  final String status;
  final bool hasUpdate;
  final String error;
  final bool autoUpdateEnabled;

  UpdatePreviewData({
    required this.currentVersion,
    required this.latestVersion,
    required this.releaseChannel,
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
      releaseChannel: json['releaseChannel'] ?? 'stable',
      releaseNotes: json['releaseNotes'] ?? '',
      downloadUrl: json['downloadUrl'] ?? '',
      status: json['status'] ?? '',
      hasUpdate: json['hasUpdate'] ?? false,
      error: json['error'] ?? '',
      autoUpdateEnabled: json['autoUpdateEnabled'] ?? true,
    );
  }
}

class _ReleaseNotesParseResult {
  final List<String> introLines;
  final List<_ReleaseNotesSection> sections;

  _ReleaseNotesParseResult({required this.introLines, required this.sections});

  bool get hasStructuredSections => sections.any((section) => section.items.isNotEmpty);
  String get introMarkdown => introLines.map((line) => line.trimRight()).where((line) => line.trim().isNotEmpty).join('\n');
}

class _ReleaseNotesSection {
  final String title;
  final List<_ReleaseNotesItem> items = [];

  _ReleaseNotesSection(this.title);
}

class _ReleaseNotesItem {
  final String tag;
  final String summary;
  final List<String> continuationLines = [];

  _ReleaseNotesItem({required this.tag, required this.summary});

  String get continuationMarkdown => continuationLines.map((line) => line.trimRight()).where((line) => line.trim().isNotEmpty).join('\n');
}

class _ParsedReleaseNoteItem {
  final String tag;
  final String summary;

  _ParsedReleaseNoteItem({required this.tag, required this.summary});
}

class WoxUpdateView extends StatefulWidget {
  final UpdatePreviewData data;

  const WoxUpdateView({super.key, required this.data});

  @override
  State<WoxUpdateView> createState() => _WoxUpdateViewState();
}

class _WoxUpdateViewState extends State<WoxUpdateView> {
  final releaseNotesScrollController = ScrollController();
  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  @override
  void dispose() {
    releaseNotesScrollController.dispose();
    super.dispose();
  }

  String tr(String key) => Get.find<WoxSettingController>().tr(key);

  Widget statusPill({required String text, required Color color}) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(10), vertical: _metrics.scaledSpacing(4)),
      decoration: BoxDecoration(color: color.withValues(alpha: 0.15), borderRadius: BorderRadius.circular(999), border: Border.all(color: color.withValues(alpha: 0.4))),
      child: Center(child: Text(text, style: TextStyle(color: color, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w600, height: 1.0))),
    );
  }

  String _statusText() {
    if (!widget.data.autoUpdateEnabled) {
      return tr('plugin_update_status_auto_update_disabled');
    }

    if (!widget.data.hasUpdate) {
      final version = widget.data.latestVersion.isNotEmpty ? widget.data.latestVersion : widget.data.currentVersion;
      if (version.isNotEmpty) {
        return Strings.format(tr('plugin_update_status_none_with_version'), [version]);
      }
      return tr('plugin_update_status_none');
    }

    final current = widget.data.currentVersion.isNotEmpty ? widget.data.currentVersion : tr('plugin_update_unknown');
    final latest = widget.data.latestVersion.isNotEmpty ? widget.data.latestVersion : tr('plugin_update_unknown');
    return '$current → $latest';
  }

  Color _statusColor() {
    if (!widget.data.autoUpdateEnabled) {
      return Colors.orange;
    }

    switch (widget.data.status.toLowerCase()) {
      case 'error':
        return Colors.red;
      case 'downloading':
        return Colors.blue;
    }

    if (widget.data.hasUpdate) {
      return Colors.orange;
    }

    return Colors.green;
  }

  bool _isBetaChannel() => widget.data.releaseChannel.toLowerCase() == 'beta';

  @override
  Widget build(BuildContext context) {
    final launcherController = Get.find<WoxLauncherController>();
    final theme = WoxThemeUtil.instance.currentTheme.value;
    final fontColor = safeFromCssColor(theme.previewFontColor);
    final data = widget.data;

    final titleText =
        data.hasUpdate && data.currentVersion.isNotEmpty && data.latestVersion.isNotEmpty
            ? Strings.format(tr('plugin_doctor_version_update_available'), [data.currentVersion, data.latestVersion])
            : tr('plugin_update_title');

    final primaryActionText =
        !data.autoUpdateEnabled
            ? tr('plugin_update_action_enable_auto_update')
            : (data.status.toLowerCase() == 'ready' ? tr('plugin_update_action_apply') : tr('plugin_update_action_check'));
    final primaryHotkey = 'enter';

    if (!data.autoUpdateEnabled) {
      final iconBox = _metrics.scaledSpacing(44);
      final iconGap = _metrics.scaledSpacing(14);
      return Container(
        padding: EdgeInsets.all(_metrics.scaledSpacing(20)),
        child: Center(
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: _metrics.scaledSpacing(760)),
            child: Container(
              padding: EdgeInsets.all(_metrics.scaledSpacing(20)),
              decoration: BoxDecoration(
                color: safeFromCssColor(theme.appBackgroundColor).withValues(alpha: 0.35),
                borderRadius: BorderRadius.circular(14),
                border: Border.all(color: safeFromCssColor(theme.previewSplitLineColor).withValues(alpha: 0.6)),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        width: iconBox,
                        height: iconBox,
                        decoration: BoxDecoration(
                          color: Colors.orange.withValues(alpha: 0.15),
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(color: Colors.orange.withValues(alpha: 0.35)),
                        ),
                        child: Icon(Icons.update, color: Colors.orange, size: _metrics.scaledSpacing(24)),
                      ),
                      SizedBox(width: iconGap),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              tr('plugin_update_auto_update_disabled_title'),
                              style: TextStyle(color: fontColor, fontSize: _metrics.scaledSpacing(18), fontWeight: FontWeight.w700),
                            ),
                            SizedBox(height: _metrics.scaledSpacing(8)),
                            Text(
                              tr('plugin_update_auto_update_disabled_desc'),
                              style: TextStyle(color: fontColor.withValues(alpha: 0.8), fontSize: _metrics.resultSubtitleFontSize, height: 1.4),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: _metrics.scaledSpacing(16)),
                  Padding(
                    padding: EdgeInsets.only(left: iconBox + iconGap),
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: ElevatedButton(
                        onPressed: () {
                          launcherController.executeDefaultAction(const UuidV4().generate());
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
      // Update preview is launcher content, so typography and major spacing
      // follow density while theme-controlled borders/radii/colors remain fixed.
      padding: EdgeInsets.all(_metrics.scaledSpacing(20)),
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
                      style: TextStyle(color: fontColor, fontSize: _metrics.scaledSpacing(18), fontWeight: FontWeight.w700, height: 1.1),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (data.error.isNotEmpty) ...[
                      SizedBox(height: _metrics.scaledSpacing(6)),
                      Text(data.error, style: TextStyle(color: Colors.red, fontSize: _metrics.smallLabelFontSize), overflow: TextOverflow.ellipsis, maxLines: 2),
                    ],
                  ],
                ),
              ),
              SizedBox(width: _metrics.scaledSpacing(12)),
              if (_isBetaChannel()) ...[statusPill(text: tr('plugin_update_release_channel_beta'), color: Colors.blue), SizedBox(width: _metrics.scaledSpacing(8))],
              statusPill(text: _statusText(), color: _statusColor()),
            ],
          ),
          SizedBox(height: _metrics.scaledSpacing(14)),
          Divider(color: safeFromCssColor(theme.previewSplitLineColor)),
          SizedBox(height: _metrics.scaledSpacing(12)),
          Expanded(
            child: Scrollbar(
              thumbVisibility: true,
              controller: releaseNotesScrollController,
              child: SingleChildScrollView(
                controller: releaseNotesScrollController,
                child: _WoxUpdateReleaseNotesView(
                  releaseNotes: data.releaseNotes.isNotEmpty ? data.releaseNotes : tr('plugin_update_no_release_notes'),
                  theme: theme,
                  fontColor: fontColor,
                  fontSize: _metrics.resultSubtitleFontSize,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _WoxUpdateReleaseNotesView extends StatelessWidget {
  final String releaseNotes;
  final WoxTheme theme;
  final Color fontColor;
  final double fontSize;

  const _WoxUpdateReleaseNotesView({required this.releaseNotes, required this.theme, required this.fontColor, required this.fontSize});

  @override
  Widget build(BuildContext context) {
    final parsed = _parseReleaseNotes(releaseNotes);
    if (!parsed.hasStructuredSections) {
      return WoxMarkdownView(data: releaseNotes, fontColor: fontColor, fontSize: fontSize);
    }

    final metrics = WoxInterfaceSizeUtil.instance.current;
    final tagMeasureStyle = TextStyle(fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700, height: 1.2);
    final tagColumnWidth = _measureTagColumnWidth(context, parsed.sections, tagMeasureStyle, maxWidth: metrics.scaledSpacing(180), extraWidth: metrics.scaledSpacing(2));
    final children = <Widget>[];
    if (parsed.introMarkdown.isNotEmpty) {
      children.add(_buildIntro(parsed.introMarkdown));
      children.add(SizedBox(height: metrics.scaledSpacing(14)));
    }

    for (final section in parsed.sections.where((section) => section.items.isNotEmpty)) {
      if (children.isNotEmpty) {
        children.add(SizedBox(height: metrics.scaledSpacing(16)));
      }
      children.add(_ReleaseNotesSectionView(section: section, tagColumnWidth: tagColumnWidth, fontColor: fontColor, fontSize: fontSize));
    }

    return Padding(
      padding: EdgeInsets.only(right: metrics.scaledSpacing(8), bottom: metrics.scaledSpacing(10)),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: children),
    );
  }

  Widget _buildIntro(String markdown) {
    final bgColor = fontColor.withValues(alpha: 0.05);
    final borderColor = safeFromCssColor(theme.previewSplitLineColor).withValues(alpha: 0.45);
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12), vertical: WoxInterfaceSizeUtil.instance.current.scaledSpacing(10)),
      decoration: BoxDecoration(color: bgColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor)),
      child: WoxMarkdownView(data: markdown, fontColor: fontColor.withValues(alpha: 0.9), fontSize: fontSize),
    );
  }

  double _measureTagColumnWidth(BuildContext context, List<_ReleaseNotesSection> sections, TextStyle tagTextStyle, {required double maxWidth, required double extraWidth}) {
    var widest = 0.0;
    for (final section in sections) {
      for (final item in section.items) {
        if (item.tag.isEmpty) {
          continue;
        }

        final painter = TextPainter(text: TextSpan(text: item.tag, style: tagTextStyle), maxLines: 1, textDirection: Directionality.of(context))..layout(maxWidth: maxWidth);
        if (painter.width > widest) {
          widest = painter.width;
        }
      }
    }

    return (widest + extraWidth).clamp(0.0, maxWidth).toDouble();
  }

  // Parses Wox's release-note convention into sections without changing generic markdown previews.
  static _ReleaseNotesParseResult _parseReleaseNotes(String markdown) {
    final introLines = <String>[];
    final sections = <_ReleaseNotesSection>[];
    _ReleaseNotesSection? currentSection;
    _ReleaseNotesItem? currentItem;
    var hasSeenSection = false;

    for (final rawLine in markdown.replaceAll('\r\n', '\n').replaceAll('\r', '\n').split('\n')) {
      final line = rawLine.trimRight();
      if (line.trim().isEmpty) {
        continue;
      }

      final sectionTitle = _parseSectionTitle(line);
      if (sectionTitle != null) {
        currentSection = _ReleaseNotesSection(sectionTitle);
        sections.add(currentSection);
        currentItem = null;
        hasSeenSection = true;
        continue;
      }

      final parsedItem = currentSection == null ? null : _parseItem(line);
      if (parsedItem != null) {
        currentItem = _ReleaseNotesItem(tag: parsedItem.tag, summary: parsedItem.summary);
        currentSection!.items.add(currentItem);
        continue;
      }

      if (currentItem != null) {
        currentItem.continuationLines.add(line.trimLeft());
      } else if (!hasSeenSection) {
        introLines.add(line);
      }
    }

    return _ReleaseNotesParseResult(introLines: introLines, sections: sections);
  }

  // Recognizes the stable top-level headings used by CHANGELOG.md and GitHub release notes.
  static String? _parseSectionTitle(String line) {
    if (line.startsWith(' ') || line.startsWith('\t')) {
      return null;
    }

    final match = RegExp(r'^-\s+(.+?)\s*$').firstMatch(line);
    if (match == null) {
      return null;
    }

    final title = match.group(1)?.trim() ?? '';
    final normalized = title.toLowerCase();
    const knownTitles = {'add', 'added', 'new', 'improve', 'improvements', 'fix', 'fixed', 'fixes', 'changed', 'change', 'remove', 'removed', 'security'};
    return knownTitles.contains(normalized) ? title : null;
  }

  // Extracts the optional [`Area`] prefix so the UI can render it as a compact tag.
  static _ParsedReleaseNoteItem? _parseItem(String line) {
    final itemMatch = RegExp(r'^\s+-\s+(.+)$').firstMatch(line);
    if (itemMatch == null) {
      return null;
    }

    final itemText = itemMatch.group(1)?.trim() ?? '';
    final taggedMatch = RegExp(r'^\[`([^`]+)`\]\s*(.*)$').firstMatch(itemText);
    if (taggedMatch != null) {
      return _ParsedReleaseNoteItem(tag: taggedMatch.group(1)?.trim() ?? '', summary: taggedMatch.group(2)?.trim() ?? '');
    }
    return _ParsedReleaseNoteItem(tag: '', summary: itemText);
  }
}

class _ReleaseNotesSectionView extends StatelessWidget {
  final _ReleaseNotesSection section;
  final double tagColumnWidth;
  final Color fontColor;
  final double fontSize;

  const _ReleaseNotesSectionView({required this.section, required this.tagColumnWidth, required this.fontColor, required this.fontSize});

  @override
  Widget build(BuildContext context) {
    final style = _sectionStyle(section.title);
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final title = style.titleKey.isEmpty ? style.fallbackTitle : Get.find<WoxSettingController>().tr(style.titleKey);
    final tagTextStyle = TextStyle(color: style.color.withValues(alpha: 0.9), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700, height: 1.2);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(children: [Text(title, style: TextStyle(color: fontColor, fontSize: metrics.scaledSpacing(18), fontWeight: FontWeight.w800, height: 1.1))]),
        SizedBox(height: metrics.scaledSpacing(10)),
        ...section.items.map((item) => _ReleaseNotesItemView(item: item, tagColumnWidth: tagColumnWidth, tagTextStyle: tagTextStyle, fontColor: fontColor, fontSize: fontSize)),
      ],
    );
  }

  _ReleaseNotesSectionStyle _sectionStyle(String rawTitle) {
    switch (rawTitle.toLowerCase()) {
      case 'add':
      case 'added':
      case 'new':
        return const _ReleaseNotesSectionStyle(titleKey: 'plugin_update_section_new', icon: Icons.add_circle_outline_rounded, color: Color(0xFF5BCB7B));
      case 'improve':
      case 'improvements':
        return const _ReleaseNotesSectionStyle(titleKey: 'plugin_update_section_improvements', icon: Icons.trending_up_rounded, color: Color(0xFF58A6FF));
      case 'fix':
      case 'fixed':
      case 'fixes':
        return const _ReleaseNotesSectionStyle(titleKey: 'plugin_update_section_fixes', icon: Icons.check_circle_outline_rounded, color: Color(0xFFFFB454));
      case 'security':
        return const _ReleaseNotesSectionStyle(titleKey: 'plugin_update_section_security', icon: Icons.shield_outlined, color: Color(0xFFFF6B7A));
      case 'remove':
      case 'removed':
        return const _ReleaseNotesSectionStyle(titleKey: 'plugin_update_section_removed', icon: Icons.remove_circle_outline, color: Color(0xFFFF7A66));
      case 'change':
      case 'changed':
        return const _ReleaseNotesSectionStyle(titleKey: 'plugin_update_section_changed', icon: Icons.tune, color: Color(0xFFB48CFF));
      default:
        return _ReleaseNotesSectionStyle(titleKey: '', fallbackTitle: rawTitle, icon: Icons.notes, color: fontColor.withValues(alpha: 0.72));
    }
  }
}

class _ReleaseNotesSectionStyle {
  final String titleKey;
  final String fallbackTitle;
  final IconData icon;
  final Color color;

  const _ReleaseNotesSectionStyle({required this.titleKey, this.fallbackTitle = '', required this.icon, required this.color});
}

class _ReleaseNotesItemView extends StatelessWidget {
  final _ReleaseNotesItem item;
  final double tagColumnWidth;
  final TextStyle tagTextStyle;
  final Color fontColor;
  final double fontSize;

  const _ReleaseNotesItemView({required this.item, required this.tagColumnWidth, required this.tagTextStyle, required this.fontColor, required this.fontSize});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final tagGap = tagColumnWidth > 0 ? metrics.scaledSpacing(10) : 0.0;
    return Padding(
      padding: EdgeInsets.only(bottom: metrics.scaledSpacing(10)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (tagColumnWidth > 0)
            SizedBox(
              width: tagColumnWidth,
              child: Padding(padding: EdgeInsets.only(top: metrics.scaledSpacing(2)), child: item.tag.isEmpty ? const SizedBox.shrink() : _buildTag()),
            ),
          if (tagGap > 0) SizedBox(width: tagGap),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                WoxMarkdownView(data: item.summary.trim(), fontColor: fontColor.withValues(alpha: 0.92), fontSize: fontSize, enableImageOverlay: true),
                if (item.continuationMarkdown.isNotEmpty) ...[
                  SizedBox(height: metrics.scaledSpacing(6)),
                  WoxMarkdownView(data: item.continuationMarkdown, fontColor: fontColor.withValues(alpha: 0.92), fontSize: fontSize, enableImageOverlay: true),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTag() {
    return Text(item.tag, maxLines: 1, overflow: TextOverflow.ellipsis, style: tagTextStyle);
  }
}
