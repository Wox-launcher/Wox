import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';

class WoxListItem<T> {
  final String id;
  final WoxImage icon;
  final String title;
  final String subTitle;
  final List<WoxQueryResultTail> tails;
  final bool isGroup;
  final String? hotkey;
  final T data; // extra data associated with the item

  WoxListItem({
    required this.id,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.tails,
    required this.isGroup,
    this.hotkey,
    required this.data,
  });

  WoxListItem<T> copyWith({
    String? id,
    WoxImage? icon,
    String? title,
    String? subTitle,
    List<WoxQueryResultTail>? tails,
    bool? isGroup,
    String? hotkey,
    T? data,
  }) {
    return WoxListItem<T>(
      id: id ?? this.id,
      icon: icon ?? this.icon,
      title: title ?? this.title,
      subTitle: subTitle ?? this.subTitle,
      tails: tails ?? this.tails,
      isGroup: isGroup ?? this.isGroup,
      hotkey: hotkey ?? this.hotkey,
      data: data ?? this.data,
    );
  }

  static WoxListItem<WoxQueryResult> fromQueryResult(WoxQueryResult result) {
    return WoxListItem<WoxQueryResult>(
      id: result.id,
      icon: result.icon,
      title: result.title,
      subTitle: result.subTitle,
      tails: result.tails,
      isGroup: result.isGroup,
      data: result,
    );
  }

  static WoxListItem<WoxResultAction> fromResultAction(WoxResultAction action) {
    return WoxListItem<WoxResultAction>(
      id: action.id,
      icon: action.icon,
      title: action.name,
      subTitle: "",
      tails: [],
      isGroup: false,
      hotkey: action.hotkey,
      data: action,
    );
  }
}
