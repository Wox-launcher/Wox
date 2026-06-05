import 'package:file_picker/file_picker.dart';

class FileSelectorParams {
  late bool isDirectory;
  List<String>? allowedExtensions;
  bool allowMultiple = false;

  FileSelectorParams({
    required this.isDirectory,
    this.allowedExtensions,
    this.allowMultiple = false,
  });

  FileSelectorParams.fromJson(Map<String, dynamic> json) {
    isDirectory = json['IsDirectory'] ?? true;
    if (json['AllowedExtensions'] != null) {
      allowedExtensions = List<String>.from(json['AllowedExtensions']);
    }
    allowMultiple = json['AllowMultiple'] ?? false;
  }
}

class FileSelector {
  static Future<List<String>> pick(String traceId, FileSelectorParams params) async {
    if (params.isDirectory) {
      String? selectedDirectory = await FilePicker.platform.getDirectoryPath();
      if (selectedDirectory != null) {
        return [selectedDirectory];
      }
      return [];
    }

    final hasExtensions = params.allowedExtensions != null && params.allowedExtensions!.isNotEmpty;
    final result = await FilePicker.platform.pickFiles(
      type: hasExtensions ? FileType.custom : FileType.any,
      allowedExtensions: hasExtensions ? params.allowedExtensions : null,
      allowMultiple: params.allowMultiple,
    );

    if (result != null && result.files.isNotEmpty) {
      return result.files.map((e) => e.path ?? "").toList();
    }

    return [];
  }
}
