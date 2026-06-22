import 'package:flutter/material.dart';
import 'package:wox/utils/color_util.dart';

class WoxTheme {
  late String themeId;
  late String themeName;
  late String themeAuthor;
  late String themeUrl;
  late String version;
  late String description;
  late bool isSystem;
  late bool isInstalled;
  late bool isUpgradable;
  late bool isAutoAppearance;
  late String darkThemeId;
  late String lightThemeId;

  late String appBackgroundColor;
  late int appPaddingLeft;
  late int appPaddingTop;
  late int appPaddingRight;
  late int appPaddingBottom;
  late int resultContainerPaddingLeft;
  late int resultContainerPaddingTop;
  late int resultContainerPaddingRight;
  late int resultContainerPaddingBottom;
  late int resultItemBorderRadius;
  late int resultItemPaddingLeft;
  late int resultItemPaddingTop;
  late int resultItemPaddingRight;
  late int resultItemPaddingBottom;
  late String resultItemTitleColor;
  late String resultItemSubTitleColor;
  late String resultItemTailTextColor;
  late int resultItemBorderLeftWidth;
  late String resultItemActiveBackgroundColor;
  late String resultItemActiveTitleColor;
  late String resultItemActiveSubTitleColor;
  late int resultItemActiveBorderLeftWidth;
  late String resultItemActiveTailTextColor;
  late String queryBoxFontColor;
  late String queryBoxBackgroundColor;
  late int queryBoxBorderRadius;
  late String queryBoxCursorColor;
  late String queryBoxTextSelectionBackgroundColor;
  late String queryBoxTextSelectionColor;
  late String actionContainerBackgroundColor;
  late String actionContainerHeaderFontColor;
  late int actionContainerPaddingLeft;
  late int actionContainerPaddingTop;
  late int actionContainerPaddingRight;
  late int actionContainerPaddingBottom;
  late String actionItemActiveBackgroundColor;
  late String actionItemActiveFontColor;
  late String actionItemFontColor;
  late String actionQueryBoxFontColor;
  late String actionQueryBoxBackgroundColor;
  late int actionQueryBoxBorderRadius;
  late String previewFontColor;
  late String previewSplitLineColor;
  late String previewPropertyTitleColor;
  late String previewPropertyContentColor;
  late String previewTextSelectionColor;
  late String toolbarFontColor;
  late String toolbarBackgroundColor;
  late int toolbarPaddingLeft;
  late int toolbarPaddingRight;

  // Cached parsed Color objects for performance
  late Color appBackgroundColorParsed;
  late Color resultItemTitleColorParsed;
  late Color resultItemSubTitleColorParsed;
  late Color resultItemTailTextColorParsed;
  late Color resultItemActiveBackgroundColorParsed;
  late Color resultItemActiveTitleColorParsed;
  late Color resultItemActiveSubTitleColorParsed;
  late Color resultItemActiveTailTextColorParsed;
  late Color queryBoxFontColorParsed;
  late Color queryBoxBackgroundColorParsed;
  late Color queryBoxCursorColorParsed;
  late Color queryBoxTextSelectionBackgroundColorParsed;
  late Color queryBoxTextSelectionColorParsed;
  late Color actionContainerBackgroundColorParsed;
  late Color actionContainerHeaderFontColorParsed;
  late Color actionItemActiveBackgroundColorParsed;
  late Color actionItemActiveFontColorParsed;
  late Color actionItemFontColorParsed;
  late Color actionQueryBoxFontColorParsed;
  late Color actionQueryBoxBackgroundColorParsed;
  late Color previewFontColorParsed;
  late Color previewSplitLineColorParsed;
  late Color previewPropertyTitleColorParsed;
  late Color previewPropertyContentColorParsed;
  late Color previewTextSelectionColorParsed;
  late Color toolbarFontColorParsed;
  late Color toolbarBackgroundColorParsed;

