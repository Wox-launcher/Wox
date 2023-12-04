import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_theme.dart';

class WoxThemeUtil {
  late WoxTheme _currentTheme;

  WoxThemeUtil._privateConstructor();

  static final WoxThemeUtil _instance = WoxThemeUtil._privateConstructor();

  static WoxThemeUtil get instance => _instance;

  Future<void> loadTheme() async {
    _currentTheme = await WoxApi.instance.loadTheme();
  }

  WoxTheme get currentTheme => _currentTheme;

  double getWoxBoxContainerHeight() {
    return 55.0 + currentTheme.appPaddingTop + currentTheme.appPaddingBottom;
  }
}
