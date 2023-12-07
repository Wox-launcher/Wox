import 'package:wox/enums/wox_msg_type_enum.dart';

class WoxWebsocketMsg {
  late String id;
  late String method;
  late WoxMsgType type;
  late dynamic data;
  late bool? success;

  WoxWebsocketMsg({required this.id, required this.method, required this.type, this.success = true, this.data});

  WoxWebsocketMsg.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    method = json['Method'];
    type = json['Type'];
    data = json['Data'];
    success = json['Success'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> finalData = <String, dynamic>{};
    finalData['Id'] = id;
    finalData['Method'] = method;
    finalData['Type'] = type;
    finalData['Success'] = success;
    finalData['Data'] = data;
    return finalData;
  }
}