  factory WoxTheme({
    String? themeId,
    String? themeName,
    String? themeAuthor,
    String? themeUrl,
    String? version,
    String? description,
    bool? isSystem,
    bool? isInstalled,
    bool? isUpgradable,
    bool? isAutoAppearance,
    String? darkThemeId,
    String? lightThemeId,
    String? appBackgroundColor,
    int? appPaddingLeft,
    int? appPaddingTop,
    int? appPaddingRight,
    int? appPaddingBottom,
    int? resultContainerPaddingLeft,
    int? resultContainerPaddingTop,
    int? resultContainerPaddingRight,
    int? resultContainerPaddingBottom,
    int? resultItemBorderRadius,
    int? resultItemPaddingLeft,
    int? resultItemPaddingTop,
    int? resultItemPaddingRight,
    int? resultItemPaddingBottom,
    String? resultItemTitleColor,
    String? resultItemSubTitleColor,
    String? resultItemTailTextColor,
    int? resultItemBorderLeftWidth,
    String? resultItemActiveBackgroundColor,
    String? resultItemActiveTitleColor,
    String? resultItemActiveSubTitleColor,
    String? resultItemActiveTailTextColor,
    int? resultItemActiveBorderLeftWidth,
    String? queryBoxFontColor,
    String? queryBoxBackgroundColor,
    int? queryBoxBorderRadius,
    String? queryBoxCursorColor,
    String? queryBoxTextSelectionBackgroundColor,
    String? queryBoxTextSelectionColor,
    String? actionContainerBackgroundColor,
    String? actionContainerHeaderFontColor,
    int? actionContainerPaddingLeft,
    int? actionContainerPaddingTop,
    int? actionContainerPaddingRight,
    int? actionContainerPaddingBottom,
    String? actionItemActiveBackgroundColor,
    String? actionItemActiveFontColor,
    String? actionItemFontColor,
    String? actionQueryBoxFontColor,
    String? actionQueryBoxBackgroundColor,
    int? actionQueryBoxBorderRadius,
    String? previewFontColor,
    String? previewSplitLineColor,
    String? previewPropertyTitleColor,
    String? previewPropertyContentColor,
    String? previewTextSelectionColor,
    String? toolbarFontColor,
    String? toolbarBackgroundColor,
    int? toolbarPaddingLeft,
    int? toolbarPaddingRight,
  }) {
    final theme = WoxTheme.empty();

    // Bug fix: the auto-appearance preview/icon fallbacks only provide a small subset of fields.
    // The previous constructor left the remaining `late` members unset, so reading `theme.isAutoAppearance`
    // in widgets crashed before the fallback theme could render. Start from empty defaults, then override
    // only the values supplied by the caller so every direct construction path stays safe.
    theme.themeId = themeId ?? theme.themeId;
    theme.themeName = themeName ?? theme.themeName;
    theme.themeAuthor = themeAuthor ?? theme.themeAuthor;
    theme.themeUrl = themeUrl ?? theme.themeUrl;
    theme.version = version ?? theme.version;
    theme.description = description ?? theme.description;
    theme.isSystem = isSystem ?? theme.isSystem;
    theme.isInstalled = isInstalled ?? theme.isInstalled;
    theme.isUpgradable = isUpgradable ?? theme.isUpgradable;
    theme.isAutoAppearance = isAutoAppearance ?? theme.isAutoAppearance;
    theme.darkThemeId = darkThemeId ?? theme.darkThemeId;
    theme.lightThemeId = lightThemeId ?? theme.lightThemeId;
    theme.appBackgroundColor = appBackgroundColor ?? theme.appBackgroundColor;
    theme.appPaddingLeft = appPaddingLeft ?? theme.appPaddingLeft;
    theme.appPaddingTop = appPaddingTop ?? theme.appPaddingTop;
    theme.appPaddingRight = appPaddingRight ?? theme.appPaddingRight;
    theme.appPaddingBottom = appPaddingBottom ?? theme.appPaddingBottom;
    theme.resultContainerPaddingLeft = resultContainerPaddingLeft ?? theme.resultContainerPaddingLeft;
    theme.resultContainerPaddingTop = resultContainerPaddingTop ?? theme.resultContainerPaddingTop;
    theme.resultContainerPaddingRight = resultContainerPaddingRight ?? theme.resultContainerPaddingRight;
    theme.resultContainerPaddingBottom = resultContainerPaddingBottom ?? theme.resultContainerPaddingBottom;
    theme.resultItemBorderRadius = resultItemBorderRadius ?? theme.resultItemBorderRadius;
    theme.resultItemPaddingLeft = resultItemPaddingLeft ?? theme.resultItemPaddingLeft;
    theme.resultItemPaddingTop = resultItemPaddingTop ?? theme.resultItemPaddingTop;
    theme.resultItemPaddingRight = resultItemPaddingRight ?? theme.resultItemPaddingRight;
    theme.resultItemPaddingBottom = resultItemPaddingBottom ?? theme.resultItemPaddingBottom;
    theme.resultItemTitleColor = resultItemTitleColor ?? theme.resultItemTitleColor;
    theme.resultItemSubTitleColor = resultItemSubTitleColor ?? theme.resultItemSubTitleColor;
    theme.resultItemTailTextColor = resultItemTailTextColor ?? theme.resultItemTailTextColor;
    theme.resultItemBorderLeftWidth = resultItemBorderLeftWidth ?? theme.resultItemBorderLeftWidth;
    theme.resultItemActiveBackgroundColor = resultItemActiveBackgroundColor ?? theme.resultItemActiveBackgroundColor;
    theme.resultItemActiveTitleColor = resultItemActiveTitleColor ?? theme.resultItemActiveTitleColor;
    theme.resultItemActiveSubTitleColor = resultItemActiveSubTitleColor ?? theme.resultItemActiveSubTitleColor;
    theme.resultItemActiveTailTextColor = resultItemActiveTailTextColor ?? theme.resultItemActiveTailTextColor;
    theme.resultItemActiveBorderLeftWidth = resultItemActiveBorderLeftWidth ?? theme.resultItemActiveBorderLeftWidth;
    theme.queryBoxFontColor = queryBoxFontColor ?? theme.queryBoxFontColor;
    theme.queryBoxBackgroundColor = queryBoxBackgroundColor ?? theme.queryBoxBackgroundColor;
    theme.queryBoxBorderRadius = queryBoxBorderRadius ?? theme.queryBoxBorderRadius;
    theme.queryBoxCursorColor = queryBoxCursorColor ?? theme.queryBoxCursorColor;
    theme.queryBoxTextSelectionBackgroundColor = queryBoxTextSelectionBackgroundColor ?? theme.queryBoxTextSelectionBackgroundColor;
    theme.queryBoxTextSelectionColor = queryBoxTextSelectionColor ?? theme.queryBoxTextSelectionColor;
    theme.actionContainerBackgroundColor = actionContainerBackgroundColor ?? theme.actionContainerBackgroundColor;
    theme.actionContainerHeaderFontColor = actionContainerHeaderFontColor ?? theme.actionContainerHeaderFontColor;
    theme.actionContainerPaddingLeft = actionContainerPaddingLeft ?? theme.actionContainerPaddingLeft;
    theme.actionContainerPaddingTop = actionContainerPaddingTop ?? theme.actionContainerPaddingTop;
    theme.actionContainerPaddingRight = actionContainerPaddingRight ?? theme.actionContainerPaddingRight;
    theme.actionContainerPaddingBottom = actionContainerPaddingBottom ?? theme.actionContainerPaddingBottom;
    theme.actionItemActiveBackgroundColor = actionItemActiveBackgroundColor ?? theme.actionItemActiveBackgroundColor;
    theme.actionItemActiveFontColor = actionItemActiveFontColor ?? theme.actionItemActiveFontColor;
    theme.actionItemFontColor = actionItemFontColor ?? theme.actionItemFontColor;
    theme.actionQueryBoxFontColor = actionQueryBoxFontColor ?? theme.actionQueryBoxFontColor;
    theme.actionQueryBoxBackgroundColor = actionQueryBoxBackgroundColor ?? theme.actionQueryBoxBackgroundColor;
    theme.actionQueryBoxBorderRadius = actionQueryBoxBorderRadius ?? theme.actionQueryBoxBorderRadius;
    theme.previewFontColor = previewFontColor ?? theme.previewFontColor;
    theme.previewSplitLineColor = previewSplitLineColor ?? theme.previewSplitLineColor;
    theme.previewPropertyTitleColor = previewPropertyTitleColor ?? theme.previewPropertyTitleColor;
    theme.previewPropertyContentColor = previewPropertyContentColor ?? theme.previewPropertyContentColor;
    theme.previewTextSelectionColor = previewTextSelectionColor ?? theme.previewTextSelectionColor;
    theme.toolbarFontColor = toolbarFontColor ?? theme.toolbarFontColor;
    theme.toolbarBackgroundColor = toolbarBackgroundColor ?? theme.toolbarBackgroundColor;
    theme.toolbarPaddingLeft = toolbarPaddingLeft ?? theme.toolbarPaddingLeft;
    theme.toolbarPaddingRight = toolbarPaddingRight ?? theme.toolbarPaddingRight;
    theme._parseColors();

    return theme;
  }

