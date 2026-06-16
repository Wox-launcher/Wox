import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';

import 'smoke_test_helper.dart';

const String _testNodeTemplatePluginIdEnv = 'WOX_TEST_NODE_TEMPLATE_PLUGIN_ID';
const String _testNodeTemplatePluginNameEnv = 'WOX_TEST_NODE_TEMPLATE_PLUGIN_NAME';
const String _testNodeTemplatePluginTriggerKeywordEnv = 'WOX_TEST_NODE_TEMPLATE_PLUGIN_TRIGGER_KEYWORD';
const String _testPythonTemplatePluginIdEnv = 'WOX_TEST_PYTHON_TEMPLATE_PLUGIN_ID';
const String _testPythonTemplatePluginNameEnv = 'WOX_TEST_PYTHON_TEMPLATE_PLUGIN_NAME';
const String _testPythonTemplatePluginTriggerKeywordEnv = 'WOX_TEST_PYTHON_TEMPLATE_PLUGIN_TRIGGER_KEYWORD';
const String _customCommandsPluginId = '09785021-7f9e-4482-903a-51c90a585a7d';
const String _customCommandsPluginName = 'Custom Commands';

class _SmokeTemplatePluginConfig {
  const _SmokeTemplatePluginConfig({required this.id, required this.name, required this.triggerKeyword, required this.runtime});

  final String id;
  final String name;
  final String triggerKeyword;
  final String runtime;

  static _SmokeTemplatePluginConfig? fromEnvironment({required String runtime, required String idEnvKey, required String nameEnvKey, required String triggerKeywordEnvKey}) {
    final id = Platform.environment[idEnvKey]?.trim() ?? '';
    final name = Platform.environment[nameEnvKey]?.trim() ?? '';
    final triggerKeyword = Platform.environment[triggerKeywordEnvKey]?.trim() ?? '';
    if (id.isEmpty || name.isEmpty || triggerKeyword.isEmpty) {
      return null;
    }

    return _SmokeTemplatePluginConfig(id: id, name: name, triggerKeyword: triggerKeyword, runtime: runtime);
  }
}

class _RuntimeMissingPathSmokeCase {
  const _RuntimeMissingPathSmokeCase({required this.runtime, required this.settingKey});

  final String runtime;
  final String settingKey;
}

