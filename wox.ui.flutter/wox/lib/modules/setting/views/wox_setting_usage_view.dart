import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/entity/wox_usage_stats.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxSettingUsageView extends WoxSettingBaseView {
  const WoxSettingUsageView({super.key});

  String _weekdayLabel(int weekday) {
    switch (weekday) {
      case 0:
        return controller.tr('ui_weekday_sun');
      case 1:
        return controller.tr('ui_weekday_mon');
      case 2:
        return controller.tr('ui_weekday_tue');
      case 3:
        return controller.tr('ui_weekday_wed');
      case 4:
        return controller.tr('ui_weekday_thu');
      case 5:
        return controller.tr('ui_weekday_fri');
      case 6:
        return controller.tr('ui_weekday_sat');
      default:
        return weekday.toString();
    }
  }

  Widget _statCard(
      {required String title, required String value, required IconData icon}) {
    final bg = safeFromCssColor(WoxThemeUtil
            .instance.currentTheme.value.actionItemActiveBackgroundColor)
        .withOpacity(0.4);
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: getThemeTextColor().withOpacity(0.08)),
      ),
      child: Row(
        children: [
          Container(
            width: 34,
            height: 34,
            decoration: BoxDecoration(
              color: getThemeTextColor().withOpacity(0.08),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(icon, size: 18, color: getThemeTextColor()),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title,
                    style:
                        TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                Text(value,
                    style: TextStyle(
                        color: getThemeTextColor(),
                        fontSize: 20,
                        fontWeight: FontWeight.w600)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _barChart(
      {required List<int> data,
      required List<String> labels,
      double height = 120}) {
    final maxValue = data.isEmpty ? 0 : data.reduce((a, b) => a > b ? a : b);
    final barColor = safeFromCssColor(
        WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor);
    final bgLine = getThemeTextColor().withOpacity(0.06);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: getThemeTextColor().withOpacity(0.08)),
        color: getThemeTextColor().withOpacity(0.03),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            height: height,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: List.generate(data.length, (i) {
                final v = data[i];
                final ratio = maxValue == 0 ? 0.0 : v / maxValue;
                return Expanded(
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 2),
                    child: Container(
                      height: (height - 12) * ratio + 4,
                      decoration: BoxDecoration(
                        color: barColor.withOpacity(0.65),
                        borderRadius: BorderRadius.circular(6),
                        border: Border.all(color: bgLine),
                      ),
                    ),
                  ),
                );
              }),
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: List.generate(labels.length, (i) {
              return Expanded(
                child: Text(
                  labels[i],
                  textAlign: TextAlign.center,
                  style: TextStyle(color: getThemeSubTextColor(), fontSize: 11),
                  overflow: TextOverflow.ellipsis,
                ),
              );
            }),
          ),
        ],
      ),
    );
  }

  Widget _topList(
      {required String title,
      required List<WoxUsageStatsItem> items,
      required String emptyText}) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: getThemeTextColor().withOpacity(0.08)),
        color: getThemeTextColor().withOpacity(0.03),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title,
              style: TextStyle(
                  color: getThemeTextColor(),
                  fontSize: 13,
                  fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          if (items.isEmpty)
            Text(emptyText,
                style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))
          else
            Column(
              children: items.take(10).map((e) {
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 6),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          e.name.isNotEmpty ? e.name : e.id,
                          style: TextStyle(
                              color: getThemeTextColor(), fontSize: 13),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Text(
                        e.count.toString(),
                        style: TextStyle(
                            color: getThemeSubTextColor(), fontSize: 12),
                      ),
                    ],
                  ),
                );
              }).toList(),
            ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final isLoading = controller.isUsageStatsLoading.value;
      final error = controller.usageStatsError.value;
      final stats = controller.usageStats.value;

      final mostHour = stats.mostActiveHour < 0
          ? '-'
          : '${stats.mostActiveHour.toString().padLeft(2, '0')}:00';
      final mostDay =
          stats.mostActiveDay < 0 ? '-' : _weekdayLabel(stats.mostActiveDay);

      return form(children: [
        Row(
          children: [
            Text(
              controller.tr('ui_usage'),
              style: TextStyle(
                  color: getThemeTextColor(),
                  fontSize: 16,
                  fontWeight: FontWeight.w600),
            ),
            const SizedBox(width: 12),
            if (isLoading)
              const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2))
            else
              WoxButton.text(
                text: controller.tr('ui_refresh'),
                onPressed: () => controller.refreshUsageStats(),
              ),
          ],
        ),
        const SizedBox(height: 14),
        if (error.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: Text(error,
                style: const TextStyle(color: Colors.red, fontSize: 12)),
          ),
        LayoutBuilder(
          builder: (context, constraints) {
            final width =
                constraints.maxWidth.isFinite ? constraints.maxWidth : 960.0;
            final spacing = 12.0;
            final columns = width >= 760 ? 4 : 2;
            final cardWidth = (width - (columns - 1) * spacing) / columns;

            return Wrap(
              spacing: spacing,
              runSpacing: spacing,
              children: [
                SizedBox(
                  width: cardWidth,
                  child: _statCard(
                    title: controller.tr('ui_usage_opened'),
                    value: stats.totalOpened.toString(),
                    icon: Icons.visibility_outlined,
                  ),
                ),
                SizedBox(
                  width: cardWidth,
                  child: _statCard(
                    title: controller.tr('ui_usage_app_launches'),
                    value: stats.totalAppLaunch.toString(),
                    icon: Icons.rocket_launch_outlined,
                  ),
                ),
                SizedBox(
                  width: cardWidth,
                  child: _statCard(
                    title: controller.tr('ui_usage_apps_used'),
                    value: stats.totalAppsUsed.toString(),
                    icon: Icons.apps_outlined,
                  ),
                ),
                SizedBox(
                  width: cardWidth,
                  child: _statCard(
                    title: controller.tr('ui_usage_actions'),
                    value: stats.totalActions.toString(),
                    icon: Icons.bolt_outlined,
                  ),
                ),
              ],
            );
          },
        ),
        const SizedBox(height: 18),
        LayoutBuilder(
          builder: (context, constraints) {
            final width =
                constraints.maxWidth.isFinite ? constraints.maxWidth : 960.0;
            final spacing = 12.0;
            final columns = width >= 760 ? 2 : 1;
            final blockWidth =
                columns == 1 ? width : (width - spacing) / columns;

            final hourLabels = List<String>.generate(
                24, (i) => i % 6 == 0 ? i.toString().padLeft(2, '0') : '');
            final weekdayLabels =
                List<String>.generate(7, (i) => _weekdayLabel(i));

            return Wrap(
              spacing: spacing,
              runSpacing: spacing,
              children: [
                SizedBox(
                  width: blockWidth,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(controller.tr('ui_usage_opened_by_hour'),
                          style: TextStyle(
                              color: getThemeTextColor(),
                              fontSize: 13,
                              fontWeight: FontWeight.w600)),
                      const SizedBox(height: 8),
                      _barChart(data: stats.openedByHour, labels: hourLabels),
                      const SizedBox(height: 8),
                      Text(
                          '${controller.tr('ui_usage_most_active_hour')}: $mostHour',
                          style: TextStyle(
                              color: getThemeSubTextColor(), fontSize: 12)),
                    ],
                  ),
                ),
                SizedBox(
                  width: blockWidth,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(controller.tr('ui_usage_opened_by_weekday'),
                          style: TextStyle(
                              color: getThemeTextColor(),
                              fontSize: 13,
                              fontWeight: FontWeight.w600)),
                      const SizedBox(height: 8),
                      _barChart(
                          data: stats.openedByWeekday, labels: weekdayLabels),
                      const SizedBox(height: 8),
                      Text(
                          '${controller.tr('ui_usage_most_active_day')}: $mostDay',
                          style: TextStyle(
                              color: getThemeSubTextColor(), fontSize: 12)),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
        const SizedBox(height: 18),
        LayoutBuilder(
          builder: (context, constraints) {
            final width =
                constraints.maxWidth.isFinite ? constraints.maxWidth : 960.0;
            final spacing = 12.0;
            final columns = width >= 760 ? 2 : 1;
            final blockWidth =
                columns == 1 ? width : (width - spacing) / columns;
            final emptyText = controller.tr('ui_usage_no_data');

            return Wrap(
              spacing: spacing,
              runSpacing: spacing,
              children: [
                SizedBox(
                  width: blockWidth,
                  child: _topList(
                      title: controller.tr('ui_usage_top_apps'),
                      items: stats.topApps,
                      emptyText: emptyText),
                ),
                SizedBox(
                  width: blockWidth,
                  child: _topList(
                      title: controller.tr('ui_usage_top_plugins'),
                      items: stats.topPlugins,
                      emptyText: emptyText),
                ),
              ],
            );
          },
        ),
      ]);
    });
  }
}