  WoxTheme.fromJson(Map<String, dynamic> json) {
    themeId = json['ThemeId'];
    themeName = json['ThemeName'];
    themeAuthor = json['ThemeAuthor'];
    themeUrl = json['ThemeUrl'];
    version = json['Version'];
    description = json['Description'];
    isSystem = json['IsSystem'] ?? false;
    isInstalled = json['IsInstalled'] ?? false;
    isUpgradable = json['IsUpgradable'] ?? false;
    isAutoAppearance = json['IsAutoAppearance'] ?? false;
    darkThemeId = json['DarkThemeId'] ?? '';
    lightThemeId = json['LightThemeId'] ?? '';
    appBackgroundColor = json['AppBackgroundColor'];
    appPaddingLeft = json['AppPaddingLeft'];
    appPaddingTop = json['AppPaddingTop'];
    appPaddingRight = json['AppPaddingRight'];
    appPaddingBottom = json['AppPaddingBottom'];
    resultContainerPaddingLeft = json['ResultContainerPaddingLeft'];
    resultContainerPaddingTop = json['ResultContainerPaddingTop'];
    resultContainerPaddingRight = json['ResultContainerPaddingRight'];
    resultContainerPaddingBottom = json['ResultContainerPaddingBottom'];
    resultItemBorderRadius = json['ResultItemBorderRadius'];
    resultItemPaddingLeft = json['ResultItemPaddingLeft'];
    resultItemPaddingTop = json['ResultItemPaddingTop'];
    resultItemPaddingRight = json['ResultItemPaddingRight'];
    resultItemPaddingBottom = json['ResultItemPaddingBottom'];
    resultItemTitleColor = json['ResultItemTitleColor'];
    resultItemSubTitleColor = json['ResultItemSubTitleColor'];
    resultItemTailTextColor = json['ResultItemTailTextColor'];
    resultItemBorderLeftWidth = _parseInt(json['ResultItemBorderLeftWidth'] ?? json['ResultItemBorderLeft']);
    resultItemActiveBackgroundColor = json['ResultItemActiveBackgroundColor'];
    resultItemActiveTitleColor = json['ResultItemActiveTitleColor'];
    resultItemActiveSubTitleColor = json['ResultItemActiveSubTitleColor'];
    resultItemActiveBorderLeftWidth = _parseInt(json['ResultItemActiveBorderLeftWidth'] ?? json['ResultItemActiveBorderLeft']);
    resultItemActiveTailTextColor = json['ResultItemActiveTailTextColor'];
    queryBoxFontColor = json['QueryBoxFontColor'];
    queryBoxBackgroundColor = json['QueryBoxBackgroundColor'];
    queryBoxBorderRadius = json['QueryBoxBorderRadius'];
    queryBoxCursorColor = json['QueryBoxCursorColor'];
    final selectionBackground = json['QueryBoxTextSelectionBackgroundColor'] ?? json['QueryBoxTextSelectionColor'];
    queryBoxTextSelectionBackgroundColor = selectionBackground ?? '';
    queryBoxTextSelectionColor = json['QueryBoxTextSelectionColor'] ?? json['ResultItemActiveTitleColor'] ?? '';
    actionContainerBackgroundColor = json['ActionContainerBackgroundColor'];
    actionContainerHeaderFontColor = json['ActionContainerHeaderFontColor'];
    actionContainerPaddingLeft = json['ActionContainerPaddingLeft'];
    actionContainerPaddingTop = json['ActionContainerPaddingTop'];
    actionContainerPaddingRight = json['ActionContainerPaddingRight'];
    actionContainerPaddingBottom = json['ActionContainerPaddingBottom'];
    actionItemActiveBackgroundColor = json['ActionItemActiveBackgroundColor'];
    actionItemActiveFontColor = json['ActionItemActiveFontColor'];
    actionItemFontColor = json['ActionItemFontColor'];
    actionQueryBoxFontColor = json['ActionQueryBoxFontColor'];
    actionQueryBoxBackgroundColor = json['ActionQueryBoxBackgroundColor'];
    actionQueryBoxBorderRadius = json['ActionQueryBoxBorderRadius'];
    previewFontColor = json['PreviewFontColor'];
    previewSplitLineColor = json['PreviewSplitLineColor'];
    previewPropertyTitleColor = json['PreviewPropertyTitleColor'];
    previewPropertyContentColor = json['PreviewPropertyContentColor'];
    previewTextSelectionColor = json['PreviewTextSelectionColor'];
    toolbarFontColor = json['ToolbarFontColor'];
    toolbarBackgroundColor = json['ToolbarBackgroundColor'];
    toolbarPaddingLeft = json['ToolbarPaddingLeft'];
    toolbarPaddingRight = json['ToolbarPaddingRight'];

    // Parse and cache Color objects
    _parseColors();
  }

