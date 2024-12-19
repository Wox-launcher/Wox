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

    return [];
  }
}
