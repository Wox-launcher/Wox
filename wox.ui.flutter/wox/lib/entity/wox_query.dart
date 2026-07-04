import 'dart:convert';
import 'package:flutter/foundation.dart';

import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_result_action_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';

typedef WoxLocalActionHandler = bool Function(String traceId);

class PlainQuery {
  late String queryId;
  late WoxQueryType queryType;
  late String queryText;
  late Selection querySelection;
  late Map<String, String> queryRefinements;
  late Map<String, String> contextData;

  PlainQuery({
    required this.queryId,
    required this.queryType,
    required this.queryText,
    required this.querySelection,
    Map<String, String>? queryRefinements,
    Map<String, String>? contextData,
  }) {
    // Query refinements are exposed to plugins as a simple string map. The UI
    // may keep list selections internally, but the transport mirrors the
    // plugin-facing API and joins multi-select values at the boundary.
    this.queryRefinements = queryRefinements ?? <String, String>{};
    this.contextData = contextData ?? <String, String>{};
  }

  static Map<String, String> parseQueryRefinements(dynamic rawRefinements) {
    if (rawRefinements is String && rawRefinements.isNotEmpty) {
      try {
        rawRefinements = jsonDecode(rawRefinements);
      } catch (_) {
        rawRefinements = <String, dynamic>{};
      }
    }
    if (rawRefinements is! Map) {
      return <String, String>{};
    }

    final parsed = <String, String>{};
    for (final entry in rawRefinements.entries) {
      final rawValue = entry.value;
      if (rawValue == null) {
        continue;
      }

      // Protocol migration: older payloads may still carry selected values as
      // arrays. Join them here so the rest of the UI and core transport use the
      // same map[string]string shape.
      if (rawValue is Iterable) {
        final encoded = rawValue.map((value) => value.toString()).where((value) => value.isNotEmpty).join(',');
        if (encoded.isNotEmpty) {
          parsed[entry.key.toString()] = encoded;
        }
        continue;
      }

      parsed[entry.key.toString()] = rawValue.toString();
    }
    return parsed;
  }

  PlainQuery.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'] ?? "";
    queryType = json['QueryType'];
    queryText = json['QueryText'];
    querySelection = Selection.fromJson(json['QuerySelection']);
    queryRefinements = parseQueryRefinements(json['QueryRefinements'] ?? json['queryRefinements']);
    contextData = parseQueryRefinements(json['ContextData'] ?? json['contextData']);
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['QueryId'] = queryId;
    data['QueryType'] = queryType;
    data['QueryText'] = queryText;
    data['QuerySelection'] = querySelection.toJson();
    data['QueryRefinements'] = queryRefinements;
    data['ContextData'] = contextData;
    return data;
  }

  bool get isEmpty => queryText.isEmpty && querySelection.type.isEmpty;

  static PlainQuery empty() {
    return PlainQuery(queryId: "", queryType: "", queryText: "", querySelection: Selection.empty());
  }

  static PlainQuery text(String text) {
    return PlainQuery(queryId: "", queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: text, querySelection: Selection.empty());
  }

  static PlainQuery emptyInput() {
    return PlainQuery(queryId: "", queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: "", querySelection: Selection.empty());
  }
}

class Selection {
  late WoxSelectionType type;

  // Only available when Type is SelectionTypeText
  late String text;

  // Only available when Type is SelectionTypeFile
  late List<String> filePaths;

  Selection.fromJson(Map<String, dynamic> json) {
    type = json['Type'];
    text = json['Text'];
    filePaths = List<String>.from(json['FilePaths'] ?? []);
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'Type': type, 'Text': text, 'FilePaths': filePaths};
  }

  Selection({required this.type, required this.text, required this.filePaths});

  static Selection empty() {
    return Selection(type: "", text: "", filePaths: []);
  }
}

class QueryHistory {
  PlainQuery? query;
  int? timestamp;

  QueryHistory.fromJson(Map<String, dynamic> json) {
    query = json['Query'] != null ? PlainQuery.fromJson(json['Query']) : null;
    timestamp = json['Timestamp'];
  }
}

class QueryCompletionHint {
  late String inputPrefix;
  late String completionText;
  late String suffix;
  late String source;
  late int score;

  QueryCompletionHint({required this.inputPrefix, required this.completionText, required this.suffix, required this.source, required this.score});

