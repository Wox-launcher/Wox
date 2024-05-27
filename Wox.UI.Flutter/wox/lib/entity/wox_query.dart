import 'package:get/get.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/enums/wox_last_query_mode_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';

class PlainQuery {
  late String queryId;
  late WoxQueryType queryType;
  late String queryText;
  late Selection querySelection;

  PlainQuery({required this.queryId, required this.queryType, required this.queryText, required this.querySelection});

  PlainQuery.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'] ?? "";
    queryType = json['QueryType'];
    queryText = json['QueryText'];
    querySelection = Selection.fromJson(json['QuerySelection']);
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['QueryId'] = queryId;
    data['QueryType'] = queryType;
    data['QueryText'] = queryText;
    data['QuerySelection'] = querySelection.toJson();
    return data;
  }

  bool get isEmpty => queryText.isEmpty && querySelection.type.isEmpty;

  static PlainQuery empty() {
    return PlainQuery(queryId: "", queryType: "", queryText: "", querySelection: Selection.empty());
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
    return <String, dynamic>{
      'Type': type,
      'Text': text,
      'FilePaths': filePaths,
    };
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

class WoxQueryResult {
  late String queryId;
  late String id;
  late Rx<String> title;
  late Rx<String> subTitle;
  late Rx<WoxImage> icon;
  late WoxPreview preview;
  late int score;
  late String contextData;
  late List<WoxResultAction> actions;
  late int refreshInterval;

  WoxQueryResult(
      {required this.queryId,
      required this.id,
      required this.title,
      required this.subTitle,
      required this.icon,
      required this.preview,
      required this.score,
      required this.contextData,
      required this.actions,
      required this.refreshInterval});

  WoxQueryResult.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'];
    id = json['Id'];
    title = RxString(json['Title']);
    subTitle = RxString(json['SubTitle']);
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']).obs : null)!;
    preview = (json['Preview'] != null ? WoxPreview.fromJson(json['Preview']) : null)!;
    score = json['Score'];
    contextData = json['ContextData'];
    if (json['Actions'] != null) {
      actions = <WoxResultAction>[];
      json['Actions'].forEach((v) {
        actions.add(WoxResultAction.fromJson(v));
      });
    }
    refreshInterval = json['RefreshInterval'];
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
    data['ContextData'] = contextData;
    data['Actions'] = actions.map((v) => v.toJson()).toList();
    data['RefreshInterval'] = refreshInterval;
    return data;
  }
}

class WoxResultAction {
  late String id;
  late Rx<String> name;
  late Rx<WoxImage> icon;
  late bool isDefault;
  late bool preventHideAfterAction;

  WoxResultAction({required this.id, required this.name, required this.icon, required this.isDefault, required this.preventHideAfterAction});

  WoxResultAction.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = RxString(json['Name']);
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']).obs : null)!;
    isDefault = json['IsDefault'];
    preventHideAfterAction = json['PreventHideAfterAction'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Name'] = name;
    data['Icon'] = icon.toJson();
    data['IsDefault'] = isDefault;
    data['PreventHideAfterAction'] = preventHideAfterAction;
    return data;
  }

  static WoxResultAction empty() {
    return WoxResultAction(id: "", name: "".obs, icon: WoxImage.empty().obs, isDefault: false, preventHideAfterAction: false);
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

class ShowAppParams {
  late bool selectAll;
  late Position position;
  late List<QueryHistory> queryHistories;
  late WoxLastQueryMode lastQueryMode;

  ShowAppParams({required this.selectAll, required this.position, required this.queryHistories, required this.lastQueryMode});

  ShowAppParams.fromJson(Map<String, dynamic> json) {
    selectAll = json['SelectAll'];
    if (json['Position'] != null) {
      position = Position.fromJson(json['Position']);
    }
    queryHistories = <QueryHistory>[];
    if (json['QueryHistories'] != null) {
      json['QueryHistories'].forEach((v) {
        queryHistories.add(QueryHistory.fromJson(v));
      });
    } else {
      queryHistories = <QueryHistory>[];
    }
    lastQueryMode = json['LastQueryMode'];
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

class WoxRefreshableResult {
  late String resultId;
  late String title;
  late String subTitle;
  late WoxImage icon;
  late WoxPreview preview;
  late String contextData;
  late int refreshInterval;

  WoxRefreshableResult(
      {required this.resultId, required this.title, required this.subTitle, required this.icon, required this.preview, required this.contextData, required this.refreshInterval});

  WoxRefreshableResult.fromJson(Map<String, dynamic> json) {
    resultId = json['ResultId'];
    title = json['Title'];
    subTitle = json['SubTitle'] ?? "";
    icon = WoxImage.fromJson(json['Icon']);
    preview = WoxPreview.fromJson(json['Preview']);
    contextData = json['ContextData'];
    refreshInterval = json['RefreshInterval'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['ResultId'] = resultId;
    data['Title'] = title;
    data['SubTitle'] = subTitle;
    data['Icon'] = icon.toJson();
    data['Preview'] = preview.toJson();
    data['ContextData'] = contextData;
    data['RefreshInterval'] = refreshInterval;
    return data;
  }
}
