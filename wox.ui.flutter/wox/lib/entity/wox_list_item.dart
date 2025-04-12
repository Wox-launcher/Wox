import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';

class WoxListItem<T> {
  final String id;
  final WoxImage icon;
  final String title;
  final String subTitle;
  final List<WoxListItemTail> tails;
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
    List<WoxListItemTail>? tails,
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

  @override
  String toString() {
    return title;
  }
}

class WoxListItemTail {
  late String type;
  late String? text;
  late WoxImage? image;
  late HotkeyX? hotkey;

  WoxListItemTail({required this.type, this.text, this.image, this.hotkey});

  WoxListItemTail.fromJson(Map<String, dynamic> json) {
    type = json['Type'];
    if (json['Text'] != null) {
      text = json['Text'];
    } else {
      text = null;
    }

    if (json['Image'] != null) {
      image = WoxImage.fromJson(json['Image']);
    } else {
      image = null;
    }

    if (json['Hotkey'] != null) {
      hotkey = WoxHotkey.parseHotkeyFromString(json['Hotkey']);
    } else {
      hotkey = null;
    }
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Type'] = type;

    if (text != null) {
      data['Text'] = text;
    } else {
      data['Text'] = null;
    }

    if (image != null) {
      data['Image'] = image!.toJson();
    } else {
      data['Image'] = null;
    }

    if (hotkey != null) {
      data['Hotkey'] = hotkey!.toString();
    } else {
      data['Hotkey'] = null;
    }
    return data;
  }

  factory WoxListItemTail.text(String text) {
    return WoxListItemTail(type: WoxQueryResultTailTypeEnum.WOX_QUERY_RESULT_TAIL_TYPE_TEXT.code, text: text);
  }

  factory WoxListItemTail.hotkey(HotkeyX hotkey) {
    return WoxListItemTail(type: WoxQueryResultTailTypeEnum.WOX_QUERY_RESULT_TAIL_TYPE_HOTKEY.code, hotkey: hotkey);
  }

  factory WoxListItemTail.image(WoxImage image) {
    return WoxListItemTail(type: WoxQueryResultTailTypeEnum.WOX_QUERY_RESULT_TAIL_TYPE_IMAGE.code, image: image);
  }
}