  QueryCompletionHint.fromJson(Map<String, dynamic> json) {
    inputPrefix = json['InputPrefix'] ?? "";
    completionText = json['CompletionText'] ?? "";
    suffix = json['Suffix'] ?? "";
    source = json['Source'] ?? "";
    score = (json['Score'] as num?)?.toInt() ?? 0;
  }
}

class WoxQueryResult {
  late String queryId;
  late String id;
  late String title;
  late String subTitle;
  late WoxImage icon;
  late WoxPreview preview;
  late int score;
  late String group;
  late int groupScore;
  late List<WoxListItemTail> tails;

  late List<WoxResultAction> actions;
  WoxResultDragData? dragData;

  // Used by the frontend to determine if this result is a group
  late bool isGroup;

  WoxQueryResult({
    required this.queryId,
    required this.id,
    required this.title,
    required this.subTitle,
    required this.icon,
    required this.preview,
    required this.score,
    required this.group,
    required this.groupScore,
    required this.tails,
    required this.actions,
    this.dragData,
    required this.isGroup,
  });

  WoxQueryResult.empty() {
    queryId = "";
    id = "";
    title = "";
    subTitle = "";
    icon = WoxImage.empty();
    preview = WoxPreview.empty();
    score = 0;
    group = "";
    groupScore = 0;
    tails = [];
    actions = [];
    dragData = null;
    isGroup = false;
  }

  WoxQueryResult.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'];
    id = json['Id'];
    title = json['Title'];
    subTitle = json['SubTitle'];
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : null)!;
    preview = (json['Preview'] != null ? WoxPreview.fromJson(json['Preview']) : null)!;
    score = json['Score'];
    group = json['Group'];
    groupScore = json['GroupScore'];
    final rawDragData = json['DragData'];
    dragData = rawDragData is Map ? WoxResultDragData.fromJson(Map<String, dynamic>.from(rawDragData)) : null;

    if (json['Tails'] != null) {
      tails = [];
      json['Tails'].forEach((v) {
        tails.add(WoxListItemTail.fromJson(v));
      });
    } else {
      tails = [];
    }

    if (json['Actions'] != null) {
      actions = [];
      json['Actions'].forEach((v) {
        var action = WoxResultAction.fromJson(v);
        action.resultId = id;
        actions.add(action);
      });
    } else {
      actions = [];
    }

    isGroup = json['IsGroup'] == true;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['QueryId'] = queryId;
    data['Id'] = id;
    data['Title'] = title;
    data['SubTitle'] = subTitle;
    data['Icon'] = icon.toJson();
    data['Preview'] = preview.toJson();
    data['Score'] = score;
    data['Group'] = group;
    data['GroupScore'] = groupScore;
    data['Actions'] = actions.map((v) => v.toJson()).toList();
    data['Tails'] = tails.map((v) => v.toJson()).toList();
    data['DragData'] = dragData?.toJson();
    data['IsGroup'] = isGroup;
    return data;
  }
}

class WoxResultDragData {
  static const String typeFiles = "files";

  late String type;
  late List<String> files;

  WoxResultDragData({required this.type, required this.files});

  WoxResultDragData.fromJson(Map<String, dynamic> json) {
    type = json['Type'] ?? "";
    files = List<String>.from((json['Files'] ?? const <dynamic>[]).map((filePath) => filePath.toString()));
  }

  bool get isFiles => type == typeFiles && files.isNotEmpty;

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Type'] = type;
    data['Files'] = files;
    return data;
  }
}

class WoxResultAction {
  late String id;
  late WoxResultActionType type;
  late String name;
  late WoxImage icon;
  late bool isDefault;
  late bool preventHideAfterAction;
  late String hotkey;
  late bool isSystemAction;
  late String resultId;
  late Map<String, String> contextData;
  late List<PluginSettingDefinitionItem> form;
  WoxLocalActionHandler? localActionHandler;

  WoxResultAction({
    required this.id,
    required this.type,
    required this.name,
    required this.icon,
    required this.isDefault,
    required this.preventHideAfterAction,
    required this.hotkey,
    required this.isSystemAction,
    required this.resultId,
    required this.contextData,
    required this.form,
    this.localActionHandler,
  });

