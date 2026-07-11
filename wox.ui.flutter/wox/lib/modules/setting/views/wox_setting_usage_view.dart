import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:get/get.dart';
import 'package:path_provider/path_provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_panel.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_usage_stats.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/screenshot/screenshot_platform_bridge.dart';

class WoxSettingUsageView extends StatefulWidget {
  const WoxSettingUsageView({super.key});

  @override
  State<WoxSettingUsageView> createState() => _WoxSettingUsageViewState();
}

class _WoxSettingUsageViewState extends State<WoxSettingUsageView> {
  final WoxSettingController controller = Get.find<WoxSettingController>();
  late final MemoryImage _woxIconImage = MemoryImage(base64Decode(WOX_ICON.split(';base64,').last));
  bool _isSharingUsage = false;
  String _shareStatusMessage = '';
  bool _shareStatusIsError = false;

  @override
  void initState() {
    super.initState();
    // Usage numbers can change while the settings window is open. Refreshing when this tab is
    // mounted keeps the page current without exposing a manual refresh button for a passive report.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      unawaited(controller.refreshUsageStats());
    });
  }

  Widget _form({double width = GENERAL_SETTING_COMPACT_FORM_WIDTH, required List<Widget> children}) {
    return Align(
      alignment: Alignment.topLeft,
      child: SingleChildScrollView(
        child: SizedBox(
          width: width,
          child: Padding(
            padding: const EdgeInsets.only(left: 38, right: 44, bottom: 30, top: 34),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: children),
          ),
        ),
      ),
    );
  }

  Color _panelColor() {
    if (isThemeDark()) {
      return getThemePanelBackgroundColor().lighter(4);
    }
    return Colors.white;
  }

  Color _outlineColor() {
    return isThemeDark() ? Colors.white.withValues(alpha: 0.08) : const Color(0xFFE6EAF0);
  }

  Color _shareImageBackgroundColor() {
    // The capture overlay must use an opaque color. Hiding the overlay with opacity previously made
    // the exported PNG translucent, which looked wrong after pasting into X or image viewers.
    return isThemeDark() ? const Color(0xFF202733) : const Color(0xFFF6F8FB);
  }

  Widget _shareButton({required bool disabled, required VoidCallback onPressed}) {
    final textColor = disabled ? getThemeSubTextColor().withValues(alpha: 0.55) : getThemeTextColor();
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: disabled ? null : onPressed,
        borderRadius: BorderRadius.circular(8),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(color: _panelColor(), borderRadius: BorderRadius.circular(8), border: Border.all(color: _outlineColor())),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.ios_share, size: 15, color: textColor),
              const SizedBox(width: 8),
              Text(controller.tr('ui_usage_share_x'), style: TextStyle(color: textColor, fontSize: 13, fontWeight: FontWeight.w600)),
            ],
          ),
        ),
      ),
    );
  }

  List<_UsagePeriodOption> get _periodOptions {
    return [
      _UsagePeriodOption(code: '7d', label: controller.tr('ui_usage_period_7d')),
      _UsagePeriodOption(code: '30d', label: controller.tr('ui_usage_period_30d')),
      _UsagePeriodOption(code: '365d', label: controller.tr('ui_usage_period_365d')),
      _UsagePeriodOption(code: 'all', label: controller.tr('ui_usage_period_all')),
    ];
  }

  String _periodLabel(String period) {
    for (final option in _periodOptions) {
      if (option.code == period) {
        return option.label;
      }
    }
    return controller.tr('ui_usage_period_30d');
  }

  String _overviewLabel(String period) {
    return controller.tr('ui_usage_overview').replaceAll('{period}', _periodLabel(period));
  }

  Widget _periodSelector({required String selectedPeriod, required bool disabled}) {
    return Container(
      padding: const EdgeInsets.all(3),
      decoration: BoxDecoration(color: _panelColor(), borderRadius: BorderRadius.circular(8), border: Border.all(color: _outlineColor())),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children:
            _periodOptions.map((option) {
              final selected = option.code == selectedPeriod;
              final textColor = selected ? getThemeTextColor() : getThemeSubTextColor();
              return Material(
                color: Colors.transparent,
                child: InkWell(
                  onTap: disabled || selected ? null : () => unawaited(controller.refreshUsageStats(period: option.code)),
                  borderRadius: BorderRadius.circular(6),
                  child: AnimatedContainer(
                    duration: const Duration(milliseconds: 120),
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                    decoration: BoxDecoration(
                      // Period is a dashboard filter, not a navigation tab. A segmented control keeps the
                      // range choice close to the share action while avoiding a bulky dropdown that would
                      // hide the available reporting scopes.
                      color: selected ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.36 : 0.16) : Colors.transparent,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      option.label,
                      style: TextStyle(
                        color: disabled && !selected ? textColor.withValues(alpha: 0.5) : textColor,
                        fontSize: 12,
                        fontWeight: selected ? FontWeight.w700 : FontWeight.w600,
                      ),
                    ),
                  ),
                ),
              );
            }).toList(),
      ),
    );
  }

  Widget _statCard({required String title, required String value, required IconData icon, required Color accentColor}) {
    return WoxPanel(
      height: 92,
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Row(
            children: [
              Container(
                width: 46,
                height: 46,
                decoration: BoxDecoration(color: accentColor.withValues(alpha: isThemeDark() ? 0.22 : 0.12), borderRadius: BorderRadius.circular(8)),
                child: Icon(icon, size: 22, color: accentColor),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(title, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w600), overflow: TextOverflow.ellipsis),
                    const SizedBox(height: 6),
                    Text(value, style: TextStyle(color: getThemeTextColor(), fontSize: 22, fontWeight: FontWeight.w700, height: 1.0)),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _dashboardPanel({required String title, required IconData icon, required Widget child, String? footer, double? height, Alignment childAlignment = Alignment.topLeft}) {
    final content = height == null ? child : Expanded(child: Align(alignment: childAlignment, child: child));

    return WoxPanel(
      height: height,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 16, color: getThemeTextColor()),
              const SizedBox(width: 8),
              Expanded(child: Text(title, style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w700), overflow: TextOverflow.ellipsis)),
            ],
          ),
          const SizedBox(height: 14),
          content,
          if (footer != null) ...[const SizedBox(height: 12), Text(footer, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w500))],
        ],
      ),
    );
  }

  // Builds a GitHub-style daily activity grid so each visible cell maps to one local calendar day.
  Widget _dailyHeatmap({required List<WoxUsageStatsDay> data, required Color accentColor}) {
    final days = _buildLatestYearHeatmapDays(data);

    if (days.isEmpty) {
      return SizedBox(
        height: 118,
        child: Align(alignment: Alignment.centerLeft, child: Text(controller.tr('ui_usage_no_data'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))),
      );
    }

    final maxCount = days.map((day) => day.count).reduce(math.max);
    final thresholds = _heatmapThresholds(days);
    final firstOffset = _heatmapWeekdayIndex(days.first.date);
    final totalCells = firstOffset + days.length;
    final weekCount = math.max(1, (totalCells / 7).ceil());
    const monthHeight = 18.0;
    final dayCells = List<_UsageHeatmapDay?>.filled(weekCount * 7, null);

    for (var i = 0; i < days.length; i++) {
      dayCells[firstOffset + i] = days[i];
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        const cellGap = 2.5;
        const horizontalInset = 4.0;
        final availableGridWidth = math.max(0.0, constraints.maxWidth - horizontalInset * 2);
        // Windows CI can render the settings content a few pixels narrower than
        // macOS. Let the yearly heatmap cells shrink slightly instead of forcing
        // a one-row overflow when 53 weeks are visible.
        final cellSize = ((availableGridWidth - cellGap * (weekCount - 1)) / weekCount).clamp(5.0, 13.0).toDouble();
        final weekStride = cellSize + cellGap;
        final gridWidth = weekCount * cellSize + (weekCount - 1) * cellGap;
        final gridHeight = cellSize * 7 + cellGap * 6;

        return Padding(
          padding: const EdgeInsets.symmetric(horizontal: horizontalInset),
          child: Align(
            alignment: Alignment.center,
            child: SizedBox(
              width: gridWidth,
              height: monthHeight + gridHeight,
              child: Stack(
                children: [
                  ..._heatmapMonthLabels(days, firstOffset, weekStride, gridWidth).map(
                    (label) =>
                        Positioned(left: label.left, top: 0, child: Text(label.text, style: TextStyle(color: getThemeSubTextColor(), fontSize: 10, fontWeight: FontWeight.w600))),
                  ),
                  Positioned(
                    left: 0,
                    right: 0,
                    bottom: 0,
                    height: gridHeight,
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: List.generate(weekCount, (week) {
                        return Padding(
                          padding: EdgeInsets.only(right: week == weekCount - 1 ? 0 : cellGap),
                          child: Column(
                            children: List.generate(7, (row) {
                              final day = dayCells[week * 7 + row];
                              return Padding(
                                padding: EdgeInsets.only(bottom: row == 6 ? 0 : cellGap),
                                child: _heatmapCell(day: day, maxCount: maxCount, thresholds: thresholds, accentColor: accentColor, size: cellSize),
                              );
                            }),
                          ),
                        );
                      }),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  // Keep the visual range fixed even if a stale backend returns all-time daily buckets.
  List<_UsageHeatmapDay> _buildLatestYearHeatmapDays(List<WoxUsageStatsDay> data) {
    final countsByDate = <String, int>{};
    for (final day in data) {
      final date = _parseUsageDate(day.date);
      if (date == null) {
        continue;
      }
      countsByDate[_formatUsageDate(date)] = day.count;
    }

    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final start = DateTime(today.year - 1, today.month, today.day);
    final days = <_UsageHeatmapDay>[];
    for (var day = start; !day.isAfter(today); day = day.add(const Duration(days: 1))) {
      days.add(_UsageHeatmapDay(date: day, count: countsByDate[_formatUsageDate(day)] ?? 0));
    }
    return days;
  }

  Widget _heatmapCell({required _UsageHeatmapDay? day, required int maxCount, required _HeatmapThresholds thresholds, required Color accentColor, required double size}) {
    final color = day == null ? Colors.transparent : _heatmapColor(day.count, maxCount, thresholds, accentColor);
    final borderColor = day == null ? Colors.transparent : _outlineColor();
    final cell = AnimatedContainer(
      duration: const Duration(milliseconds: 120),
      width: size,
      height: size,
      decoration: BoxDecoration(color: color, borderRadius: BorderRadius.circular(3), border: Border.all(color: borderColor)),
    );

    if (day == null) {
      return cell;
    }

    return WoxTooltip(message: _heatmapTooltip(day), waitDuration: const Duration(milliseconds: 180), child: MouseRegion(cursor: SystemMouseCursors.click, child: cell));
  }

  Color _heatmapColor(int count, int maxCount, _HeatmapThresholds thresholds, Color accentColor) {
    final emptyColor = isThemeDark() ? Colors.white.withValues(alpha: 0.07) : const Color(0xFFE8EDF3);
    if (count <= 0 || maxCount <= 0) {
      return emptyColor;
    }

    final level =
        count > thresholds.high
            ? 4.0
            : count > thresholds.medium
            ? 3.0
            : count > thresholds.low
            ? 2.0
            : 1.0;
    final baseAlpha = isThemeDark() ? 0.22 : 0.18;
    final alpha = baseAlpha + level * (isThemeDark() ? 0.16 : 0.18);
    return accentColor.withValues(alpha: alpha.clamp(0.0, 1.0).toDouble());
  }

  // Quartile thresholds keep the heatmap relative to the user's own yearly activity distribution.
  _HeatmapThresholds _heatmapThresholds(List<_UsageHeatmapDay> days) {
    final positiveCounts = days.map((day) => day.count).where((count) => count > 0).toList()..sort();
    if (positiveCounts.isEmpty) {
      return const _HeatmapThresholds(low: 0, medium: 0, high: 0);
    }

    int percentile(double value) {
      final index = ((positiveCounts.length - 1) * value).floor().clamp(0, positiveCounts.length - 1);
      return positiveCounts[index];
    }

    return _HeatmapThresholds(low: percentile(0.25), medium: percentile(0.50), high: percentile(0.75));
  }

  List<_HeatmapMonthLabel> _heatmapMonthLabels(List<_UsageHeatmapDay> days, int firstOffset, double weekStride, double gridWidth) {
    final labels = <_HeatmapMonthLabel>[];
    final seenMonths = <String>{};
    for (var i = 0; i < days.length; i++) {
      final date = days[i].date;
      final key = '${date.year}-${date.month}';
      if (!seenMonths.add(key)) {
        continue;
      }

      final column = (firstOffset + i) ~/ 7;
      final left = math.min(column * weekStride, math.max(0, gridWidth - 32));
      labels.add(_HeatmapMonthLabel(left: left.toDouble(), text: _monthLabel(date.month)));
    }
    return labels;
  }

  String _heatmapTooltip(_UsageHeatmapDay day) {
    return controller.tr('ui_usage_day_opened_count').replaceAll('{date}', _formatUsageDate(day.date)).replaceAll('{count}', day.count.toString());
  }

  DateTime? _parseUsageDate(String value) {
    final parts = value.split('-');
    if (parts.length != 3) {
      return null;
    }

    final year = int.tryParse(parts[0]);
    final month = int.tryParse(parts[1]);
    final day = int.tryParse(parts[2]);
    if (year == null || month == null || day == null) {
      return null;
    }
    return DateTime(year, month, day);
  }

  int _heatmapWeekdayIndex(DateTime date) {
    return date.weekday % 7;
  }

  String _formatUsageDate(DateTime date) {
    return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
  }

  String _monthLabel(int month) {
    return controller.tr('ui_month_short_$month');
  }

  Widget _itemIcon(WoxUsageStatsItem item, Color accentColor) {
    if (item.icon.imageData.isNotEmpty) {
      return ClipRRect(borderRadius: BorderRadius.circular(4), child: WoxImageView(woxImage: item.icon, width: 18, height: 18));
    }

    return Container(
      width: 18,
      height: 18,
      decoration: BoxDecoration(color: accentColor.withValues(alpha: isThemeDark() ? 0.18 : 0.10), borderRadius: BorderRadius.circular(4)),
      child: Icon(Icons.apps_outlined, size: 13, color: accentColor),
    );
  }

  Widget _topList({
    required String title,
    required IconData titleIcon,
    required List<WoxUsageStatsItem> items,
    required String emptyText,
    required Color accentColor,
    bool showItemIcons = false,
  }) {
    final maxCount = items.isEmpty ? 0 : items.map((e) => e.count).reduce(math.max);

    return _dashboardPanel(
      title: title,
      icon: titleIcon,
      child:
          items.isEmpty
              ? SizedBox(height: 72, child: Align(alignment: Alignment.centerLeft, child: Text(emptyText, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))))
              : Column(
                children:
                    items.take(10).toList().asMap().entries.map((entry) {
                      final rank = entry.key + 1;
                      final e = entry.value;
                      final name = e.name.isNotEmpty ? e.name : e.id;
                      final progress = maxCount == 0 ? 0.0 : e.count / maxCount;
                      return Padding(
                        padding: const EdgeInsets.symmetric(vertical: 5),
                        child: SizedBox(
                          height: 24,
                          child: Row(
                            children: [
                              SizedBox(
                                width: 24,
                                child: Text(
                                  _rankLabel(rank),
                                  textAlign: TextAlign.center,
                                  style: TextStyle(color: getThemeSubTextColor(), fontSize: rank <= 3 ? 14 : 12, fontWeight: FontWeight.w600),
                                ),
                              ),
                              if (showItemIcons) ...[_itemIcon(e, accentColor), const SizedBox(width: 8)],
                              Expanded(
                                flex: 5,
                                child: Text(name, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500), overflow: TextOverflow.ellipsis),
                              ),
                              const SizedBox(width: 12),
                              // The accepted design puts ranking text on the left and uses a thin
                              // progress meter on the right. Keeping progress out of the row
                              // background prevents long app names from fighting with colored bars.
                              Expanded(
                                flex: 4,
                                child: ClipRRect(
                                  borderRadius: BorderRadius.circular(99),
                                  child: Stack(
                                    children: [
                                      Container(
                                        height: 3,
                                        margin: const EdgeInsets.symmetric(vertical: 10.5),
                                        color: getThemeTextColor().withValues(alpha: isThemeDark() ? 0.08 : 0.055),
                                      ),
                                      FractionallySizedBox(
                                        widthFactor: progress.clamp(0.0, 1.0),
                                        child: Container(
                                          height: 3,
                                          margin: const EdgeInsets.symmetric(vertical: 10.5),
                                          decoration: BoxDecoration(color: accentColor.withValues(alpha: isThemeDark() ? 0.72 : 0.68), borderRadius: BorderRadius.circular(99)),
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                              ),
                              const SizedBox(width: 10),
                              SizedBox(
                                width: 32,
                                child: Text(
                                  e.count.toString(),
                                  textAlign: TextAlign.right,
                                  style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w700),
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ],
                          ),
                        ),
                      );
                    }).toList(),
              ),
    );
  }

  String _rankLabel(int rank) {
    switch (rank) {
      case 1:
        return '🏆';
      case 2:
        return '🥈';
      case 3:
        return '🥉';
      default:
        return rank.toString();
    }
  }

  String _shareImageTitle() {
    return controller.tr('ui_usage_share_image_title');
  }

  Widget _usageDashboardBody({required WoxUsageStats stats, required String selectedPeriod, required String error}) {
    const blueAccent = Color(0xFF3B82F6);
    const tealAccent = Color(0xFF14B8A6);
    const amberAccent = Color(0xFFF59E0B);
    const violetAccent = Color(0xFF8B5CF6);
    const greenAccent = Color(0xFF22C55E);
    const heatmapPanelHeight = 252.0;
    // KPI cards can keep their semantic accents, but the lower analytics panels need a calmer
    // reading rhythm. Reusing blue for app/time panels and violet for plugin panels matches the
    // accepted mockup more closely than giving every card its own dominant color.
    const appPanelAccent = blueAccent;
    const pluginPanelAccent = violetAccent;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (error.isNotEmpty) Padding(padding: const EdgeInsets.only(bottom: 10), child: Text(error, style: const TextStyle(color: Colors.red, fontSize: 12))),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth.isFinite ? constraints.maxWidth : GENERAL_SETTING_COMPACT_FORM_WIDTH;
            final spacing = 12.0;
            final columns = width >= 760 ? 4 : 2;
            final cardWidth = (width - (columns - 1) * spacing) / columns;

            return Wrap(
              spacing: spacing,
              runSpacing: spacing,
              children: [
                SizedBox(
                  width: cardWidth,
                  child: _statCard(title: controller.tr('ui_usage_opened'), value: stats.periodOpened.toString(), icon: Icons.visibility_outlined, accentColor: blueAccent),
                ),
                SizedBox(
                  width: cardWidth,
                  child: _statCard(
                    title: controller.tr('ui_usage_app_launches'),
                    value: stats.periodAppLaunch.toString(),
                    icon: Icons.rocket_launch_outlined,
                    accentColor: tealAccent,
                  ),
                ),
                SizedBox(
                  width: cardWidth,
                  child: _statCard(title: controller.tr('ui_usage_apps_used'), value: stats.periodAppsUsed.toString(), icon: Icons.apps_outlined, accentColor: amberAccent),
                ),
                SizedBox(
                  width: cardWidth,
                  child: _statCard(title: controller.tr('ui_usage_actions'), value: stats.periodActions.toString(), icon: Icons.bolt_outlined, accentColor: violetAccent),
                ),
              ],
            );
          },
        ),
        const SizedBox(height: 18),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth.isFinite ? constraints.maxWidth : GENERAL_SETTING_COMPACT_FORM_WIDTH;

            return SizedBox(
              width: width,
              child: _dashboardPanel(
                title: controller.tr('ui_usage_opened_by_day'),
                icon: Icons.calendar_today_outlined,
                child: _dailyHeatmap(data: stats.openedByDay, accentColor: greenAccent),
                height: heatmapPanelHeight,
                childAlignment: Alignment.center,
              ),
            );
          },
        ),
        const SizedBox(height: 18),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth.isFinite ? constraints.maxWidth : GENERAL_SETTING_COMPACT_FORM_WIDTH;
            final spacing = 12.0;
            final columns = width >= 760 ? 2 : 1;
            final blockWidth = columns == 1 ? width : (width - spacing) / columns;
            final emptyText = controller.tr('ui_usage_no_data');

            return Wrap(
              spacing: spacing,
              runSpacing: spacing,
              children: [
                SizedBox(
                  width: blockWidth,
                  child: _topList(
                    title: controller.tr('ui_usage_top_apps'),
                    titleIcon: Icons.apps_outlined,
                    items: stats.topApps,
                    emptyText: emptyText,
                    accentColor: appPanelAccent,
                    showItemIcons: true,
                  ),
                ),
                SizedBox(
                  width: blockWidth,
                  child: _topList(
                    title: controller.tr('ui_usage_top_plugins'),
                    titleIcon: Icons.extension_outlined,
                    items: stats.topPlugins,
                    emptyText: emptyText,
                    accentColor: pluginPanelAccent,
                  ),
                ),
              ],
            );
          },
        ),
      ],
    );
  }

  Widget _usageSummaryHeader({required bool isLoading, required bool disableShare, required String selectedPeriod}) {
    final titleBlock = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(controller.tr('ui_usage'), style: TextStyle(color: getThemeTextColor(), fontSize: 21, fontWeight: FontWeight.w800, height: 1.1)),
            if (isLoading) ...[const SizedBox(width: 10), const WoxLoadingIndicator(size: 16)],
          ],
        ),
        const SizedBox(height: 6),
        Text(_overviewLabel(selectedPeriod), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, fontWeight: FontWeight.w500)),
      ],
    );

    final shareAction =
        _isSharingUsage ? const Padding(padding: EdgeInsets.only(top: 6), child: WoxLoadingIndicator(size: 16)) : _shareButton(disabled: disableShare, onPressed: _shareUsageToX);
    final periodSelector = _periodSelector(selectedPeriod: selectedPeriod, disabled: isLoading);

    // The period selector is a page-level filter, not part of the share action. It stays centered in
    // the whole header instead of participating in a Wrap; the previous width threshold was too
    // conservative and pushed the selector onto a second row on normal settings-window sizes.
    return SizedBox(
      height: 54,
      child: Stack(
        alignment: Alignment.topCenter,
        children: [
          Align(alignment: Alignment.topLeft, child: titleBlock),
          Align(alignment: Alignment.topCenter, child: periodSelector),
          Align(alignment: Alignment.topRight, child: shareAction),
        ],
      ),
    );
  }

  Widget _usageShareHeader({required String selectedPeriod}) {
    // The share image header is rendered only in the temporary capture overlay. The visible settings
    // page keeps its regular title/subtitle, while the exported image gets a more editorial header.
    return SizedBox(
      width: double.infinity,
      height: 72,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Keep the logo in the brand line instead of beside the whole title stack. The
                // previous lockup made the large title start after the icon, which looked visually
                // offset even though the widget edges were technically aligned.
                Row(
                  children: [
                    ClipRRect(borderRadius: BorderRadius.circular(8), child: Image(image: _woxIconImage, width: 30, height: 30, fit: BoxFit.cover)),
                    const SizedBox(width: 10),
                    Text('Wox Launcher', style: TextStyle(color: getThemeSubTextColor(), fontSize: 11, fontWeight: FontWeight.w800, letterSpacing: 1.1)),
                  ],
                ),
                const SizedBox(height: 9),
                Text(
                  _shareImageTitle(),
                  style: TextStyle(color: getThemeTextColor(), fontSize: 30, fontWeight: FontWeight.w900, height: 1.0),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(width: 20),
          // A small period pill gives the top-right corner useful context without competing with
          // the Wox logo. The previous card-like badge was visually heavier than the report title.
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 7),
            decoration: BoxDecoration(color: _panelColor().withValues(alpha: 0.72), borderRadius: BorderRadius.circular(999), border: Border.all(color: _outlineColor())),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.calendar_today_outlined, size: 13, color: getThemeSubTextColor()),
                const SizedBox(width: 7),
                Text(_periodLabel(selectedPeriod), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w800)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Future<ui.Image> _captureUsagePageImage() async {
    final overlay = Overlay.of(context);
    final captureKey = GlobalKey();
    final selectedPeriod = controller.usageStatsPeriod.value;
    final stats = controller.usageStats.value;
    final error = controller.usageStatsError.value;
    late final OverlayEntry entry;

    // The capture overlay is rendered and read back immediately. Precache the bundled icon first so
    // the share image does not grab a frame where the logo slot has layout but no decoded pixels yet.
    await precacheImage(_woxIconImage, context);

    entry = OverlayEntry(
      builder: (context) {
        return Positioned(
          left: -10000,
          top: -10000,
          child: IgnorePointer(
            child: Material(
              type: MaterialType.transparency,
              child: RepaintBoundary(
                key: captureKey,
                child: SizedBox(
                  width: GENERAL_SETTING_WIDE_FORM_WIDTH,
                  child: ColoredBox(
                    color: _shareImageBackgroundColor(),
                    // The exported image needs breathing room on every edge; the visible settings
                    // page keeps its dense layout, while only the capture overlay gets this padding.
                    child: Padding(
                      padding: const EdgeInsets.all(32),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          _usageShareHeader(selectedPeriod: selectedPeriod),
                          const SizedBox(height: 22),
                          _usageDashboardBody(stats: stats, selectedPeriod: selectedPeriod, error: error),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );

    overlay.insert(entry);
    try {
      // The capture target is hidden in an overlay so the screenshot can have a share-only title and
      // outer padding without mutating the visible settings page or capturing interactive controls.
      RenderRepaintBoundary? boundary;
      for (var attempt = 0; attempt < 6; attempt++) {
        WidgetsBinding.instance.ensureVisualUpdate();
        await Future<void>.delayed(const Duration(milliseconds: 16));
        await WidgetsBinding.instance.endOfFrame;
        boundary = captureKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
        if (boundary != null && !boundary.debugNeedsPaint) {
          break;
        }
      }

      final readyBoundary = boundary;
      if (readyBoundary == null || readyBoundary.debugNeedsPaint) {
        throw StateError('Usage page is not ready for sharing');
      }
      return await readyBoundary.toImage(pixelRatio: 2);
    } finally {
      entry.remove();
    }
  }

  String _buildXShareText() {
    // Keep the compose text in i18n so the X draft follows the user's Wox language. The image
    // itself is still copied separately because X intent URLs cannot attach clipboard images.
    return controller.tr('ui_usage_share_tweet_text');
  }

  Future<void> _shareUsageToX() async {
    final traceId = const UuidV4().generate();
    setState(() {
      _isSharingUsage = true;
      _shareStatusMessage = '';
      _shareStatusIsError = false;
    });

    try {
      // Exporting the live dashboard preserves the user's selected period and current theme. The
      // previous off-screen share card was visually disconnected from the page the user chose to
      // share, so the capture now targets the dashboard body directly.
      final image = await _captureUsagePageImage();
      final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
      image.dispose();
      if (byteData == null) {
        throw StateError('Failed to encode usage share image');
      }

      final directory = await getTemporaryDirectory();
      final file = File('${directory.path}/wox-usage-share.png');
      await file.writeAsBytes(byteData.buffer.asUint8List(), flush: true);

      var statusMessage = '';
      var statusIsError = false;
      try {
        await ScreenshotPlatformBridge.instance.writeClipboardImageFile(filePath: file.path);
      } catch (e) {
        // Clipboard support is platform-specific. The generated image file is still useful, so the
        // share flow continues to X and reports a warning instead of failing the whole action.
        Logger.instance.warn(traceId, 'Usage share image generated but clipboard copy failed: $e');
        statusMessage = controller.tr('ui_usage_share_clipboard_unsupported');
      }

      final text = Uri.encodeQueryComponent(_buildXShareText());
      final uri = Uri.parse('https://x.com/intent/tweet?text=$text');
      final opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!opened) {
        statusMessage = controller.tr('ui_usage_share_failed');
        statusIsError = true;
      }

      if (mounted) {
        setState(() {
          _shareStatusMessage = statusMessage;
          _shareStatusIsError = statusIsError;
        });
      }
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to share usage stats to X: $e');
      if (mounted) {
        setState(() {
          _shareStatusMessage = '${controller.tr('ui_usage_share_failed')}: $e';
          _shareStatusIsError = true;
        });
      }
    } finally {
      if (mounted) {
        setState(() {
          _isSharingUsage = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final isLoading = controller.isUsageStatsLoading.value;
      final error = controller.usageStatsError.value;
      final stats = controller.usageStats.value;
      final selectedPeriod = controller.usageStatsPeriod.value;

      return Stack(
        children: [
          _form(
            width: GENERAL_SETTING_WIDE_FORM_WIDTH,
            children: [
              if (_shareStatusMessage.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 10),
                  child: Text(_shareStatusMessage, style: TextStyle(color: _shareStatusIsError ? Colors.red : getThemeSubTextColor(), fontSize: 12)),
                ),
              // Normal settings view keeps its original title and subtitle. The share-only title
              // and outer padding are rendered in a temporary overlay when the user clicks Share.
              _usageSummaryHeader(isLoading: isLoading, disableShare: isLoading, selectedPeriod: selectedPeriod),
              const SizedBox(height: 18),
              _usageDashboardBody(stats: stats, selectedPeriod: selectedPeriod, error: error),
            ],
          ),
        ],
      );
    });
  }
}

class _UsageHeatmapDay {
  const _UsageHeatmapDay({required this.date, required this.count});

  final DateTime date;
  final int count;
}

class _HeatmapMonthLabel {
  const _HeatmapMonthLabel({required this.left, required this.text});

  final double left;
  final String text;
}

class _HeatmapThresholds {
  const _HeatmapThresholds({required this.low, required this.medium, required this.high});

  final int low;
  final int medium;
  final int high;
}

class _UsagePeriodOption {
  const _UsagePeriodOption({required this.code, required this.label});

  final String code;
  final String label;
}
