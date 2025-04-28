import 'package:wox/entity/wox_image.dart';

class DoctorCheckResult {
  final String name;
  final String type;
  final bool passed;
  final String description;
  final String actionName;
  final Map<String, dynamic>? preview;

  DoctorCheckResult({
    required this.name,
    required this.type,
    required this.passed,
    required this.description,
    required this.actionName,
    this.preview,
  });

  factory DoctorCheckResult.fromJson(Map<String, dynamic> json) {
    return DoctorCheckResult(
      name: json['Name'] ?? '',
      type: json['Type'] ?? '',
      passed: json['Passed'] ?? false,
      description: json['Description'] ?? '',
      actionName: json['ActionName'] ?? '',
      preview: json['Preview'],
    );
  }

  bool get isVersionIssue => type.toLowerCase() == 'update' && !passed;
  bool get isPermissionIssue => type.toLowerCase() == 'accessibility' && !passed;
}

class DoctorCheckInfo {
  final List<DoctorCheckResult> results;
  final bool allPassed;
  final WoxImage icon;
  final String message;

  DoctorCheckInfo({
    required this.results,
    required this.allPassed,
    required this.icon,
    required this.message,
  });

  factory DoctorCheckInfo.empty() {
    return DoctorCheckInfo(
      results: [],
      allPassed: true,
      icon: WoxImage.empty(),
      message: '',
    );
  }
}
