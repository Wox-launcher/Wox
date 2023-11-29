typedef SelectionType = String;
typedef LastQueryMode = String;

const SelectionType selectionTypeText = "text";
const SelectionType selectionTypeFile = "file";

const LastQueryMode lastQueryModePreserve = "preserve";
const LastQueryMode lastQueryModeEmpty = "empty";

class Selection {
  SelectionType? type;

  // Only available when Type is SelectionTypeText
  String? text;

  // Only available when Type is SelectionTypeFile
  List<String>? filePaths;
}

class ChangedQuery {
  String? queryType;
  String? queryText;
  Selection? querySelection;
}

class QueryHistory {
  ChangedQuery? query;
  int? timestamp;
}
