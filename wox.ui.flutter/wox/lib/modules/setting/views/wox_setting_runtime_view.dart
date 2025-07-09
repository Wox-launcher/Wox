import 'dart:async';
import 'dart:io';
import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/picker.dart';

class WoxSettingRuntimeView extends WoxSettingBaseView {
  WoxSettingRuntimeView({super.key});

  // Validation states
  final RxString pythonValidationMessage = ''.obs;
  final RxString nodejsValidationMessage = ''.obs;
  final RxBool isPythonValidating = false.obs;
  final RxBool isNodejsValidating = false.obs;

  // Text controllers for immediate updates
  late final TextEditingController pythonController;
  late final TextEditingController nodejsController;

  // Debounce timers for validation
  Timer? _pythonValidationTimer;
  Timer? _nodejsValidationTimer;

  // Validation methods
  Future<void> validatePythonPath(String path) async {
    if (path.isEmpty) {
      pythonValidationMessage.value = '';
      return;
    }

    isPythonValidating.value = true;
    try {
      final result = await Process.run(path, ['--version']);
      if (result.exitCode == 0) {
        final version = result.stdout.toString().trim();
        pythonValidationMessage.value = '✓ $version';
      } else {
        pythonValidationMessage.value = '✗ ${controller.tr("ui_runtime_validation_failed")}';
      }
    } catch (e) {
      pythonValidationMessage.value = '✗ ${controller.tr("ui_runtime_validation_error")}: ${e.toString()}';
    } finally {
      isPythonValidating.value = false;
    }
  }

  Future<void> validateNodejsPath(String path) async {
    if (path.isEmpty) {
      nodejsValidationMessage.value = '';
      return;
    }

    isNodejsValidating.value = true;
    try {
      final result = await Process.run(path, ['-v']);
      if (result.exitCode == 0) {
        final version = result.stdout.toString().trim();
        nodejsValidationMessage.value = '✓ $version';
      } else {
        nodejsValidationMessage.value = '✗ ${controller.tr("ui_runtime_validation_failed")}';
      }
    } catch (e) {
      nodejsValidationMessage.value = '✗ ${controller.tr("ui_runtime_validation_error")}: ${e.toString()}';
    } finally {
      isNodejsValidating.value = false;
    }
  }

  void updatePythonPath(String value) {
    controller.updateConfig("CustomPythonPath", value);

    // Cancel previous timer
    _pythonValidationTimer?.cancel();

    // Start new timer for debounced validation
    _pythonValidationTimer = Timer(const Duration(milliseconds: 500), () {
      validatePythonPath(value);
    });
  }

  void updateNodejsPath(String value) {
    controller.updateConfig("CustomNodejsPath", value);

    // Cancel previous timer
    _nodejsValidationTimer?.cancel();

    // Start new timer for debounced validation
    _nodejsValidationTimer = Timer(const Duration(milliseconds: 500), () {
      validateNodejsPath(value);
    });
  }

  void dispose() {
    _pythonValidationTimer?.cancel();
    _nodejsValidationTimer?.cancel();
    pythonController.dispose();
    nodejsController.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Initialize controllers with current values
    pythonController = TextEditingController(text: controller.woxSetting.value.customPythonPath);
    nodejsController = TextEditingController(text: controller.woxSetting.value.customNodejsPath);

    // Initial validation
    if (pythonController.text.isNotEmpty) {
      validatePythonPath(pythonController.text);
    }
    if (nodejsController.text.isNotEmpty) {
      validateNodejsPath(nodejsController.text);
    }
    return Obx(() {
      return form(children: [
        formField(
          label: controller.tr("ui_runtime_python_path"),
          tips: controller.tr("ui_runtime_python_path_tips"),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: TextBox(
                      controller: pythonController,
                      placeholder: controller.tr("ui_runtime_python_path_placeholder"),
                      onChanged: (value) {
                        updatePythonPath(value);
                      },
                    ),
                  ),
                  const SizedBox(width: 10),
                  Button(
                    child: Text(controller.tr("ui_runtime_browse")),
                    onPressed: () async {
                      final result = await FileSelector.pick(
                        const UuidV4().generate(),
                        FileSelectorParams(isDirectory: false),
                      );
                      if (result.isNotEmpty) {
                        pythonController.text = result.first;
                        updatePythonPath(result.first);
                      }
                    },
                  ),
                  const SizedBox(width: 10),
                  Button(
                    child: Text(controller.tr("ui_runtime_clear")),
                    onPressed: () {
                      pythonController.clear();
                      updatePythonPath("");
                    },
                  ),
                ],
              ),
              const SizedBox(height: 5),
              Obx(() {
                if (isPythonValidating.value) {
                  return Row(
                    children: [
                      const SizedBox(width: 16, height: 16, child: ProgressRing()),
                      const SizedBox(width: 8),
                      Text(controller.tr("ui_runtime_validating")),
                    ],
                  );
                } else if (pythonValidationMessage.value.isNotEmpty) {
                  return Text(
                    pythonValidationMessage.value,
                    style: TextStyle(
                      color: pythonValidationMessage.value.startsWith('✓') ? Colors.green : Colors.red,
                      fontSize: 12,
                    ),
                  );
                }
                return const SizedBox.shrink();
              }),
            ],
          ),
        ),
        formField(
          label: controller.tr("ui_runtime_nodejs_path"),
          tips: controller.tr("ui_runtime_nodejs_path_tips"),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: TextBox(
                      controller: nodejsController,
                      placeholder: controller.tr("ui_runtime_nodejs_path_placeholder"),
                      onChanged: (value) {
                        updateNodejsPath(value);
                      },
                    ),
                  ),
                  const SizedBox(width: 10),
                  Button(
                    child: Text(controller.tr("ui_runtime_browse")),
                    onPressed: () async {
                      final result = await FileSelector.pick(
                        const UuidV4().generate(),
                        FileSelectorParams(isDirectory: false),
                      );
                      if (result.isNotEmpty) {
                        nodejsController.text = result.first;
                        updateNodejsPath(result.first);
                      }
                    },
                  ),
                  const SizedBox(width: 10),
                  Button(
                    child: Text(controller.tr("ui_runtime_clear")),
                    onPressed: () {
                      nodejsController.clear();
                      updateNodejsPath("");
                    },
                  ),
                ],
              ),
              const SizedBox(height: 5),
              Obx(() {
                if (isNodejsValidating.value) {
                  return Row(
                    children: [
                      const SizedBox(width: 16, height: 16, child: ProgressRing()),
                      const SizedBox(width: 8),
                      Text(controller.tr("ui_runtime_validating")),
                    ],
                  );
                } else if (nodejsValidationMessage.value.isNotEmpty) {
                  return Text(
                    nodejsValidationMessage.value,
                    style: TextStyle(
                      color: nodejsValidationMessage.value.startsWith('✓') ? Colors.green : Colors.red,
                      fontSize: 12,
                    ),
                  );
                }
                return const SizedBox.shrink();
              }),
            ],
          ),
        ),
      ]);
    });
  }
}
