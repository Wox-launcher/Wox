import 'package:wox/entity/wox_theme.dart';

class EntityFactory {
  static T generateOBJ<T>(Map<String, dynamic> json) {
    if (T.toString() == "WoxTheme") {
      return WoxTheme.fromJson(json) as T;
    } else {
      return json as T;
    }
  }
}