  void _parseColors() {
    appBackgroundColorParsed = safeFromCssColor(appBackgroundColor);
    resultItemTitleColorParsed = safeFromCssColor(resultItemTitleColor);
    resultItemSubTitleColorParsed = safeFromCssColor(resultItemSubTitleColor);
    resultItemTailTextColorParsed = safeFromCssColor(resultItemTailTextColor);
    resultItemActiveBackgroundColorParsed = safeFromCssColor(resultItemActiveBackgroundColor);
    resultItemActiveTitleColorParsed = safeFromCssColor(resultItemActiveTitleColor);
    resultItemActiveSubTitleColorParsed = safeFromCssColor(resultItemActiveSubTitleColor);
    resultItemActiveTailTextColorParsed = safeFromCssColor(resultItemActiveTailTextColor);
    queryBoxFontColorParsed = safeFromCssColor(queryBoxFontColor);
    queryBoxBackgroundColorParsed = safeFromCssColor(queryBoxBackgroundColor);
    queryBoxCursorColorParsed = safeFromCssColor(queryBoxCursorColor);
    queryBoxTextSelectionBackgroundColorParsed = safeFromCssColor(queryBoxTextSelectionBackgroundColor);
    queryBoxTextSelectionColorParsed = safeFromCssColor(queryBoxTextSelectionColor);
    actionContainerBackgroundColorParsed = safeFromCssColor(actionContainerBackgroundColor);
    actionContainerHeaderFontColorParsed = safeFromCssColor(actionContainerHeaderFontColor);
    actionItemActiveBackgroundColorParsed = safeFromCssColor(actionItemActiveBackgroundColor);
    actionItemActiveFontColorParsed = safeFromCssColor(actionItemActiveFontColor);
    actionItemFontColorParsed = safeFromCssColor(actionItemFontColor);
    actionQueryBoxFontColorParsed = safeFromCssColor(actionQueryBoxFontColor);
    actionQueryBoxBackgroundColorParsed = safeFromCssColor(actionQueryBoxBackgroundColor);
    previewFontColorParsed = safeFromCssColor(previewFontColor);
    previewSplitLineColorParsed = safeFromCssColor(previewSplitLineColor);
    previewPropertyTitleColorParsed = safeFromCssColor(previewPropertyTitleColor);
    previewPropertyContentColorParsed = safeFromCssColor(previewPropertyContentColor);
    previewTextSelectionColorParsed = safeFromCssColor(previewTextSelectionColor);
    toolbarFontColorParsed = safeFromCssColor(toolbarFontColor);
    toolbarBackgroundColorParsed = safeFromCssColor(toolbarBackgroundColor);
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['ThemeId'] = themeId;
    data['ThemeName'] = themeName;
    data['ThemeAuthor'] = themeAuthor;
    data['ThemeUrl'] = themeUrl;
    data['Version'] = version;
    data['Description'] = description;
    data['IsSystem'] = isSystem;
    data['IsInstalled'] = isInstalled;
    data['IsUpgradable'] = isUpgradable;
    data['IsAutoAppearance'] = isAutoAppearance;
    data['DarkThemeId'] = darkThemeId;
    data['LightThemeId'] = lightThemeId;
    data['AppBackgroundColor'] = appBackgroundColor;
    data['AppPaddingLeft'] = appPaddingLeft;
    data['AppPaddingTop'] = appPaddingTop;
    data['AppPaddingRight'] = appPaddingRight;
    data['AppPaddingBottom'] = appPaddingBottom;
    data['ResultContainerPaddingLeft'] = resultContainerPaddingLeft;
    data['ResultContainerPaddingTop'] = resultContainerPaddingTop;
    data['ResultContainerPaddingRight'] = resultContainerPaddingRight;
    data['ResultContainerPaddingBottom'] = resultContainerPaddingBottom;
    data['ResultItemBorderRadius'] = resultItemBorderRadius;
    data['ResultItemPaddingLeft'] = resultItemPaddingLeft;
    data['ResultItemPaddingTop'] = resultItemPaddingTop;
    data['ResultItemPaddingRight'] = resultItemPaddingRight;
    data['ResultItemPaddingBottom'] = resultItemPaddingBottom;
    data['ResultItemTitleColor'] = resultItemTitleColor;
    data['ResultItemSubTitleColor'] = resultItemSubTitleColor;
    data['ResultItemTailTextColor'] = resultItemTailTextColor;
    data['ResultItemBorderLeftWidth'] = resultItemBorderLeftWidth;
    data['ResultItemActiveBackgroundColor'] = resultItemActiveBackgroundColor;
    data['ResultItemActiveTitleColor'] = resultItemActiveTitleColor;
    data['ResultItemActiveSubTitleColor'] = resultItemActiveSubTitleColor;
    data['ResultItemActiveBorderLeftWidth'] = resultItemActiveBorderLeftWidth;
    data['ResultItemActiveTailTextColor'] = resultItemActiveTailTextColor;
    data['QueryBoxFontColor'] = queryBoxFontColor;
    data['QueryBoxBackgroundColor'] = queryBoxBackgroundColor;
    data['QueryBoxBorderRadius'] = queryBoxBorderRadius;
    data['QueryBoxCursorColor'] = queryBoxCursorColor;
    data['QueryBoxTextSelectionBackgroundColor'] = queryBoxTextSelectionBackgroundColor;
    data['QueryBoxTextSelectionColor'] = queryBoxTextSelectionColor;
    data['ActionContainerBackgroundColor'] = actionContainerBackgroundColor;
    data['ActionContainerHeaderFontColor'] = actionContainerHeaderFontColor;
    data['ActionContainerPaddingLeft'] = actionContainerPaddingLeft;
    data['ActionContainerPaddingTop'] = actionContainerPaddingTop;
    data['ActionContainerPaddingRight'] = actionContainerPaddingRight;
    data['ActionContainerPaddingBottom'] = actionContainerPaddingBottom;
    data['ActionItemActiveBackgroundColor'] = actionItemActiveBackgroundColor;
    data['ActionItemActiveFontColor'] = actionItemActiveFontColor;
    data['ActionItemFontColor'] = actionItemFontColor;
    data['ActionQueryBoxFontColor'] = actionQueryBoxFontColor;
    data['ActionQueryBoxBackgroundColor'] = actionQueryBoxBackgroundColor;
    data['ActionQueryBoxBorderRadius'] = actionQueryBoxBorderRadius;
    data['PreviewFontColor'] = previewFontColor;
    data['PreviewSplitLineColor'] = previewSplitLineColor;
    data['PreviewPropertyTitleColor'] = previewPropertyTitleColor;
    data['PreviewPropertyContentColor'] = previewPropertyContentColor;
    data['PreviewTextSelectionColor'] = previewTextSelectionColor;
    data['ToolbarFontColor'] = toolbarFontColor;
    data['ToolbarBackgroundColor'] = toolbarBackgroundColor;
    data['ToolbarPaddingLeft'] = toolbarPaddingLeft;
    data['ToolbarPaddingRight'] = toolbarPaddingRight;
    return data;
  }