  WoxResultAction.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    type = json['Type'] ?? WoxResultActionTypeEnum.WOX_RESULT_ACTION_TYPE_EXECUTE.code;
    name = json['Name'];
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : null)!;
    isDefault = json['IsDefault'];
    preventHideAfterAction = json['PreventHideAfterAction'];
    if (json['Hotkey'] != null) {
      hotkey = json['Hotkey'];
    }
    isSystemAction = json['IsSystemAction'];
    resultId = json['ResultId'] ?? "";
    final rawContextData = json['ContextData'];
    if (rawContextData is Map) {
      contextData = rawContextData.map((key, value) => MapEntry(key.toString(), value.toString()));
    } else if (rawContextData is String && rawContextData.isNotEmpty) {
      try {
        final decoded = jsonDecode(rawContextData);
        if (decoded is Map) {
          contextData = decoded.map((key, value) => MapEntry(key.toString(), value.toString()));
        } else {
          contextData = {};
        }
      } catch (_) {
        contextData = {};
      }
    } else {
      contextData = {};
    }
    if (json['Form'] != null) {
      form = (json['Form'] as List<dynamic>).map((e) => PluginSettingDefinitionItem.fromJson(e)).toList();
    } else {
      form = [];
    }
    localActionHandler = null;
  }

  WoxResultAction copyWith({
    String? id,
    WoxResultActionType? type,
    String? name,
    WoxImage? icon,
    bool? isDefault,
    bool? preventHideAfterAction,
    String? hotkey,
    bool? isSystemAction,
    String? resultId,
    Map<String, String>? contextData,
    List<PluginSettingDefinitionItem>? form,
    WoxLocalActionHandler? localActionHandler,
  }) {
    return WoxResultAction(
      id: id ?? this.id,
      type: type ?? this.type,
      name: name ?? this.name,
      icon: icon ?? this.icon,
      isDefault: isDefault ?? this.isDefault,
      preventHideAfterAction: preventHideAfterAction ?? this.preventHideAfterAction,
      hotkey: hotkey ?? this.hotkey,
      isSystemAction: isSystemAction ?? this.isSystemAction,
      resultId: resultId ?? this.resultId,
      contextData: contextData != null ? Map<String, String>.from(contextData) : Map<String, String>.from(this.contextData),
      form: form != null ? List<PluginSettingDefinitionItem>.from(form) : List<PluginSettingDefinitionItem>.from(this.form),
      localActionHandler: localActionHandler ?? this.localActionHandler,
    );
  }

  static WoxResultAction local({
    required String id,
    required String name,
    required String hotkey,
    required WoxLocalActionHandler handler,
    String? emoji,
    WoxImage? icon,
    bool preventHideAfterAction = true,
    bool isDefault = false,
  }) {
    return WoxResultAction(
      id: id,
      type: WoxResultActionTypeEnum.WOX_RESULT_ACTION_TYPE_LOCAL.code,
      name: name,
      icon: icon ?? WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: emoji ?? ""),
      isDefault: isDefault,
      preventHideAfterAction: preventHideAfterAction,
      hotkey: hotkey,
      isSystemAction: true,
      resultId: "",
      contextData: {},
      form: [],
      localActionHandler: handler,
    );
  }

  bool runLocalAction(String traceId) {
    return localActionHandler?.call(traceId) ?? false;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Type'] = type;
    data['Name'] = name;
    data['Icon'] = icon.toJson();
    data['IsDefault'] = isDefault;
    data['PreventHideAfterAction'] = preventHideAfterAction;
    data['Hotkey'] = hotkey;
    data['IsSystemAction'] = isSystemAction;
    data['ResultId'] = resultId;
    data['ContextData'] = contextData;
    data['Form'] = form.map((e) => {"Type": e.type, "Value": e.value, "DisabledInPlatforms": e.disabledInPlatforms, "IsPlatformSpecific": e.isPlatformSpecific}).toList();
    return data;
  }

  static WoxResultAction empty() {
    return WoxResultAction(
      id: "",
      type: WoxResultActionTypeEnum.WOX_RESULT_ACTION_TYPE_EXECUTE.code,
      name: "",
      icon: WoxImage.empty(),
      isDefault: false,
      preventHideAfterAction: false,
      hotkey: "",
      isSystemAction: false,
      resultId: "",
      contextData: {},
      form: [],
      localActionHandler: null,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is WoxResultAction &&
        other.id == id &&
        other.type == type &&
        other.name == name &&
        other.icon == icon &&
        other.isDefault == isDefault &&
        other.preventHideAfterAction == preventHideAfterAction &&
        other.hotkey == hotkey &&
        other.isSystemAction == isSystemAction &&
        other.resultId == resultId &&
        mapEquals(other.contextData, contextData) &&
        listEquals(other.form, form); // Note: this requires PluginSettingDefinitionItem equality or relying on identity/empty
    // Since PluginSettingDefinitionItem doesn't enforce equality, listEquals might fail to catch deep equality if instances differ.
    // For now, if form is critical we should rely on json comparison or implement equality there too.
    // Given the context, form updates are rare in this path.
  }

  @override
  int get hashCode {
    return id.hashCode ^
        type.hashCode ^
        name.hashCode ^
        icon.hashCode ^
        isDefault.hashCode ^
        preventHideAfterAction.hashCode ^
        hotkey.hashCode ^
        isSystemAction.hashCode ^
        resultId.hashCode ^
        contextData.hashCode ^
        form.hashCode;
  }
}

