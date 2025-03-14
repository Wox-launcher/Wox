import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/consts.dart';

class WoxThemeUtil {
  late WoxTheme _currentTheme;

  WoxThemeUtil._privateConstructor();

  static final WoxThemeUtil _instance = WoxThemeUtil._privateConstructor();

  static WoxThemeUtil get instance => _instance;

  Future<void> loadTheme() async {
    final theme = await WoxApi.instance.loadTheme();
    changeTheme(theme);
  }

  changeTheme(WoxTheme theme) {
    _currentTheme = theme;
  }

  WoxTheme get currentTheme => _currentTheme;

  double getQueryBoxHeight() {
    return QUERY_BOX_BASE_HEIGHT + currentTheme.appPaddingTop + currentTheme.appPaddingBottom;
  }

  double getResultItemHeight() {
    return RESULT_ITEM_BASE_HEIGHT + currentTheme.resultItemPaddingTop + currentTheme.resultItemPaddingBottom;
  }

  double getToolbarHeight() {
    return TOOLBAR_HEIGHT;
  }

  double getResultListViewHeightByCount(int count) {
    if (count == 0) {
      return 0;
    }
    return getResultItemHeight() * count;
  }

  double getResultContainerMaxHeight() {
    return getResultListViewHeightByCount(MAX_LIST_VIEW_ITEM_COUNT) + currentTheme.resultContainerPaddingTop + currentTheme.resultContainerPaddingBottom;
  }
}
