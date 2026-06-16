import 'package:wox/entity/wox_image.dart';

class WoxUsageStats {
  late int totalOpened;
  late int totalAppLaunch;
  late int totalActions;
  late int totalAppsUsed;
  // Real usage duration from the backend lets the share image show "days with Wox"
  // without estimating from local UI state or hardcoded sample data.
  late int usageDays;
  late String period;
  late int periodDays;
  late int periodOpened;
  late int previousPeriodOpened;
  late double? openedChangePercent;
  late int periodAppLaunch;
  late int previousPeriodAppLaunch;
  late double? appLaunchChangePercent;
  late int periodAppsUsed;
  late int previousPeriodAppsUsed;
  late double? appsUsedChangePercent;
  late int periodActions;
  late int previousPeriodActions;
  late double? actionsChangePercent;
  late int mostActiveHour;
  late int mostActiveDay;
  late List<int> openedByHour;
  late List<int> openedByWeekday;
  late List<WoxUsageStatsDay> openedByDay;
  late List<WoxUsageStatsItem> topApps;
  late List<WoxUsageStatsItem> topPlugins;

  WoxUsageStats({
    required this.totalOpened,
    required this.totalAppLaunch,
    required this.totalActions,
    required this.totalAppsUsed,
    required this.usageDays,
    required this.period,
    required this.periodDays,
    required this.periodOpened,
    required this.previousPeriodOpened,
    required this.openedChangePercent,
    required this.periodAppLaunch,
    required this.previousPeriodAppLaunch,
    required this.appLaunchChangePercent,
    required this.periodAppsUsed,
    required this.previousPeriodAppsUsed,
    required this.appsUsedChangePercent,
    required this.periodActions,
    required this.previousPeriodActions,
    required this.actionsChangePercent,
    required this.mostActiveHour,
    required this.mostActiveDay,
    required this.openedByHour,
    required this.openedByWeekday,
    required this.openedByDay,
    required this.topApps,
    required this.topPlugins,
  });

  WoxUsageStats.empty() {
    totalOpened = 0;
    totalAppLaunch = 0;
    totalActions = 0;
    totalAppsUsed = 0;
    usageDays = 0;
    period = '30d';
    periodDays = 30;
    periodOpened = 0;
    previousPeriodOpened = 0;
    openedChangePercent = 0;
    periodAppLaunch = 0;
    previousPeriodAppLaunch = 0;
    appLaunchChangePercent = 0;
    periodAppsUsed = 0;
    previousPeriodAppsUsed = 0;
    appsUsedChangePercent = 0;
    periodActions = 0;
    previousPeriodActions = 0;
    actionsChangePercent = 0;
    mostActiveHour = -1;
    mostActiveDay = -1;
    openedByHour = List<int>.filled(24, 0);
    openedByWeekday = List<int>.filled(7, 0);
    openedByDay = <WoxUsageStatsDay>[];
    topApps = <WoxUsageStatsItem>[];
    topPlugins = <WoxUsageStatsItem>[];
  }

  WoxUsageStats.fromJson(Map<String, dynamic> json) {
    totalOpened = json['TotalOpened'] ?? 0;
    totalAppLaunch = json['TotalAppLaunch'] ?? 0;
    totalActions = json['TotalActions'] ?? 0;
    totalAppsUsed = json['TotalAppsUsed'] ?? 0;
    usageDays = json['UsageDays'] ?? 0;
    period = json['Period'] ?? '30d';
    periodDays = json['PeriodDays'] ?? 30;
    // Period fields power the visible Usage dashboard. The all-time fields remain parsed for the
    // share card and older API responses, so a backend that has not yet sent period metrics still
    // renders a meaningful page instead of showing empty KPI cards.
    periodOpened = json['PeriodOpened'] ?? totalOpened;
    previousPeriodOpened = json['PreviousPeriodOpened'] ?? 0;
    openedChangePercent = _parseNullableDouble(json['OpenedChangePercent']);
    periodAppLaunch = json['PeriodAppLaunch'] ?? totalAppLaunch;
    previousPeriodAppLaunch = json['PreviousPeriodAppLaunch'] ?? 0;
    appLaunchChangePercent = _parseNullableDouble(json['AppLaunchChangePercent']);
    periodAppsUsed = json['PeriodAppsUsed'] ?? totalAppsUsed;
    previousPeriodAppsUsed = json['PreviousPeriodAppsUsed'] ?? 0;
    appsUsedChangePercent = _parseNullableDouble(json['AppsUsedChangePercent']);
    periodActions = json['PeriodActions'] ?? totalActions;
    previousPeriodActions = json['PreviousPeriodActions'] ?? 0;
    actionsChangePercent = _parseNullableDouble(json['ActionsChangePercent']);
    mostActiveHour = json['MostActiveHour'] ?? -1;
    mostActiveDay = json['MostActiveDay'] ?? -1;

    openedByHour = (json['OpenedByHour'] as List?)?.map((e) => e as int).toList() ?? List<int>.filled(24, 0);
    if (openedByHour.length != 24) {
      openedByHour = List<int>.filled(24, 0);
    }

    openedByWeekday = (json['OpenedByWeekday'] as List?)?.map((e) => e as int).toList() ?? List<int>.filled(7, 0);
    if (openedByWeekday.length != 7) {
      openedByWeekday = List<int>.filled(7, 0);
    }

    if (json['OpenedByDay'] != null) {
      openedByDay = <WoxUsageStatsDay>[];
      for (final v in json['OpenedByDay'] as List) {
        openedByDay.add(WoxUsageStatsDay.fromJson(v));
      }
    } else {
      openedByDay = <WoxUsageStatsDay>[];
    }

    if (json['TopApps'] != null) {
      topApps = <WoxUsageStatsItem>[];
      for (final v in json['TopApps'] as List) {
        topApps.add(WoxUsageStatsItem.fromJson(v));
      }
    } else {
      topApps = <WoxUsageStatsItem>[];
    }

    if (json['TopPlugins'] != null) {
      topPlugins = <WoxUsageStatsItem>[];
      for (final v in json['TopPlugins'] as List) {
        topPlugins.add(WoxUsageStatsItem.fromJson(v));
      }
    } else {
      topPlugins = <WoxUsageStatsItem>[];
    }
  }

  static double? _parseNullableDouble(dynamic value) {
    if (value == null) {
      return null;
    }
    if (value is int) {
      return value.toDouble();
    }
    if (value is double) {
      return value;
    }
    return double.tryParse(value.toString());
  }
}

class WoxUsageStatsDay {
  late String date;
  late int count;

  WoxUsageStatsDay({required this.date, required this.count});

  WoxUsageStatsDay.fromJson(Map<String, dynamic> json) {
    date = json['Date'] ?? '';
    count = json['Count'] ?? 0;
  }
}

class WoxUsageStatsItem {
  late String id;
  late String name;
  late int count;
  late WoxImage icon;

  WoxUsageStatsItem({required this.id, required this.name, required this.count, WoxImage? icon}) {
    this.icon = icon ?? WoxImage.empty();
  }

  WoxUsageStatsItem.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? '';
    name = json['Name'] ?? '';
    count = json['Count'] ?? 0;
    icon = json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : WoxImage.empty();
  }
}
