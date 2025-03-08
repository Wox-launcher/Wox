class WoxBackup {
  late String id;
  late String name;
  late int timestamp;
  late String type;
  late String path;

  WoxBackup({
    required this.id,
    required this.name,
    required this.timestamp,
    required this.type,
    required this.path,
  });

  WoxBackup.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    timestamp = json['Timestamp'];
    type = json['Type'];
    path = json['Path'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Name'] = name;
    data['Timestamp'] = timestamp;
    data['Type'] = type;
    data['Path'] = path;
    return data;
  }
}
