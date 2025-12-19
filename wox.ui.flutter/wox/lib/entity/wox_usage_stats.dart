class WoxUsageStats {
  late int totalOpened;
  late int totalAppLaunch;
  late int totalActions;
  late int totalAppsUsed;
  late int mostActiveHour;
  late int mostActiveDay;
  late List<int> openedByHour;
  late List<int> openedByWeekday;
  late List<WoxUsageStatsItem> topApps;
  late List<WoxUsageStatsItem> topPlugins;

  WoxUsageStats({
    required this.totalOpened,
    required this.totalAppLaunch,
    required this.totalActions,
    required this.totalAppsUsed,
    required this.mostActiveHour,
    required this.mostActiveDay,
    required this.openedByHour,
    required this.openedByWeekday,
    required this.topApps,
    required this.topPlugins,
  });

  WoxUsageStats.empty() {
    totalOpened = 0;
    totalAppLaunch = 0;
    totalActions = 0;
    totalAppsUsed = 0;
    mostActiveHour = -1;
    mostActiveDay = -1;
    openedByHour = List<int>.filled(24, 0);
    openedByWeekday = List<int>.filled(7, 0);
    topApps = <WoxUsageStatsItem>[];
    topPlugins = <WoxUsageStatsItem>[];
  }

  WoxUsageStats.fromJson(Map<String, dynamic> json) {
    totalOpened = json['TotalOpened'] ?? 0;
    totalAppLaunch = json['TotalAppLaunch'] ?? 0;
    totalActions = json['TotalActions'] ?? 0;
    totalAppsUsed = json['TotalAppsUsed'] ?? 0;
    mostActiveHour = json['MostActiveHour'] ?? -1;
    mostActiveDay = json['MostActiveDay'] ?? -1;

    openedByHour =
        (json['OpenedByHour'] as List?)?.map((e) => e as int).toList() ??
            List<int>.filled(24, 0);
    if (openedByHour.length != 24) {
      openedByHour = List<int>.filled(24, 0);
    }

    openedByWeekday =
        (json['OpenedByWeekday'] as List?)?.map((e) => e as int).toList() ??
            List<int>.filled(7, 0);
    if (openedByWeekday.length != 7) {
      openedByWeekday = List<int>.filled(7, 0);
    }

    if (json['TopApps'] != null) {
      topApps = <WoxUsageStatsItem>[];
      (json['TopApps'] as List).forEach((v) {
        topApps.add(WoxUsageStatsItem.fromJson(v));
      });
    } else {
      topApps = <WoxUsageStatsItem>[];
    }

    if (json['TopPlugins'] != null) {
      topPlugins = <WoxUsageStatsItem>[];
      (json['TopPlugins'] as List).forEach((v) {
        topPlugins.add(WoxUsageStatsItem.fromJson(v));
      });
    } else {
      topPlugins = <WoxUsageStatsItem>[];
    }
  }
}

class WoxUsageStatsItem {
  late String id;
  late String name;
  late int count;

  WoxUsageStatsItem({
    required this.id,
    required this.name,
    required this.count,
  });

  WoxUsageStatsItem.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? '';
    name = json['Name'] ?? '';
    count = json['Count'] ?? 0;
  }
}
