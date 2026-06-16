import 'package:wox/entity/wox_image.dart';

WoxImage? _parseToolbarImage(dynamic value) {
  if (value is Map<String, dynamic>) {
    return WoxImage.fromJson(value);
  }
  if (value is Map) {
    return WoxImage.fromJson(value.map((key, item) => MapEntry(key.toString(), item)));
  }
  if (value is String) {
    return WoxImage.parse(value);
  }
  return null;
}

class ToolbarActionInfo {
  final String name;
  final String hotkey;
  final Function()? action; // Optional action callback for cases without result (e.g., doctor check)

  ToolbarActionInfo({required this.name, required this.hotkey, this.action});
}

class ToolbarMsgActionInfo {
  final String id;
  final String name;
  final WoxImage? icon;
  final String hotkey;
  final bool isDefault;
  final bool preventHideAfterAction;
  final Map<String, String> contextData;

  ToolbarMsgActionInfo({
    required this.id,
    required this.name,
    required this.icon,
    required this.hotkey,
    required this.isDefault,
    required this.preventHideAfterAction,
    required this.contextData,
  });

  factory ToolbarMsgActionInfo.fromJson(Map<String, dynamic> json) {
    final rawContextData = json['ContextData'];
    final contextData = rawContextData is Map ? rawContextData.map((key, value) => MapEntry(key.toString(), value.toString())) : <String, String>{};

    return ToolbarMsgActionInfo(
      id: json['Id'] ?? "",
      name: json['Name'] ?? "",
      icon: _parseToolbarImage(json['Icon']),
      hotkey: json['Hotkey'] ?? "",
      isDefault: json['IsDefault'] == true,
      preventHideAfterAction: json['PreventHideAfterAction'] == true,
      contextData: contextData,
    );
  }
}

class ToolbarInfo {
  // left side of the toolbar
  final WoxImage? icon;
  final String? text;

  // right side of the toolbar
  final List<ToolbarActionInfo>? actions; // All actions with hotkeys

  ToolbarInfo({this.icon, this.text, this.actions});

  static ToolbarInfo empty() {
    return ToolbarInfo(text: '');
  }

  ToolbarInfo copyWith({WoxImage? icon, String? text, List<ToolbarActionInfo>? actions}) {
    return ToolbarInfo(icon: icon ?? this.icon, text: text ?? this.text, actions: actions ?? this.actions);
  }

  ToolbarInfo emptyRightSide() {
    return ToolbarInfo(icon: icon, text: text, actions: null);
  }

  ToolbarInfo emptyLeftSide() {
    return ToolbarInfo(icon: null, text: null, actions: actions);
  }

  // text and actions are both empty
  bool isEmpty() {
    return (text == null || text!.isEmpty) && (actions == null || actions!.isEmpty);
  }

  bool isNotEmpty() {
    return !isEmpty();
  }
}

class ToolbarMsg {
  final String id;
  final String title;
  final WoxImage? icon;
  final String? text;
  final int? progress;
  final bool indeterminate;
  final List<ToolbarMsgActionInfo> actions;
  final int displaySeconds; // how long to display the message, 0 for forever

  const ToolbarMsg({this.id = "", this.title = "", this.icon, this.text, this.progress, this.indeterminate = false, this.actions = const [], this.displaySeconds = 10});

  factory ToolbarMsg.empty() {
    return const ToolbarMsg();
  }

  static ToolbarMsg fromJson(Map<String, dynamic> json) {
    return ToolbarMsg(
      id: json['Id'] ?? "",
      title: json['Title'] ?? "",
      icon: _parseToolbarImage(json['Icon']),
      text: json['Text'] ?? '',
      progress: json['Progress'] is int ? json['Progress'] : null,
      indeterminate: json['Indeterminate'] == true,
      actions: (json['Actions'] as List<dynamic>? ?? []).map((item) => ToolbarMsgActionInfo.fromJson(item)).toList(),
      displaySeconds: json['DisplaySeconds'] ?? 10,
    );
  }

  bool get isPersistent => id.isNotEmpty || title.isNotEmpty || progress != null || indeterminate || actions.isNotEmpty;

  bool get isEmpty => !isPersistent && (text == null || text!.isEmpty);

  String get displayText => isPersistent ? title : (text ?? '');
}
