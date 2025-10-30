import 'package:wox/entity/wox_image.dart';

class ToolbarActionInfo {
  final String name;
  final String hotkey;
  final Function()? action; // Optional action callback for cases without result (e.g., doctor check)

  ToolbarActionInfo({
    required this.name,
    required this.hotkey,
    this.action,
  });
}

class ToolbarInfo {
  // left side of the toolbar
  final WoxImage? icon;
  final String? text;

  // right side of the toolbar
  final List<ToolbarActionInfo>? actions; // All actions with hotkeys

  ToolbarInfo({
    this.icon,
    this.text,
    this.actions,
  });

  static ToolbarInfo empty() {
    return ToolbarInfo(
      text: '',
    );
  }

  ToolbarInfo copyWith({
    WoxImage? icon,
    String? text,
    List<ToolbarActionInfo>? actions,
  }) {
    return ToolbarInfo(
      icon: icon ?? this.icon,
      text: text ?? this.text,
      actions: actions ?? this.actions,
    );
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
  final WoxImage? icon;
  final String? text;
  final int displaySeconds; // how long to display the message, 0 for forever

  ToolbarMsg({
    this.icon,
    this.text,
    this.displaySeconds = 10,
  });

  static ToolbarMsg fromJson(Map<String, dynamic> json) {
    return ToolbarMsg(
      icon: WoxImage.parse(json['Icon']),
      text: json['Text'] ?? '',
      displaySeconds: json['DisplaySeconds'] ?? 10,
    );
  }
}