void registerLauncherPluginSmokeTests() {
  group('T4: Template Plugin Smoke Tests', () {
    testWidgets('T4-01: Packaged Nodejs template plugin loads and basic behaviors work', (tester) async {
      final config = _SmokeTemplatePluginConfig.fromEnvironment(
        runtime: 'nodejs',
        idEnvKey: _testNodeTemplatePluginIdEnv,
        nameEnvKey: _testNodeTemplatePluginNameEnv,
        triggerKeywordEnvKey: _testNodeTemplatePluginTriggerKeywordEnv,
      );
      if (config == null) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final installedPlugins = await WoxApi.instance.findInstalledPlugins(const UuidV4().generate());
      final installedPlugin = installedPlugins.where((plugin) => plugin.id == config.id).toList();
      expect(installedPlugin, hasLength(1));
      expect(installedPlugin.first.name, equals(config.name));
      expect(installedPlugin.first.runtime, equals(config.runtime));
      expect(installedPlugin.first.isInstalled, isTrue);
      expect(installedPlugin.first.triggerKeywords, contains(config.triggerKeyword));

      const search = 'smoke-check';

      await queryAndWaitForResults(tester, controller, '${config.triggerKeyword} $search');

      expect(controller.activeResultViewController.items, isNotEmpty);
      final result = controller.activeResultViewController.activeItem.data;

      expect(result.title, equals('Hello World $search'));
      expect(result.subTitle, equals('This is a subtitle'));
      expect(result.preview.previewType, equals(WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TEXT.code));
      expect(result.preview.previewData, equals('This is a preview'));
      final property1Tag = result.preview.previewTags.firstWhere((tag) => tag.tooltip == 'Property1');
      final property2Tag = result.preview.previewTags.firstWhere((tag) => tag.tooltip == 'Property2');
      expect(property1Tag.label, equals('Hello World'));
      expect(property2Tag.label, equals('This is a property'));
      // Dev smoke builds now append batch/latency/score tails automatically, so
      // assert only on the template plugin's own tail payload here.
      final businessTails = getSmokeBusinessTails(result);
      expect(businessTails, hasLength(1));
      expect(businessTails.first.text, equals('This is a tail'));
      final openActions = result.actions.where((action) => action.name == 'Open').toList();
      expect(openActions, isNotEmpty);
      expect(openActions.first.contextData['search'], equals(search));

      controller.executeDefaultAction(const UuidV4().generate());
      await waitForWindowVisibility(tester, false);
    });

    testWidgets('T4-02: Packaged Python template plugin loads and basic behaviors work', (tester) async {
      final config = _SmokeTemplatePluginConfig.fromEnvironment(
        runtime: 'python',
        idEnvKey: _testPythonTemplatePluginIdEnv,
        nameEnvKey: _testPythonTemplatePluginNameEnv,
        triggerKeywordEnvKey: _testPythonTemplatePluginTriggerKeywordEnv,
      );
      if (config == null) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final installedPlugins = await WoxApi.instance.findInstalledPlugins(const UuidV4().generate());
      final installedPlugin = installedPlugins.where((plugin) => plugin.id == config.id).toList();
      expect(installedPlugin, hasLength(1));
      expect(installedPlugin.first.name, equals(config.name));
      expect(installedPlugin.first.runtime, equals(config.runtime));
      expect(installedPlugin.first.isInstalled, isTrue);
      expect(installedPlugin.first.triggerKeywords, contains(config.triggerKeyword));

      const search = 'SmOkE-ChEcK';

      await queryAndWaitForResults(tester, controller, '${config.triggerKeyword} $search');

      expect(controller.activeResultViewController.items, isNotEmpty);
      final result = controller.activeResultViewController.activeItem.data;

      expect(result.title, equals('you typed smoke-check'));
      expect(result.subTitle, equals('this is subsitle'));
      // The Python template intentionally returns no plugin-defined tails. Keep
      // ignoring the dev-only debug tails so this smoke test still validates the
      // packaged template payload instead of the environment-specific annotations.
      expect(getSmokeBusinessTails(result), isEmpty);

      final myActions = result.actions.where((action) => action.name == 'My Action').toList();
      expect(myActions, isNotEmpty);
      expect(myActions.first.contextData['search_term'], equals('smoke-check'));
      expect(myActions.first.preventHideAfterAction, isTrue);

      controller.executeDefaultAction(const UuidV4().generate());
      await waitForWindowVisibility(tester, true);
    });

    testWidgets('T4-03: Runtime status distinguishes missing Node.js and Python executables', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      const cases = <_RuntimeMissingPathSmokeCase>[
        _RuntimeMissingPathSmokeCase(runtime: 'NODEJS', settingKey: 'CustomNodejsPath'),
        _RuntimeMissingPathSmokeCase(runtime: 'PYTHON', settingKey: 'CustomPythonPath'),
      ];

      for (final testCase in cases) {
        final missingPath = _missingExecutablePath(testCase.runtime);
        try {
          // Smoke coverage: force the same user-visible failure that happens
          // when a plugin install needs a missing runtime executable. The
          // backend must report executable_missing instead of a generic stopped
          // host so the UI can render install/path guidance.
          final canPersistMissingPath = await _tryPersistMissingRuntimePathForSmoke(testCase.settingKey, missingPath);
          if (!canPersistMissingPath) {
            continue;
          }

          await _expectRuntimeRestartFailure(testCase.runtime);

          final statuses = await WoxApi.instance.getRuntimeStatuses(const UuidV4().generate());
          final status = statuses.firstWhere((status) => status.runtime.toUpperCase() == testCase.runtime);
          expect(status.isStarted, isFalse);
          expect(status.statusCode, equals('executable_missing'));
          expect(status.executablePath, equals(missingPath));
          expect(status.lastStartError, isNotEmpty);
          expect(status.canRestart, isFalse);
        } finally {
          await updateSettingDirect(testCase.settingKey, '');
          await _bestEffortRestartRuntime(testCase.runtime);
        }
      }
    });

    testWidgets('T4-04: Settings store install shows actionable Node.js missing runtime guidance', (tester) async {
      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      final settingController = Get.find<WoxSettingController>();
      final missingPath = _missingExecutablePath('NODEJS');
      var canPersistMissingPath = false;
      try {
        canPersistMissingPath = await _tryPersistMissingRuntimePathForSmoke('CustomNodejsPath', missingPath);
        if (!canPersistMissingPath) {
          return;
        }

        await _expectRuntimeRestartFailure('NODEJS');

        // Settings Store smoke: exercise the same controller path used by the
        // Install button. It must populate runtime diagnosis UI state instead
        // of leaving users with only the raw install exception.
        final traceId = const UuidV4().generate();
        await settingController.loadStorePlugins(traceId);
        await settingController.switchToPluginList(traceId, true);
        final storePlugin = settingController.storePlugins.firstWhere((plugin) => plugin.id == _customCommandsPluginId);
        await settingController.installPlugin(storePlugin);

        final runtimeStatus = settingController.getActionableRuntimeStatusForPlugin(storePlugin);
        expect(settingController.pluginInstallError.value, contains(_customCommandsPluginName));
        expect(settingController.pluginInstallError.value, isNot(contains('runtime is not started')));
        expect(runtimeStatus, isNotNull);
        expect(runtimeStatus!.statusCode, equals('executable_missing'));
        expect(runtimeStatus.executablePath, equals(missingPath));
        expect(runtimeStatus.lastStartError, contains('custom Node.js path does not exist'));
        expect(runtimeStatus.statusMessage, contains('Node.js was not found'));
        expect(runtimeStatus.statusMessage, isNot(contains('failed to prepare')));
        expect(runtimeStatus.installUrl, equals('https://nodejs.org/'));
      } finally {
        if (canPersistMissingPath) {
          await updateSettingDirect('CustomNodejsPath', '');
          await _bestEffortRestartRuntime('NODEJS');
        }
      }
    });

    testWidgets('T4-05: WPM install reports Node.js missing runtime instead of generic not-started error', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final missingPath = _missingExecutablePath('NODEJS');
      var canPersistMissingPath = false;
      try {
        canPersistMissingPath = await _tryPersistMissingRuntimePathForSmoke('CustomNodejsPath', missingPath);
        if (!canPersistMissingPath) {
          return;
        }

        await _expectRuntimeRestartFailure('NODEJS');

        final result = await queryAndWaitForResultWhere(
          tester,
          controller,
          'wpm install $_customCommandsPluginName',
          (candidate) => candidate.title == _customCommandsPluginName,
          description: 'Expected WPM to return the Custom Commands store plugin.',
        );
        final installAction = expectResultActionByName(result, 'Install');
        await controller.executeAction(const UuidV4().generate(), result, installAction);

        await pumpUntil(tester, () {
          final text = controller.resolvedToolbarText ?? '';
          return text.contains('requires Node.js') && !text.contains('runtime is not started') && !text.contains('failed to prepare');
        }, timeout: const Duration(seconds: 10));

        final toolbarText = controller.resolvedToolbarText ?? '';
        expect(toolbarText, contains(_customCommandsPluginName));
        expect(toolbarText, contains('requires Node.js'));
        expect(toolbarText, contains('Node.js was not found'));
        expect(toolbarText, isNot(contains('failed to prepare')));
        expect(toolbarText, isNot(contains('custom Node.js path does not exist')));
        expect(toolbarText, isNot(contains('runtime is not started')));
      } finally {
        if (canPersistMissingPath) {
          await updateSettingDirect('CustomNodejsPath', '');
          await _bestEffortRestartRuntime('NODEJS');
        }
      }
    });
  });
}