class Position {
  late WoxPositionType type;
  late int x;
  late int y;

  Position({required this.type, required this.x, required this.y});

  Position.fromJson(Map<String, dynamic> json) {
    type = json['Type'];
    x = json['X'];
    y = json['Y'];
  }
}

class WindowRect {
  late int x;
  late int y;
  late int width;
  late int height;

  WindowRect({required this.x, required this.y, required this.width, required this.height});

  WindowRect.fromJson(Map<String, dynamic> json) {
    x = json['X'] ?? 0;
    y = json['Y'] ?? 0;
    width = json['Width'] ?? 0;
    height = json['Height'] ?? 0;
  }
}

class TrayAnchor {
  late int windowX;
  late int bottom;
  late WindowRect screenRect;

  TrayAnchor({required this.windowX, required this.bottom, required this.screenRect});

  TrayAnchor.fromJson(Map<String, dynamic> json) {
    windowX = json['WindowX'] ?? 0;
    bottom = json['Bottom'] ?? 0;
    if (json['ScreenRect'] != null) {
      screenRect = WindowRect.fromJson(json['ScreenRect']);
    } else {
      screenRect = WindowRect(x: 0, y: 0, width: 0, height: 0);
    }
  }
}

class ShowAppParams {
  late bool selectAll;
  late Position position;
  TrayAnchor? trayAnchor;
  late int windowWidth;
  late int maxResultCount;
  late List<QueryHistory> queryHistories;
  late String launchMode;
  late String startPage;
  late bool hideQueryBox;
  late bool hideToolbar;
  late bool queryBoxAtBottom;
  late bool hideOnBlur;
  late String showSource;
  late int activationStartedAt;
  late int attentionUnreadCount;

  ShowAppParams({
    required this.selectAll,
    required this.position,
    this.windowWidth = 0,
    this.maxResultCount = 0,
    required this.queryHistories,
    required this.launchMode,
    required this.startPage,
    this.hideQueryBox = false,
    this.hideToolbar = false,
    this.queryBoxAtBottom = false,
    this.hideOnBlur = false,
    this.showSource = 'default',
    this.activationStartedAt = 0,
    this.attentionUnreadCount = 0,
  });

  ShowAppParams.fromJson(Map<String, dynamic> json) {
    selectAll = json['SelectAll'];
    if (json['Position'] != null) {
      position = Position.fromJson(json['Position']);
    }
    if (json['TrayAnchor'] != null) {
      trayAnchor = TrayAnchor.fromJson(json['TrayAnchor']);
    }
    windowWidth = json['WindowWidth'] ?? 0;
    maxResultCount = json['MaxResultCount'] ?? 0;
    queryHistories = <QueryHistory>[];
    if (json['QueryHistories'] != null) {
      final List<dynamic> histories = json['QueryHistories'];
      queryHistories = histories.map((v) => QueryHistory.fromJson(v)).toList();
    }
    launchMode = json['LaunchMode'] ?? 'continue';
    startPage = json['StartPage'] ?? 'mru';
    hideQueryBox = json['HideQueryBox'] ?? false;
    hideToolbar = json['HideToolbar'] ?? false;
    queryBoxAtBottom = json['QueryBoxAtBottom'] ?? false;
    hideOnBlur = json['HideOnBlur'] ?? false;
    showSource = json['ShowSource'] ?? 'default';
    activationStartedAt = (json['ActivationStartedAt'] as num?)?.toInt() ?? 0;
    attentionUnreadCount = (json['AttentionUnreadCount'] as num?)?.toInt() ?? 0;
  }
}

