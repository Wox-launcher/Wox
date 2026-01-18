import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/windows/window_manager.dart';
import 'package:wox/utils/wox_setting_util.dart';

class WoxThemeUtil {
  final Rx<WoxTheme> _currentTheme = WoxTheme.empty().obs;

  WoxThemeUtil._privateConstructor();

  static final WoxThemeUtil _instance = WoxThemeUtil._privateConstructor();

  static WoxThemeUtil get instance => _instance;

  Future<void> loadTheme(String traceId) async {
    final theme = await WoxApi.instance.loadTheme(traceId);
    changeTheme(theme);
  }

  changeTheme(WoxTheme theme) {
    _currentTheme.value = theme;

    // Update window appearance based on theme background luminance
    // We check appBackgroundColorParsed (Color) luminance
    // 0.0 is black, 1.0 is white. < 0.5 considered dark.
    final isDark = theme.appBackgroundColorParsed.computeLuminance() < 0.5;
    WindowManager.instance.setAppearance(isDark ? 'dark' : 'light');
  }

  Rx<WoxTheme> get currentTheme => _currentTheme;

  double getQueryBoxHeight() {
    return QUERY_BOX_BASE_HEIGHT + currentTheme.value.appPaddingTop + currentTheme.value.appPaddingBottom;
  }

  double getResultItemHeight() {
    return RESULT_ITEM_BASE_HEIGHT + currentTheme.value.resultItemPaddingTop + currentTheme.value.resultItemPaddingBottom;
  }

  double getActionItemHeight() {
    return ACTION_ITEM_BASE_HEIGHT;
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
