import 'package:wox/entity/wox_image.dart';

class AIModel {
  late String name;
  late String provider;

  AIModel({required this.name, required this.provider});

  AIModel.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    provider = json['Provider'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Provider'] = provider;
    return data;
  }
}

class AIMCPTool {
  late String name;
  late String description;

  AIMCPTool({required this.name, required this.description});

  AIMCPTool.fromJson(Map<String, dynamic> json) {
    name = json['Name'] ?? "";
    description = json['Description'] ?? "";
  }
}

class ChatSelectItem {
  final String id;
  final String name;
  final WoxImage icon;
  final bool isCategory;
  final List<ChatSelectItem> children;
  Function(String traceId)? onExecute;

  ChatSelectItem({
    required this.id,
    required this.name,
    required this.icon,
    required this.isCategory,
    required this.children,
    this.onExecute,
  });
}
