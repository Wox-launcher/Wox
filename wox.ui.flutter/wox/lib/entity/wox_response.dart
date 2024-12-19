class WoxResponse {
  bool? success;
  String? message;
  dynamic data;

  WoxResponse({this.success, this.message, this.data});

  WoxResponse.fromJson(Map<String, dynamic> json) {
    success = json['Success'];
    message = json['Message'];
    data = json['Data'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Success'] = success;
    data['Message'] = message;
    if (this.data != null) {
      data['Data'] = this.data!.toJson();
    }
    return data;
  }
}
