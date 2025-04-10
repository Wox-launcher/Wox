import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';

class WoxListItem {
  final String id;
  final WoxImage icon;
  final String title;
  final String subTitle;
  final List<WoxQueryResultTail> tails;
  final WoxTheme woxTheme;
  final bool isGroup;

  WoxListItem({
    required this.id,
    required this.woxTheme,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.tails,
    required this.isGroup,
  });
}
