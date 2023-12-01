import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_theme.dart';

class WoxThemeUtil {
  late WoxTheme _currentTheme;

  WoxThemeUtil._privateConstructor();

  static final WoxThemeUtil _instance = WoxThemeUtil._privateConstructor();

  static WoxThemeUtil get instance => _instance;

  Future<void> loadTheme() async {
    WoxApi.instance.loadTheme().then((value) {
      _currentTheme = value;
    });
  }

  WoxTheme get currentTheme => _currentTheme;
}
