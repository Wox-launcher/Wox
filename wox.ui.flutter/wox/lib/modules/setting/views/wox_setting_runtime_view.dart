import 'dart:async';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_panel.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/picker.dart';

// ignore: must_be_immutable
class WoxSettingRuntimeView extends WoxSettingBaseView {
  WoxSettingRuntimeView({super.key});
  static const double _runtimeStatusDetailAreaHeight = 40;
  static const double _runtimeStatusDetailBottomSpacing = 14;

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

  String _runtimeIcon(String runtime) {
    switch (runtime.toUpperCase()) {
      case 'PYTHON':
        return PYTHON_ICON;
      case 'NODEJS':
        return NODEJS_ICON;
      case 'SCRIPT':
        return SCRIPT_ICON;
      default:
        return SCRIPT_ICON;
    }
  }

  String _runtimeStatusLabel(WoxRuntimeStatus status) {
    switch (status.statusCode) {
      case 'running':
        return controller.tr("ui_runtime_status_running");
      case 'executable_missing':
        return controller.tr("ui_runtime_status_executable_missing");
      case 'unsupported_version':
        return controller.tr("ui_runtime_status_unsupported_version");
      case 'start_failed':
        return controller.tr("ui_runtime_status_start_failed");
      default:
        return controller.tr("ui_runtime_status_stopped");
    }
  }

  String _runtimeStatusDetail(WoxRuntimeStatus status) {
    switch (status.statusCode) {
      case 'executable_missing':
        return controller.tr("ui_runtime_status_executable_missing_detail").replaceAll("{runtime}", _runtimeDisplayName(status.runtime));
      case 'unsupported_version':
        return controller.tr("ui_runtime_status_unsupported_version_detail").replaceAll("{runtime}", _runtimeDisplayName(status.runtime));
      case 'start_failed':
        return status.lastStartError.isNotEmpty ? status.lastStartError : controller.tr("ui_runtime_status_start_failed_detail");
      default:
        return status.executablePath;
    }
  }

  Color _runtimeStatusColor(WoxRuntimeStatus status) {
    switch (status.statusCode) {
      case 'running':
        return Colors.green;
      case 'executable_missing':
      case 'unsupported_version':
      case 'start_failed':
        return Colors.red;
      default:
        return Colors.orange;
    }
  }

  double _runtimeStatusCardHeight(WoxRuntimeStatus status) {
    // Layout fix: normal runtimes can show a title, status pill, two-line path,
    // and plugin count. The previous compact height only worked for shorter
    // paths and overflowed under Chinese text metrics, so keep a small buffer
    // instead of relying on exact pixel-fit math.
    return status.isActionableFailure ? 196 : 140;
  }

  Widget _buildRuntimeStatusCard(BuildContext context, WoxRuntimeStatus status, double cardHeight) {
    final bool isDarkTheme = isThemeDark();
    final Color textColor = getThemeTextColor();
    final Color subTextColor = getThemeSubTextColor();
    final Color statusColor = _runtimeStatusColor(status);
    final Color iconBackgroundColor = getThemeTextColor().withValues(alpha: isDarkTheme ? 0.10 : 0.05);

    final String stateLabel = _runtimeStatusLabel(status);
    final String statusDetail = _runtimeStatusDetail(status);
    final String pluginCountLabel = controller.tr("ui_runtime_status_plugin_count").replaceAll("{count}", status.loadedPluginCount.toString());
    final String hostVersionLabel = status.hostVersion.isNotEmpty && !status.hostVersion.toLowerCase().startsWith('v') ? 'v${status.hostVersion}' : status.hostVersion;
    final WoxImage runtimeIcon = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: _runtimeIcon(status.runtime));
    final bool isRestarting = controller.restartingRuntime.value == status.runtime.toUpperCase();

