import 'package:wox/entity/wox_image.dart';

typedef SelectionType = String;
typedef LastQueryMode = String;
typedef QueryType = String;
typedef WoxImageType = String;
typedef WoxPreviewType = String;
typedef WebsocketMsgType = String;
typedef PositionType = String;

const PositionType positionTypeMouseScreen = "MouseScreen";
const PositionType positionTypeLastLocation = "LastLocation";

const WebsocketMsgType websocketMsgTypeRequest = "WebsocketMsgTypeRequest";
const WebsocketMsgType websocketMsgTypeResponse = "WebsocketMsgTypeResponse";

const SelectionType selectionTypeText = "text";
const SelectionType selectionTypeFile = "file";

const LastQueryMode lastQueryModePreserve = "preserve";
const LastQueryMode lastQueryModeEmpty = "empty";

const QueryType queryTypeInput = "input";
const QueryType queryTypeSelection = "selection";

const WoxImageType woxImageTypeAbsolutePath = "absolute";
const WoxImageType woxImageTypeRelativePath = "relative";
const WoxImageType woxImageTypeBase64 = "base64";
const WoxImageType woxImageTypeSvg = "svg";
const WoxImageType woxImageTypeUrl = "url";

const WoxPreviewType woxPreviewTypeMarkdown = "markdown";
const WoxPreviewType woxPreviewTypeText = "text";
const WoxPreviewType woxPreviewTypeImage = "image";
const WoxPreviewType woxPreviewTypeUrl = "url";

class Selection {
  late SelectionType type;

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

class ChangedQuery {
  late String queryId;
  late QueryType queryType;
  late String queryText;
  late Selection querySelection;

  ChangedQuery.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'] ?? "";
    queryType = json['QueryType'];
    queryText = json['QueryText'];
    querySelection = Selection.fromJson(json['QuerySelection']);
  }

  bool get isEmpty => queryText.isEmpty && querySelection.type.isEmpty;

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'QueryId': queryId,
      'QueryType': queryType,
      'QueryText': queryText,
      'QuerySelection': querySelection.toJson(),
    };
  }

  ChangedQuery({required this.queryId, required this.queryType, required this.queryText, required this.querySelection});

  static ChangedQuery empty() {
    return ChangedQuery(queryId: "", queryType: "", queryText: "", querySelection: Selection.empty());
  }

  @override
  String toString() {
    if (queryType == queryTypeInput) {
      return queryText;
    } else {
      if (querySelection.type == selectionTypeFile) {
        return querySelection.filePaths.join(";");
      } else {
        return querySelection.text;
      }
    }
  }
}

class QueryHistory {
  ChangedQuery? query;
  int? timestamp;

  QueryHistory.fromJson(Map<String, dynamic> json) {
    query = json['Query'] != null ? ChangedQuery.fromJson(json['Query']) : null;
    timestamp = json['Timestamp'];
  }
}

class WoxPreview {
  late WoxPreviewType previewType;
  late String previewData;
  late Map<String, String> previewProperties;

  WoxPreview({required this.previewType, required this.previewData, required this.previewProperties});

  WoxPreview.fromJson(Map<String, dynamic> json) {
    previewType = json['PreviewType'];
    previewData = json['PreviewData'];
    previewProperties = Map<String, String>.from(json['PreviewProperties'] ?? {});
  }

  static WoxPreview empty() {
    return WoxPreview(previewType: "", previewData: "", previewProperties: {});
  }
}

class WoxResultAction {
  late String id;
  late String name;
  late WoxImage icon;
  late bool isDefault;
  late bool preventHideAfterAction;

  WoxResultAction.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    icon = WoxImage.fromJson(json['Icon']);
    isDefault = json['IsDefault'];
    preventHideAfterAction = json['PreventHideAfterAction'];
  }
}

class QueryResult {
  late String queryId;
  late String id;
  late String title;
  late String subTitle;
  late WoxImage icon;
  late int score;
  late WoxPreview preview;
  late String contextData;
  late List<WoxResultAction> actions;
  late int refreshInterval;

  QueryResult.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'];
    id = json['Id'];
    title = json['Title'];
    subTitle = json['SubTitle'];
    icon = WoxImage.fromJson(json['Icon']);
    score = json['Score'];
    preview = WoxPreview.fromJson(json['Preview']);
    contextData = json['ContextData'];
    actions = <WoxResultAction>[];
    json['Actions']?.forEach((v) {
      actions.add(WoxResultAction.fromJson(v));
    });
    refreshInterval = json['RefreshInterval'];
  }
}

class WebsocketMsg {
  late String? id;
  late String? type;
  late String? method;
  late bool? success;
  late dynamic data;

  WebsocketMsg({required this.id, this.type = websocketMsgTypeRequest, required this.method, this.success = true, required this.data});

  WebsocketMsg.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    type = json['Type'];
    method = json['Method'];
    success = json['Success'];
    data = json['Data'];
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'Id': id,
      'Type': type,
      'Method': method,
      'Success': success,
      'Data': data,
    };
  }
}

class Position {
  late PositionType type;
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
  late LastQueryMode lastQueryMode;

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
