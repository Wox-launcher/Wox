import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
import 'package:wox/utils/wox_setting_util.dart';

void main() {
  setUpAll(() async {
    WoxSettingUtil.instance.setCurrentSettingForTesting(_setting(appWidth: 1200));
  });

  testWidgets('result tails stay attached to the trailing edge after title text expands', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1200, 240));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final itemKey = GlobalKey();

    await tester.pumpWidget(
      MaterialApp(
        home: Material(
          child: SizedBox(
            width: 1000,
            height: 56,
            child: WoxListItemView(
              key: itemKey,
              item: WoxListItem<void>(
                id: 'tail-layout',
                icon: WoxImage.empty(),
                title: 'Wox.Plugin.Template.Nodejs',
                subTitle: r'C:\dev\Wox.Plugin.Template.Nodejs',
                tails: [
                  WoxListItemTail.text('4 天前'),
                  WoxListItemTail.text('B2'),
                  WoxListItemTail.text('4ms'),
                  WoxListItemTail.text('27ms', textCategory: woxListItemTailTextCategoryDanger),
                ],
                isGroup: true,
                data: null,
              ),
              woxTheme: WoxTheme(resultItemTitleColor: '#ffffff', resultItemSubTitleColor: '#cccccc', resultItemTailTextColor: '#cccccc'),
              isActive: false,
              isHovered: false,
              listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code,
            ),
          ),
        ),
      ),
    );

    final tailScrollFinder = find.descendant(of: find.byKey(itemKey), matching: find.byType(SingleChildScrollView)).first;
    final itemRight = tester.getTopRight(find.byKey(itemKey)).dx;
    final tailRight = tester.getTopRight(tailScrollFinder).dx;

    expect(itemRight - tailRight, lessThanOrEqualTo(5));
  });
}

WoxSetting _setting({required int appWidth}) {
  return WoxSetting(
    enableAutostart: false,
    mainHotkey: '',
    selectionHotkey: '',
    ignoredHotkeyApps: [],
    logLevel: 'INFO',
    usePinYin: false,
    switchInputMethodABC: false,
    hideOnStart: false,
    onboardingFinished: true,
    hideOnLostFocus: false,
    showTray: false,
    langCode: 'en_US',
    releaseChannel: 'stable',
    queryHotkeys: [],
    queryShortcuts: [],
    trayQueries: [],
    launchMode: 'continue',
    startPage: 'mru',
    showPosition: 'mouse_screen',
    aiProviders: [],
    appWidth: appWidth,
    maxResultCount: 8,
    uiDensity: 'normal',
    themeId: '',
    appFontFamily: '',
    enableQueryCompletionHint: false,
    enableGlance: false,
    primaryGlance: GlanceRef.empty(),
    hideGlanceIcon: false,
    httpProxyEnabled: false,
    httpProxyUrl: '',
    enableAutoBackup: false,
    enableAutoUpdate: false,
    enableAnonymousUsageStats: false,
    customPythonPath: '',
    customNodejsPath: '',
    showScoreTail: false,
    showPerformanceTail: false,
    showPerformanceTailBatch: true,
    showPerformanceTailPluginQuery: true,
    showPerformanceTailBackendPrepared: true,
    showPerformanceTailUiReceived: true,
  );
}