class WoxListViewItemParams {
  late String title;
  late String subTitle;
  late WoxImage icon;

  WoxListViewItemParams.fromJson(Map<String, dynamic> json) {
    title = json['Title'];
    subTitle = json['SubTitle'] ?? "";
    icon = json['Icon'];
  }
}

class QueryIconInfo {
  final WoxImage icon;
  final Function()? action;

  QueryIconInfo({required this.icon, this.action});

  static QueryIconInfo empty() {
    return QueryIconInfo(icon: WoxImage.empty());
  }
}

/// QueryRefinement describes one plugin-owned filter or sort control.
///
/// The backend already transports refinements with QueryResponse. Flutter keeps
/// the model close to result/query entities so every launcher view consumes the
/// same parsed shape instead of re-reading raw websocket maps.
class WoxQueryRefinement {
  late String id;
  late String title;
  late String type;
  late List<WoxQueryRefinementOption> options;
  late List<String> defaultValue;
  late String hotkey;
  late bool persist;

  WoxQueryRefinement({required this.id, required this.title, required this.type, required this.options, required this.defaultValue, required this.hotkey, required this.persist});

  WoxQueryRefinement.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? json['id'] ?? "";
    title = json['Title'] ?? json['title'] ?? "";
    type = json['Type'] ?? json['type'] ?? "";
    final rawOptions = json['Options'] ?? json['options'];
    options =
        rawOptions is List
            ? rawOptions.whereType<Map>().map((option) => WoxQueryRefinementOption.fromJson(Map<String, dynamic>.from(option))).toList()
            : <WoxQueryRefinementOption>[];
    defaultValue = List<String>.from((json['DefaultValue'] ?? json['defaultValue'] ?? const <dynamic>[]).map((value) => value.toString()));
    hotkey = json['Hotkey'] ?? json['hotkey'] ?? "";
    persist = json['Persist'] ?? json['persist'] ?? false;
  }

  bool get isEmpty => id.isEmpty || type.isEmpty;
}

/// One option inside a query refinement control.
///
/// Count and keywords are optional plugin hints. The first UI pass displays the
/// count and keeps keywords parsed so later keyboard/search affordances do not
/// need another transport change.
class WoxQueryRefinementOption {
  late String value;
  late String title;
  late WoxImage icon;
  late List<String> keywords;
  late int? count;

  WoxQueryRefinementOption({required this.value, required this.title, required this.icon, required this.keywords, required this.count});

  WoxQueryRefinementOption.fromJson(Map<String, dynamic> json) {
    value = json['Value'] ?? json['value'] ?? "";
    title = json['Title'] ?? json['title'] ?? "";
    final iconJson = json['Icon'] ?? json['icon'];
    icon = iconJson is Map ? WoxImage.fromJson(Map<String, dynamic>.from(iconJson)) : WoxImage.empty();
    keywords = List<String>.from((json['Keywords'] ?? json['keywords'] ?? const <dynamic>[]).map((keyword) => keyword.toString()));
    final rawCount = json['Count'] ?? json['count'];
    count = rawCount is int ? rawCount : int.tryParse(rawCount?.toString() ?? "");
  }
}

class UpdatableResult {
  late String id;
  String? title;
  String? subTitle;
  WoxImage? icon;
  List<WoxListItemTail>? tails;
  WoxPreview? preview;
  List<WoxResultAction>? actions;
  bool hasDragDataUpdate = false;
  WoxResultDragData? dragData;

  UpdatableResult({required this.id, this.title, this.subTitle, this.icon, this.tails, this.preview, this.actions, this.hasDragDataUpdate = false, this.dragData});

  UpdatableResult.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    title = json['Title'];
    subTitle = json['SubTitle'];
    if (json['Icon'] != null) {
      icon = WoxImage.fromJson(json['Icon']);
    }

    if (json['Tails'] != null) {
      tails = [];
      json['Tails'].forEach((v) {
        tails!.add(WoxListItemTail.fromJson(v));
      });
    }

    if (json['Preview'] != null) {
      preview = WoxPreview.fromJson(json['Preview']);
    }

    if (json['Actions'] != null) {
      actions = [];
      json['Actions'].forEach((v) {
        var action = WoxResultAction.fromJson(v);
        action.resultId = id;
        actions!.add(action);
      });
    }

    hasDragDataUpdate = json.containsKey('DragData');
    final rawDragData = json['DragData'];
    dragData = rawDragData is Map ? WoxResultDragData.fromJson(Map<String, dynamic>.from(rawDragData)) : null;
  }
}

