class WebsocketMsg {
  String? id;
  String? type;
  String? method;
  bool? success;
  dynamic data;

  WebsocketMsg({this.id, this.type, this.method, this.success, this.data});

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