String _missingExecutablePath(String runtime) {
  final suffix = Platform.isWindows ? '.exe' : '';
  return '${Directory.systemTemp.path}${Platform.pathSeparator}wox-smoke-missing-${runtime.toLowerCase()}-${DateTime.now().microsecondsSinceEpoch}$suffix';
}

Future<bool> _tryPersistMissingRuntimePathForSmoke(String settingKey, String missingPath) async {
  try {
    await updateSettingDirect(settingKey, missingPath);
    return true;
  } catch (error) {
    // Smoke compatibility: newer core builds reject missing custom runtime paths
    // at save time, which is the safer product behavior. In that contract these
    // missing-runtime install cases cannot be induced through settings, so the
    // test accepts the save-time guard instead of failing before the target path.
    expect(error.toString(), contains('path does not exist'));
    return false;
  }
}

Future<void> _expectRuntimeRestartFailure(String runtime) async {
  try {
    await WoxApi.instance.restartRuntime(const UuidV4().generate(), runtime);
  } catch (_) {
    return;
  }

  fail('Restarting $runtime with a missing executable path should fail.');
}

Future<void> _bestEffortRestartRuntime(String runtime) async {
  try {
    await WoxApi.instance.restartRuntime(const UuidV4().generate(), runtime);
  } catch (_) {
    // Smoke cleanup must tolerate machines that genuinely do not have the
    // runtime installed; the negative tests already assert that diagnosis.
  }
}