  WoxTheme.empty() {
    themeId = '';
    themeName = '';
    themeAuthor = '';
    themeUrl = '';
    version = '';
    description = '';
    isSystem = false;
    isInstalled = false;
    isUpgradable = false;
    isAutoAppearance = false;
    darkThemeId = '';
    lightThemeId = '';
    appBackgroundColor = '';
    appPaddingLeft = 0;
    appPaddingTop = 0;
    appPaddingRight = 0;
    appPaddingBottom = 0;
    resultContainerPaddingLeft = 0;
    resultContainerPaddingTop = 0;
    resultContainerPaddingRight = 0;
    resultContainerPaddingBottom = 0;
    resultItemBorderRadius = 0;
    resultItemPaddingLeft = 0;
    resultItemPaddingTop = 0;
    resultItemPaddingRight = 0;
    resultItemPaddingBottom = 0;
    resultItemTitleColor = '';
    resultItemSubTitleColor = '';
    resultItemTailTextColor = '';
    resultItemBorderLeftWidth = 0;
    resultItemActiveBackgroundColor = '';
    resultItemActiveTitleColor = '';
    resultItemActiveSubTitleColor = '';
    resultItemActiveBorderLeftWidth = 0;
    resultItemActiveTailTextColor = '';
    queryBoxFontColor = '';
    queryBoxBackgroundColor = '';
    queryBoxBorderRadius = 0;
    queryBoxCursorColor = '';
    queryBoxTextSelectionBackgroundColor = '';
    queryBoxTextSelectionColor = '';
    actionContainerBackgroundColor = '';
    actionContainerHeaderFontColor = '';
    actionContainerPaddingLeft = 0;
    actionContainerPaddingTop = 0;
    actionContainerPaddingRight = 0;
    actionContainerPaddingBottom = 0;
    actionItemActiveBackgroundColor = '';
    actionItemActiveFontColor = '';
    actionItemFontColor = '';
    actionQueryBoxFontColor = '';
    actionQueryBoxBackgroundColor = '';
    actionQueryBoxBorderRadius = 0;
    previewFontColor = '';
    previewSplitLineColor = '';
    previewPropertyTitleColor = '';
    previewPropertyContentColor = '';
    previewTextSelectionColor = '';
    toolbarFontColor = '';
    toolbarBackgroundColor = '';
    toolbarPaddingLeft = 0;
    toolbarPaddingRight = 0;
    _parseColors();
  }
}

int _parseInt(dynamic value, {int defaultValue = 0}) {
  if (value == null) {
    return defaultValue;
  }
  if (value is int) {
    return value;
  }
  if (value is double) {
    return value.toInt();
  }
  return int.tryParse(value.toString()) ?? defaultValue;
}
