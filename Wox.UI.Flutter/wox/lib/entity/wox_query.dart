import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/enums/wox_last_query_mode_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';

class WoxChangeQuery {
  late String queryId;
  late WoxQueryType queryType;
  late String queryText;
  late Selection querySelection;

  WoxChangeQuery({required this.queryId, required this.queryType, required this.queryText, required this.querySelection});

  WoxChangeQuery.fromJson(Map<String, dynamic> json) {
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

  static WoxChangeQuery empty() {
    return WoxChangeQuery(queryId: "", queryType: "", queryText: "", querySelection: Selection.empty());
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
  WoxChangeQuery? query;
  int? timestamp;

  QueryHistory.fromJson(Map<String, dynamic> json) {
    query = json['Query'] != null ? WoxChangeQuery.fromJson(json['Query']) : null;
    timestamp = json['Timestamp'];
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
    title = json['Title'];
    subTitle = json['SubTitle'];
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : null)!;
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
  late String name;
  late WoxImage icon;
  late bool isDefault;
  late bool preventHideAfterAction;

  WoxResultAction({required this.id, required this.name, required this.icon, required this.isDefault, required this.preventHideAfterAction});

  WoxResultAction.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : null)!;
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
}

class Position {
  late WoxPositionType type;
  late int x;
  late int y;

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

  ShowAppParams.fromJson(Map<String, dynamic> json) {
    selectAll = json['SelectAll'];
    position = Position.fromJson(json['Position']);
    queryHistories = <QueryHistory>[];
    json['QueryHistories'].forEach((v) {
      queryHistories.add(QueryHistory.fromJson(v));
    });
    lastQueryMode = json['LastQueryMode'];
  }
}
