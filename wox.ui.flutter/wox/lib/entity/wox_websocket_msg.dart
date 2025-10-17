import 'package:wox/enums/wox_msg_type_enum.dart';

class WoxWebsocketMsg {
  /// The unique identifier of the request. different for each request
  late String requestId;

  /// trace id between ui and wox, used for logging
  late String traceId;

  late String method;
  late WoxMsgType type;
  late dynamic data;
  late bool? success;
  late int sendTimestamp; // timestamp when message is sent (milliseconds since epoch)

  WoxWebsocketMsg({required this.requestId, required this.traceId, required this.method, required this.type, this.success = true, this.data, this.sendTimestamp = 0});

  WoxWebsocketMsg.fromJson(Map<String, dynamic> json) {
    requestId = json['RequestId'];
    traceId = json['TraceId'];
    method = json['Method'];
    type = json['Type'];
    data = json['Data'];
    success = json['Success'];
    sendTimestamp = json['SendTimestamp'] ?? 0;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> finalData = <String, dynamic>{};
    finalData['RequestId'] = requestId;
    finalData['TraceId'] = traceId;
    finalData['Method'] = method;
    finalData['Type'] = type;
    finalData['Success'] = success;
    finalData['Data'] = data;
    finalData['SendTimestamp'] = sendTimestamp;
    return finalData;
  }
}
