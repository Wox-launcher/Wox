import 'package:wox/entities/query.dart';

typedef PositionType = String;

const PositionType positionTypeMouseScreen = "MouseScreen";
const PositionType positionTypeLastLocation = "LastLocation";

class Position {
  PositionType? type;
  int? x;
  int? y;
}

class ShowAppParams {
  bool? selectAll;
  Position? position;
  List<QueryHistory>? queryHistories;
  LastQueryMode? lastQueryMode;
}