    return WoxPanel(
      padding: const EdgeInsets.all(14),
      child: SizedBox(
        height: cardHeight,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 34,
                  height: 34,
                  decoration: BoxDecoration(
                    // The leading mark identifies the runtime itself; the shared panel provides the
                    // card surface while the status pill remains the only running/stopped signal.
                    color: iconBackgroundColor,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Center(child: WoxImageView(woxImage: runtimeIcon, width: 22, height: 22)),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(child: Text(_runtimeDisplayName(status.runtime), style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: textColor))),
                          if (hostVersionLabel.isNotEmpty) Text(hostVersionLabel, style: TextStyle(color: subTextColor, fontSize: 12)),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                            decoration: BoxDecoration(color: statusColor.withValues(alpha: isDarkTheme ? 0.22 : 0.12), borderRadius: BorderRadius.circular(999)),
                            child: Text(stateLabel, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: statusColor)),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Padding(
              padding: const EdgeInsets.only(left: 46),
              child: SizedBox(
                height: _runtimeStatusDetailAreaHeight,
                // Layout fix: the detail area is reserved even when a runtime has
                // no executable path, such as Script. This keeps plugin counts
                // aligned without duplicating the plugin count as fake detail text.
                child:
                    statusDetail.isEmpty
                        ? const SizedBox.shrink()
                        : Text(statusDetail, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTextColor, fontSize: 12)),
              ),
            ),
            const SizedBox(height: _runtimeStatusDetailBottomSpacing),
            Padding(padding: const EdgeInsets.only(left: 46), child: Text(pluginCountLabel, style: TextStyle(color: subTextColor, fontSize: 13))),
            const Spacer(),
            if (status.isActionableFailure) ...[
              const SizedBox(height: 10),
              Padding(
                padding: const EdgeInsets.only(left: 46),
                child: Row(
                  children: [
                    if (status.installUrl.isNotEmpty && (status.statusCode == 'executable_missing' || status.statusCode == 'unsupported_version')) ...[
                      WoxButton.secondary(
                        text: controller
                            .tr(status.statusCode == 'unsupported_version' ? "ui_runtime_upgrade_runtime" : "ui_runtime_install_runtime")
                            .replaceAll("{runtime}", _runtimeDisplayName(status.runtime)),
                        icon: Icon(Icons.open_in_new, size: 14, color: getThemeTextColor()),
                        onPressed: () {
                          controller.openRuntimeInstallUrl(status);
                        },
                      ),
                      const SizedBox(width: 8),
                    ],
                    if (status.canRestart)
                      WoxButton.secondary(
                        text: isRestarting ? controller.tr("ui_runtime_restarting_host") : controller.tr("ui_runtime_restart_host"),
                        icon: isRestarting ? WoxLoadingIndicator(size: 14, color: getThemeActionItemActiveColor()) : Icon(Icons.restart_alt, size: 14, color: getThemeTextColor()),
                        onPressed:
                            isRestarting
                                ? null
                                : () {
                                  controller.restartRuntime(status);
                                },
                      ),
                  ],
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildRuntimeStatusCards(List<WoxRuntimeStatus> visibleStatuses) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final double availableWidth = constraints.maxWidth.isFinite ? constraints.maxWidth : GENERAL_SETTING_FORM_WIDTH;
        final double spacing = 12;
        final int columnCount =
            availableWidth >= 860
                ? 3
                : availableWidth >= 560
                ? 2
                : 1;
        final double cardWidth = columnCount == 1 ? availableWidth : (availableWidth - spacing * (columnCount - 1)) / columnCount;

        final rows = <Widget>[];
        for (var start = 0; start < visibleStatuses.length; start += columnCount) {
          final rowStatuses = visibleStatuses.skip(start).take(columnCount).toList();
          final rowHeight = rowStatuses.map(_runtimeStatusCardHeight).reduce((a, b) => a > b ? a : b);

          // Layout fix: when one runtime in a row expands to show recovery actions,
          // every card in that row uses the same height. The old per-card height
          // made healthy runtimes visually float above the failed runtime and broke
          // scan alignment across the status summary.
          rows.add(
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (var index = 0; index < rowStatuses.length; index++) ...[
                  SizedBox(width: cardWidth, child: _buildRuntimeStatusCard(context, rowStatuses[index], rowHeight)),
                  if (index < rowStatuses.length - 1) SizedBox(width: spacing),
                ],
              ],
            ),
          );
        }

        // Runtime status is a summary, not a normal label/control pair, so use
        // the full page width and keep runtimes visually grouped by row.
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var index = 0; index < rows.length; index++) ...[rows[index], if (index < rows.length - 1) SizedBox(height: spacing)],
          ],
        );
      },
    );
  }

  // Validation states
  final RxString pythonValidationMessage = ''.obs;
  final RxString nodejsValidationMessage = ''.obs;
  final RxBool isPythonValidating = false.obs;
  final RxBool isNodejsValidating = false.obs;

  // Text controllers for immediate updates
  TextEditingController? pythonController;
  TextEditingController? nodejsController;

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
    pythonController?.dispose();
    nodejsController?.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Initialize controllers with current values only if not already initialized
    pythonController ??= TextEditingController(text: controller.woxSetting.value.customPythonPath);
    nodejsController ??= TextEditingController(text: controller.woxSetting.value.customNodejsPath);

    // Initial validation
    if (pythonController!.text.isNotEmpty) {
      validatePythonPath(pythonController!.text);
    }
    if (nodejsController!.text.isNotEmpty) {
      validateNodejsPath(nodejsController!.text);
    }
    return Obx(() {
      final statuses = controller.runtimeStatuses;
      final bool isLoadingStatuses = controller.isRuntimeStatusLoading.value;
      final String runtimeStatusError = controller.runtimeStatusError.value;
      // Bug fix: this page is a runtime inventory, not an installed-plugin list.
      // The previous SCRIPT plugin-count filter hid the built-in script runtime when
      // users had not installed any script plugins, so keep every backend runtime here.
      final List<WoxRuntimeStatus> visibleStatuses = statuses.toList();

      return form(
        title: controller.tr("ui_runtime_settings"),
        description: controller.tr("ui_runtime_settings_description"),
        children: [
          formField(
            label: controller.tr("ui_runtime_status"),
            fullWidth: true,
            tips: null,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [if (isLoadingStatuses) const WoxLoadingIndicator(size: 16)]),
                const SizedBox(height: 12),
                if (runtimeStatusError.isNotEmpty) ...[Text(runtimeStatusError, style: TextStyle(color: Colors.red, fontSize: 12)), const SizedBox(height: 4)],
                if (!isLoadingStatuses && runtimeStatusError.isEmpty && visibleStatuses.isEmpty)
                  Text(controller.tr("ui_runtime_status_empty"), style: TextStyle(color: Colors.grey[120])),
                if (visibleStatuses.isNotEmpty) _buildRuntimeStatusCards(visibleStatuses),
              ],
            ),
          ),
          formSection(
            title: controller.tr("ui_runtime_executable_paths"),
            children: [
              formField(
                label: controller.tr("ui_runtime_python_path"),
                labelWidth: GENERAL_SETTING_LABEL_WIDTH,
                tips: controller.tr("ui_runtime_python_path_tips"),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: WoxTextField(
                            controller: pythonController!,
                            hintText: controller.tr("ui_runtime_python_path_placeholder"),
                            onChanged: (value) {
                              updatePythonPath(value);
                            },
                          ),
                        ),
                        const SizedBox(width: 10),
                        WoxButton.primary(
                          text: controller.tr("ui_runtime_browse"),
                          onPressed: () async {
                            final result = await FileSelector.pick(const UuidV4().generate(), FileSelectorParams(isDirectory: false));
                            if (result.isNotEmpty) {
                              pythonController!.text = result.first;
                              updatePythonPath(result.first);
                            }
                          },
                        ),
                        const SizedBox(width: 10),
                        WoxButton.secondary(
                          text: controller.tr("ui_runtime_clear"),
                          onPressed: () {
                            pythonController!.clear();
                            updatePythonPath("");
                          },
                        ),
                      ],
                    ),
                    const SizedBox(height: 5),
                    Obx(() {
                      if (isPythonValidating.value) {
                        return Row(children: [const WoxLoadingIndicator(size: 16), const SizedBox(width: 8), Text(controller.tr("ui_runtime_validating"))]);
                      } else if (pythonValidationMessage.value.isNotEmpty) {
                        return Text(
                          pythonValidationMessage.value,
                          style: TextStyle(color: pythonValidationMessage.value.startsWith('✓') ? Colors.green : Colors.red, fontSize: 12),
                        );
                      }
                      return const SizedBox.shrink();
                    }),
                  ],
                ),
              ),
              formField(
                label: controller.tr("ui_runtime_nodejs_path"),
                labelWidth: GENERAL_SETTING_LABEL_WIDTH,
                tips: controller.tr("ui_runtime_nodejs_path_tips"),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: WoxTextField(
                            controller: nodejsController!,
                            hintText: controller.tr("ui_runtime_nodejs_path_placeholder"),
                            onChanged: (value) {
                              updateNodejsPath(value);
                            },
                          ),
                        ),
                        const SizedBox(width: 10),
                        WoxButton.primary(
                          text: controller.tr("ui_runtime_browse"),
                          onPressed: () async {
                            final result = await FileSelector.pick(const UuidV4().generate(), FileSelectorParams(isDirectory: false));
                            if (result.isNotEmpty) {
                              nodejsController!.text = result.first;
                              updateNodejsPath(result.first);
                            }
                          },
                        ),
                        const SizedBox(width: 10),
                        WoxButton.secondary(
                          text: controller.tr("ui_runtime_clear"),
                          onPressed: () {
                            nodejsController!.clear();
                            updateNodejsPath("");
                          },
                        ),
                      ],
                    ),
                    const SizedBox(height: 5),
                    Obx(() {
                      if (isNodejsValidating.value) {
                        return Row(children: [const WoxLoadingIndicator(size: 16), const SizedBox(width: 8), Text(controller.tr("ui_runtime_validating"))]);
                      } else if (nodejsValidationMessage.value.isNotEmpty) {
                        return Text(
                          nodejsValidationMessage.value,
                          style: TextStyle(color: nodejsValidationMessage.value.startsWith('✓') ? Colors.green : Colors.red, fontSize: 12),
                        );
                      }
                      return const SizedBox.shrink();
                    }),
                  ],
                ),
              ),
            ],
          ),
        ],
      );
    });
  }
}
