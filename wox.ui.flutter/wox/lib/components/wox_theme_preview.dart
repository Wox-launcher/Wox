import 'package:flutter/material.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxThemePreview extends StatelessWidget {
  final WoxTheme theme;

  const WoxThemePreview({super.key, required this.theme});

  @override
  Widget build(BuildContext context) {
    Color backgroundColor = safeFromCssColor(theme.appBackgroundColor);
    Color queryBoxColor = safeFromCssColor(theme.queryBoxBackgroundColor);
    Color resultItemActiveColor = safeFromCssColor(theme.resultItemActiveBackgroundColor);
    Color resultItemColor = safeFromCssColor(theme.appBackgroundColor);

    final List<String> previewTexts = [
      "Search for applications, folders, files and more",
      "Plenty of Plugins and AI Themes",
      "Single executable file, no installation required",
      "Develop plugins with Javascript, Python, C#",
      "Customizable and extensible launcher",
    ];

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 20, 20, 200),
      child: Container(
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(8),
          color: backgroundColor,
        ),
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
              child: Container(
                height: 40,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(theme.queryBoxBorderRadius.toDouble()),
                  color: queryBoxColor,
                ),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Padding(
                    padding: const EdgeInsets.only(left: 10),
                    child: Text(
                      "Wox Theme Preview",
                      style: TextStyle(color: safeFromCssColor(theme.queryBoxFontColor)),
                    ),
                  ),
                ),
              ),
            ),
            Expanded(
              child: ListView(
                children: List.generate(5, (index) {
                  bool isActive = index == 1;
                  return Container(
                    height: 60,
                    margin: const EdgeInsets.symmetric(vertical: 0, horizontal: 10),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(theme.resultItemBorderRadius.toDouble()),
                      color: isActive ? resultItemActiveColor : resultItemColor,
                    ),
                    child: ListTile(
                      leading: WoxImageView(
                        woxImage: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_FILE),
                        width: 30,
                      ),
                      title: Text(
                        previewTexts[index],
                        style: TextStyle(
                          color: safeFromCssColor(isActive ? theme.resultItemActiveTitleColor : theme.resultItemTitleColor),
                        ),
                      ),
                      subtitle: Text(
                        "Wox Feature ${index + 1}",
                        style: TextStyle(
                          color: safeFromCssColor(isActive ? theme.resultItemActiveSubTitleColor : theme.resultItemSubTitleColor),
                        ),
                      ),
                    ),
                  );
                }),
              ),
            ),
            Container(
              height: WoxThemeUtil.instance.getToolbarHeight(),
              decoration: BoxDecoration(
                color: safeFromCssColor(theme.toolbarBackgroundColor.isEmpty ? theme.appBackgroundColor : theme.toolbarBackgroundColor),
                border: Border(
                  top: BorderSide(
                    color: safeFromCssColor(theme.toolbarFontColor).withOpacity(0.1),
                    width: 1,
                  ),
                ),
              ),
              child: Padding(
                padding: EdgeInsets.symmetric(
                  horizontal: theme.toolbarPaddingLeft.toDouble(),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    const Row(
                      children: [],
                    ),
                    Row(
                      children: [
                        Text(
                          "Open",
                          style: TextStyle(color: safeFromCssColor(theme.toolbarFontColor)),
                        ),
                        const SizedBox(width: 8),
                        WoxHotkeyView(
                          hotkey: WoxHotkey.parseHotkeyFromString("Enter")!,
                          backgroundColor: safeFromCssColor(theme.toolbarBackgroundColor),
                          borderColor: safeFromCssColor(theme.toolbarFontColor),
                          textColor: safeFromCssColor(theme.toolbarFontColor),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