/// Query-scoped presentation hints delivered with QueryResponse.
///
/// The old query metadata HTTP request used a different shape, so this parser
/// accepts both field names during the transition. The nullable ratio preserves
/// the difference between "unset" and an explicit zero preview-only layout.
class QueryLayout {
  late WoxImage icon;
  late double? resultPreviewWidthRatio;
  late bool isGridLayout;
  late GridLayoutParams gridLayoutParams;
  late bool chatMode;

  QueryLayout({required this.icon, required this.resultPreviewWidthRatio, required this.isGridLayout, required this.gridLayoutParams, this.chatMode = false});

  QueryLayout.fromJson(Map<String, dynamic> json) {
    final iconJson = json['Icon'];
    if (iconJson is Map) {
      icon = WoxImage.fromJson(Map<String, dynamic>.from(iconJson));
    } else {
      icon = WoxImage.empty();
    }

    final widthRatio = json['ResultPreviewWidthRatio'] ?? json['WidthRatio'];
    if (widthRatio != null) {
      resultPreviewWidthRatio = widthRatio.toDouble();
    } else {
      resultPreviewWidthRatio = null;
    }

    final gridLayoutJson = json['GridLayout'] ?? json['GridLayoutParams'];
    final hasGridLayoutPayload = gridLayoutJson is Map && gridLayoutJson.isNotEmpty;
    isGridLayout = json['IsGridLayout'] ?? hasGridLayoutPayload;
    if (gridLayoutJson is Map) {
      gridLayoutParams = GridLayoutParams.fromJson(Map<String, dynamic>.from(gridLayoutJson));
    } else {
      gridLayoutParams = GridLayoutParams.empty();
    }

    chatMode = json['ChatMode'] == true;
  }

  QueryLayout.empty() {
    icon = WoxImage.empty();
    resultPreviewWidthRatio = null;
    isGridLayout = false;
    gridLayoutParams = GridLayoutParams.empty();
    chatMode = false;
  }

  bool get isEmpty {
    return icon.imageData.isEmpty && resultPreviewWidthRatio == null && !isGridLayout && !chatMode;
  }
}

/// Backend-owned query classification delivered with QueryResponse.
///
/// Flutter uses this to correct local trigger-keyword guesses after core has
/// applied shortcuts and parsed the query against the actual plugin registry.
class QueryContext {
  late bool isGlobalQuery;
  late String pluginId;

  QueryContext({required this.isGlobalQuery, required this.pluginId});

  QueryContext.fromJson(Map<String, dynamic> json) {
    isGlobalQuery = json['IsGlobalQuery'] ?? false;
    pluginId = json['PluginId'] ?? '';
  }

  QueryContext.empty() {
    isGlobalQuery = false;
    pluginId = '';
  }
}

/// Parameters for grid layout display
class GridLayoutParams {
  late int columns; // number of columns per row
  late bool showTitle; // whether to show title below icon
  late double itemPadding; // padding inside each item
  late double itemMargin; // margin outside each item (all sides)
  late double aspectRatio; // width / height for each grid visual item
  late List<String> commands; // commands to enable grid layout for, empty means all

  GridLayoutParams({required this.columns, required this.showTitle, required this.itemPadding, required this.itemMargin, required this.aspectRatio, required this.commands});

  GridLayoutParams.fromJson(Map<String, dynamic> json) {
    columns = json['Columns'] ?? 8;
    showTitle = json['ShowTitle'] ?? false;
    // Behavior change: the grid active state is now an outline, so missing
    // ItemPadding should mean no extra inner gap. The old 12px fallback was
    // tied to filled-background selection and made media/emoji grids look
    // padded even when plugin metadata omitted ItemPadding.
    itemPadding = (json['ItemPadding'] ?? 0).toDouble();
    itemMargin = (json['ItemMargin'] ?? 6).toDouble();
    aspectRatio = (json['AspectRatio'] ?? 1.0).toDouble();
    if (aspectRatio <= 0) {
      aspectRatio = 1.0;
    }
    commands = json['Commands'] != null ? List<String>.from(json['Commands']) : [];
  }

  GridLayoutParams.empty() {
    columns = 0;
    showTitle = false;
    itemPadding = 0;
    itemMargin = 6;
    aspectRatio = 1.0;
    commands = [];
  }
}
