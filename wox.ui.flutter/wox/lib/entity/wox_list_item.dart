import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
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
  final bool isShowQuickSelect;
  final String quickSelectNumber;

  WoxListItem({
    required this.id,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.tails,
    required this.isGroup,
    this.hotkey,
    required this.data,
    this.isShowQuickSelect = false,
    this.quickSelectNumber = '',
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
    bool? isShowQuickSelect,
    String? quickSelectNumber,
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
      isShowQuickSelect: isShowQuickSelect ?? this.isShowQuickSelect,
      quickSelectNumber: quickSelectNumber ?? this.quickSelectNumber,
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
    // Create hotkey tail if action has hotkey
    List<WoxListItemTail> tails = [];
    if (action.hotkey.isNotEmpty) {
      var hotkey = WoxHotkey.parseHotkeyFromString(action.hotkey);
      if (hotkey != null) {
        tails.add(WoxListItemTail.hotkey(hotkey));
      }
    }

    return WoxListItem<WoxResultAction>(id: action.id, icon: action.icon, title: action.name, subTitle: "", tails: tails, isGroup: false, hotkey: action.hotkey, data: action);
  }

  @override
  String toString() {
    return title;
  }
}

class WoxListItemTail {
  late String type; // see @WoxListItemTailTypeEnum
  late String? text;
  late String textCategory;
  late WoxImage? image;
  late HotkeyX? hotkey;
  late double? imageWidth;
  late double? imageHeight;
  late String? tooltip;

  WoxListItemTail({
    required this.type,
    this.text,
    this.textCategory = woxListItemTailTextCategoryDefault,
    this.image,
    this.hotkey,
    this.imageWidth,
    this.imageHeight,
    this.tooltip,
  });

  WoxListItemTail.fromJson(Map<String, dynamic> json) {
    type = json['Type'];
    if (json['Text'] != null) {
      text = json['Text'];
    } else {
      text = null;
    }
    textCategory = WoxListItemTailTextCategoryEnum.ensureCode(json['TextCategory']);

    if (json['Image'] != null) {
      image = WoxImage.fromJson(json['Image']);
    } else {
      image = null;
    }

    imageWidth = (json['ImageWidth'] as num?)?.toDouble();
    imageHeight = (json['ImageHeight'] as num?)?.toDouble();
    tooltip = json['Tooltip'];

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
    data['TextCategory'] = type == WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code ? textCategory : null;

    if (image != null) {
      data['Image'] = image!.toJson();
    } else {
      data['Image'] = null;
    }

    data['ImageWidth'] = imageWidth;
    data['ImageHeight'] = imageHeight;
    data['Tooltip'] = tooltip;

    if (hotkey != null) {
      data['Hotkey'] = hotkey!.toString();
    } else {
      data['Hotkey'] = null;
    }
    return data;
  }

  factory WoxListItemTail.text(String text, {String textCategory = woxListItemTailTextCategoryDefault}) {
    return WoxListItemTail(type: WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code, text: text, textCategory: textCategory);
  }

  factory WoxListItemTail.hotkey(HotkeyX hotkey) {
    return WoxListItemTail(type: WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_HOTKEY.code, hotkey: hotkey);
  }

  factory WoxListItemTail.image(WoxImage image, {double? width, double? height}) {
    return WoxListItemTail(type: WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_IMAGE.code, image: image, imageWidth: width, imageHeight: height);
  }
}
