import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_setting_util.dart';

class WoxThemeUtil {
  final Rx<WoxTheme> _currentTheme = WoxTheme.empty().obs;

  WoxThemeUtil._privateConstructor();

  static final WoxThemeUtil _instance = WoxThemeUtil._privateConstructor();

  static WoxThemeUtil get instance => _instance;

  Future<void> loadTheme() async {
    final theme = await WoxApi.instance.loadTheme();
    changeTheme(theme);
  }

  changeTheme(WoxTheme theme) {
    _currentTheme.value = theme;
  }

  Rx<WoxTheme> get currentTheme => _currentTheme;

  double getQueryBoxHeight() {
    return QUERY_BOX_BASE_HEIGHT + currentTheme.value.appPaddingTop + currentTheme.value.appPaddingBottom;
  }

  double getResultItemHeight() {
    return RESULT_ITEM_BASE_HEIGHT + currentTheme.value.resultItemPaddingTop + currentTheme.value.resultItemPaddingBottom;
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

  int getMaxResultCount() {
    var maxResultCount = WoxSettingUtil.instance.currentSetting.maxResultCount;
    if (maxResultCount == 0) {
      maxResultCount = MAX_LIST_VIEW_ITEM_COUNT;
    }
    return maxResultCount;
  }

  double getMaxResultListViewHeight() {
    return getResultListViewHeightByCount(getMaxResultCount());
  }

  double getMaxResultContainerHeight() {
    return getMaxResultListViewHeight() + currentTheme.value.resultContainerPaddingTop + currentTheme.value.resultContainerPaddingBottom;
  }
}
