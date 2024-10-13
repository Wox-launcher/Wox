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
