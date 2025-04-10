import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';

class WoxListItem {
  final String id;
  final WoxImage icon;
  final String title;
  final String subTitle;
  final List<WoxQueryResultTail> tails;
  final bool isGroup;
  final String? hotkey;

  WoxListItem({
    required this.id,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.tails,
    required this.isGroup,
    this.hotkey,
  });
}
