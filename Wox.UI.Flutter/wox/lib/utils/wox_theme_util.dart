import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/consts.dart';

class WoxThemeUtil {
  late WoxTheme _currentTheme;

  WoxThemeUtil._privateConstructor();

  static final WoxThemeUtil _instance = WoxThemeUtil._privateConstructor();

  static WoxThemeUtil get instance => _instance;

  Future<void> loadTheme() async {
    _currentTheme = await WoxApi.instance.loadTheme();
  }

  WoxTheme get currentTheme => _currentTheme;

  double getWoxQueryBoxHeight() {
    return QUERY_BOX_BASE_HEIGHT + currentTheme.appPaddingTop + currentTheme.appPaddingBottom;
  }

  double getWoxResultItemHeight() {
    return RESULT_ITEM_BASE_HEIGHT + currentTheme.resultItemPaddingTop + currentTheme.resultItemPaddingBottom;
  }

  double getResultHeightByCount(int count) {
    if (count == 0) {
      return 0;
    }
    return getWoxResultItemHeight() * count;
  }

  double getResultListViewMaxHeight() {
    return getResultHeightByCount(MAX_LIST_VIEW_ITEM_COUNT) + currentTheme.resultContainerPaddingTop + currentTheme.resultContainerPaddingBottom;
  }
}
