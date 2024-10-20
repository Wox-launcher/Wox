import 'package:wox/entity/wox_image.dart';

class ToolbarInfo {
  // left side of the toolbar
  final WoxImage? icon;
  final String? text;

  // right side of the toolbar
  final String? actionName;
  final String? hotkey;
  final Function()? action;

  ToolbarInfo({
    this.icon,
    this.text,
    this.action,
    this.actionName,
    this.hotkey,
  });

  static ToolbarInfo empty() {
    return ToolbarInfo(
      text: '',
    );
  }

  // text and hotkey are both empty
  bool isEmpty() {
    return (text == null || text!.isEmpty) && (hotkey == null || hotkey!.isEmpty);
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
