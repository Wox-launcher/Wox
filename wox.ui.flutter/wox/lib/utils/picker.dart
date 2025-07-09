import 'package:file_picker/file_picker.dart';

class FileSelectorParams {
  late bool isDirectory;

  FileSelectorParams({required this.isDirectory});

  FileSelectorParams.fromJson(Map<String, dynamic> json) {
    isDirectory = json['IsDirectory'];
  }
}

class FileSelector {
  static Future<List<String>> pick(String traceId, FileSelectorParams params) async {
    if (params.isDirectory) {
      String? selectedDirectory = await FilePicker.platform.getDirectoryPath();
      if (selectedDirectory != null) {
        return [selectedDirectory];
      }
    }

    final result = await FilePicker.platform.pickFiles(
      type: FileType.any,
      allowMultiple: false,
    );

    if (result != null && result.files.isNotEmpty) {
      return result.files.map((e) => e.path ?? "").toList();
    }

    return [];
  }
}
