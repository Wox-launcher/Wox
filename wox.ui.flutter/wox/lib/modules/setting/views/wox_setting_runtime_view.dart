import 'dart:async';
import 'dart:io';
import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/picker.dart';

class WoxSettingRuntimeView extends WoxSettingBaseView {
  WoxSettingRuntimeView({super.key});

  String _runtimeDisplayName(String runtime) {
    switch (runtime.toUpperCase()) {
      case 'PYTHON':
        return controller.tr("ui_runtime_name_python");
      case 'NODEJS':
        return controller.tr("ui_runtime_name_nodejs");
      case 'SCRIPT':
        return controller.tr("ui_runtime_name_script");
      case 'GO':
        return controller.tr("ui_runtime_name_go");
      default:
        return runtime;
    }
  }

  Widget _buildRuntimeStatusCard(
      BuildContext context, WoxRuntimeStatus status) {
    final bool isRunning = status.isStarted;
    final Color accentColor = getThemeActiveBackgroundColor();
    final Color baseBackground = getThemeBackgroundColor();
    final bool isDarkTheme = baseBackground.computeLuminance() < 0.5;
    final Color panelColor = getThemePanelBackgroundColor();
    Color cardColor = panelColor.opacity < 1
        ? Color.alphaBlend(panelColor, baseBackground)
        : panelColor;
    cardColor = isDarkTheme ? cardColor.lighter(6) : cardColor.darker(4);
    final Color textColor = getThemeTextColor();
    final Color subTextColor = getThemeSubTextColor();
    final Color statusColor = isRunning ? accentColor : Colors.red;
    final Color outlineColor =
        getThemeDividerColor().withOpacity(isDarkTheme ? 0.45 : 0.25);

    final IconData statusIcon =
        isRunning ? FluentIcons.completed : FluentIcons.status_circle_error_x;
    final String stateLabel = isRunning
        ? controller.tr("ui_runtime_status_running")
        : controller.tr("ui_runtime_status_stopped");
    final String pluginCountLabel = controller
        .tr("ui_runtime_status_plugin_count")
        .replaceAll("{count}", status.loadedPluginCount.toString());

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: cardColor,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: outlineColor),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(isDarkTheme ? 0.24 : 0.08),
            blurRadius: 18,
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: statusColor.withOpacity(isDarkTheme ? 0.32 : 0.18),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Icon(statusIcon, color: statusColor, size: 20),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _runtimeDisplayName(status.runtime),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: textColor,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      stateLabel,
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        color: statusColor,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text(
            pluginCountLabel,
            style: TextStyle(color: subTextColor),
          ),
        ],
      ),
    );
  }

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
        pythonValidationMessage.value =
            '✗ ${controller.tr("ui_runtime_validation_failed")}';
      }
    } catch (e) {
      pythonValidationMessage.value =
          '✗ ${controller.tr("ui_runtime_validation_error")}: ${e.toString()}';
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
        nodejsValidationMessage.value =
            '✗ ${controller.tr("ui_runtime_validation_failed")}';
      }
    } catch (e) {
      nodejsValidationMessage.value =
          '✗ ${controller.tr("ui_runtime_validation_error")}: ${e.toString()}';
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
    pythonController = TextEditingController(
        text: controller.woxSetting.value.customPythonPath);
    nodejsController = TextEditingController(
        text: controller.woxSetting.value.customNodejsPath);

    // Initial validation
    if (pythonController.text.isNotEmpty) {
      validatePythonPath(pythonController.text);
    }
    if (nodejsController.text.isNotEmpty) {
      validateNodejsPath(nodejsController.text);
    }
    return Obx(() {
      final statuses = controller.runtimeStatuses;
      final bool isLoadingStatuses = controller.isRuntimeStatusLoading.value;
      final String runtimeStatusError = controller.runtimeStatusError.value;
      final List<WoxRuntimeStatus> visibleStatuses = statuses
          .where((status) => status.runtime.toUpperCase() != 'SCRIPT')
          .toList();

      return form(children: [
        formField(
          label: controller.tr("ui_runtime_status"),
          tips: null,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Text(
                    controller.tr("ui_runtime_status_overview"),
                    style: TextStyle(
                      fontWeight: FontWeight.w600,
                      color: getThemeTextColor(),
                    ),
                  ),
                  const Spacer(),
                  if (isLoadingStatuses)
                    const SizedBox(
                      width: 16,
                      height: 16,
                      child: ProgressRing(),
                    ),
                ],
              ),
              const SizedBox(height: 12),
              if (runtimeStatusError.isNotEmpty) ...[
                Text(
                  runtimeStatusError,
                  style: TextStyle(color: Colors.red, fontSize: 12),
                ),
                const SizedBox(height: 4),
              ],
              if (!isLoadingStatuses &&
                  runtimeStatusError.isEmpty &&
                  visibleStatuses.isEmpty)
                Text(
                  controller.tr("ui_runtime_status_empty"),
                  style: TextStyle(color: Colors.grey[120]),
                ),
              if (visibleStatuses.isNotEmpty)
                LayoutBuilder(
                  builder: (context, constraints) {
                    final double availableWidth = constraints.maxWidth.isFinite
                        ? constraints.maxWidth
                        : 960;
                    final double spacing = 12;
                    final int columnCount = availableWidth >= 640 ? 2 : 1;
                    final double cardWidth = columnCount == 1
                        ? availableWidth
                        : (availableWidth - spacing) / columnCount;

                    return Wrap(
                      spacing: spacing,
                      runSpacing: spacing,
                      children: visibleStatuses
                          .map(
                            (status) => SizedBox(
                              width:
                                  columnCount == 1 ? availableWidth : cardWidth,
                              child: _buildRuntimeStatusCard(context, status),
                            ),
                          )
                          .toList(),
                    );
                  },
                ),
            ],
          ),
        ),
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
                      placeholder:
                          controller.tr("ui_runtime_python_path_placeholder"),
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
                      const SizedBox(
                          width: 16, height: 16, child: ProgressRing()),
                      const SizedBox(width: 8),
                      Text(controller.tr("ui_runtime_validating")),
                    ],
                  );
                } else if (pythonValidationMessage.value.isNotEmpty) {
                  return Text(
                    pythonValidationMessage.value,
                    style: TextStyle(
                      color: pythonValidationMessage.value.startsWith('✓')
                          ? Colors.green
                          : Colors.red,
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
                      placeholder:
                          controller.tr("ui_runtime_nodejs_path_placeholder"),
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
                      const SizedBox(
                          width: 16, height: 16, child: ProgressRing()),
                      const SizedBox(width: 8),
                      Text(controller.tr("ui_runtime_validating")),
                    ],
                  );
                } else if (nodejsValidationMessage.value.isNotEmpty) {
                  return Text(
                    nodejsValidationMessage.value,
                    style: TextStyle(
                      color: nodejsValidationMessage.value.startsWith('✓')
                          ? Colors.green
                          : Colors.red,
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
