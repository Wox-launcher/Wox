class WoxBackup {
  late String id;
  late String name;
  late int timestamp;
  late String type;

  WoxBackup({
    required this.id,
    required this.name,
    required this.timestamp,
    required this.type,
  });

  WoxBackup.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    timestamp = json['Timestamp'];
    type = json['Type'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Name'] = name;
    data['Timestamp'] = timestamp;
    data['Type'] = type;
    return data;
  }
}
