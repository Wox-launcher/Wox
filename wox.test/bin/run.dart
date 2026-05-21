import 'dart:async';
import 'dart:convert';
import 'dart:io';

const int defaultDevServerPort = 34987;
const Duration coreStartupTimeout = Duration(minutes: 3);
const Duration runtimeStartupTimeout = Duration(minutes: 2);
const String testWoxDataDirEnv = 'WOX_TEST_DATA_DIR';
const String testUserDataDirEnv = 'WOX_TEST_USER_DIR';
const String testServerPortEnv = 'WOX_TEST_SERVER_PORT';
const String testDisableTelemetryEnv = 'WOX_TEST_DISABLE_TELEMETRY';
const String testNodeTemplatePluginIdEnv = 'WOX_TEST_NODE_TEMPLATE_PLUGIN_ID';
const String testNodeTemplatePluginNameEnv = 'WOX_TEST_NODE_TEMPLATE_PLUGIN_NAME';
const String testNodeTemplatePluginTriggerKeywordEnv = 'WOX_TEST_NODE_TEMPLATE_PLUGIN_TRIGGER_KEYWORD';
const String testPythonTemplatePluginIdEnv = 'WOX_TEST_PYTHON_TEMPLATE_PLUGIN_ID';
const String testPythonTemplatePluginNameEnv = 'WOX_TEST_PYTHON_TEMPLATE_PLUGIN_NAME';
const String testPythonTemplatePluginTriggerKeywordEnv = 'WOX_TEST_PYTHON_TEMPLATE_PLUGIN_TRIGGER_KEYWORD';

const String _templatePluginAuthor = 'Wox Smoke';
const String _templatePluginWebsite = 'https://github.com/Wox-launcher/Wox';

class _SmokeTemplatePluginDefinition {
  const _SmokeTemplatePluginDefinition({
    required this.runtime,
    required this.repoUrl,
    required this.name,
    required this.description,
    required this.triggerKeyword,
    required this.idEnvKey,
    required this.nameEnvKey,
    required this.triggerKeywordEnvKey,
  });

  final String runtime;
  final String repoUrl;
  final String name;
  final String description;
  final String triggerKeyword;
  final String idEnvKey;
  final String nameEnvKey;
  final String triggerKeywordEnvKey;
}

const List<_SmokeTemplatePluginDefinition> _templatePluginDefinitions = [
  _SmokeTemplatePluginDefinition(
    runtime: 'nodejs',
    repoUrl: 'https://github.com/Wox-launcher/Wox.Plugin.Template.Nodejs',
    name: 'SmokeTemplateNodejs',
    description: 'Nodejs smoke template plugin',
    triggerKeyword: 'zzwoxsmoke',
    idEnvKey: testNodeTemplatePluginIdEnv,
    nameEnvKey: testNodeTemplatePluginNameEnv,
    triggerKeywordEnvKey: testNodeTemplatePluginTriggerKeywordEnv,
  ),
  _SmokeTemplatePluginDefinition(
    runtime: 'python',
    repoUrl: 'https://github.com/Wox-launcher/Wox.Plugin.Template.Python',
    name: 'SmokeTemplatePython',
    description: 'Python smoke template plugin',
    triggerKeyword: 'pywoxsmoke',
    idEnvKey: testPythonTemplatePluginIdEnv,
    nameEnvKey: testPythonTemplatePluginNameEnv,
    triggerKeywordEnvKey: testPythonTemplatePluginTriggerKeywordEnv,
  ),
];

class _SmokeTemplatePluginPackage {
  const _SmokeTemplatePluginPackage({required this.definition, required this.packagePath, required this.id, required this.name, required this.triggerKeyword});

  final _SmokeTemplatePluginDefinition definition;

  final String packagePath;
  final String id;
  final String name;
  final String triggerKeyword;
}

Future<void> main(List<String> args) async {
  if (args.isEmpty || args.first == 'help' || args.first == '--help') {
    _printHelp();
    exitCode = 0;
    return;
  }

  switch (args.first) {
    case 'smoke':
      final testName = args.length > 1 ? args.sublist(1).join(' ').trim() : null;
      // Use exit() explicitly because Future.any leaves dangling futures
      // (10-min timeout timer, file-polling loop) that keep the event loop alive.
      exit(await _runSmoke(testName: testName?.isEmpty == true ? null : testName));
    default:
      stderr.writeln('Unknown command: ${args.first}');
      _printHelp();
      exitCode = 64;
  }
}

void _printHelp() {
  stdout.writeln('Usage: dart run bin/run.dart <command> [arguments]');
  stdout.writeln('');
  stdout.writeln('Commands:');
  stdout.writeln('  smoke [test name]    Run the desktop smoke E2E flow');
}

List<_SmokeTemplatePluginDefinition> _templatePluginDefinitionsForSmoke(String? testName) {
  if (testName == null) {
    return _templatePluginDefinitions;
  }

  // Smoke coverage: runtime diagnostic tests intentionally put Node.js/Python
  // into missing-executable states. They do not need packaged template plugins,
  // so skip template setup and host readiness checks to let the negative runtime
  // assertions run on machines where Node.js is actually unavailable.
  const runtimeDiagnosticTests = ['T4-03', 'T4-04', 'T4-05'];
  if (runtimeDiagnosticTests.any(testName.contains)) {
    return const <_SmokeTemplatePluginDefinition>[];
  }

  // Settings-focused smoke cases use built-in settings and installed system
  // plugins only. Skipping template packaging keeps these UI flows away from
  // unrelated template clone/install failures on Windows.
  const settingsFocusedTests = ['T2-15'];
  if (settingsFocusedTests.any(testName.contains)) {
    return const <_SmokeTemplatePluginDefinition>[];
  }

  return _templatePluginDefinitions;
}

Future<int> _runSmoke({String? testName}) async {
  final packageRoot = _resolvePackageRoot();
  final repoRoot = packageRoot.parent;
  final artifactsDir = await _createFreshArtifactsDir(packageRoot);
  final coreBinary = File('${artifactsDir.path}${Platform.pathSeparator}wox-core-smoke${Platform.isWindows ? '.exe' : ''}');
  final woxDataDir = Directory('${artifactsDir.path}${Platform.pathSeparator}wox-data');
  final userDataDir = Directory('${artifactsDir.path}${Platform.pathSeparator}user-data');
  await woxDataDir.create(recursive: true);
  await userDataDir.create(recursive: true);
  final serverPort = await _reserveServerPort();

  final environment = Map<String, String>.from(Platform.environment);
  environment[testWoxDataDirEnv] = woxDataDir.path;
  environment[testUserDataDirEnv] = userDataDir.path;
  environment[testServerPortEnv] = '$serverPort';
  environment[testDisableTelemetryEnv] = 'true';

  stdout.writeln('Artifacts: ${artifactsDir.path}');
  stdout.writeln('Wox data dir: ${woxDataDir.path}');
  stdout.writeln('Wox user dir: ${userDataDir.path}');
  stdout.writeln('Server port: $serverPort');
  if (testName != null) {
    stdout.writeln('Test filter: $testName');
  }

  final coreLog = File('${artifactsDir.path}${Platform.pathSeparator}core.log');
  final testLog = File('${artifactsDir.path}${Platform.pathSeparator}flutter_test.log');
  final templatePluginLog = File('${artifactsDir.path}${Platform.pathSeparator}template_plugin.log');

  final templateDefinitions = _templatePluginDefinitionsForSmoke(testName);
  final templatePlugins = <_SmokeTemplatePluginPackage>[];
  for (final definition in templateDefinitions) {
    stdout.writeln('Preparing template ${definition.runtime} plugin package...');
    final templatePlugin = await _prepareSmokeTemplatePlugin(definition: definition, artifactsDir: artifactsDir, environment: environment, logFile: templatePluginLog);
    templatePlugins.add(templatePlugin);
    environment[definition.idEnvKey] = templatePlugin.id;
    environment[definition.nameEnvKey] = templatePlugin.name;
    environment[definition.triggerKeywordEnvKey] = templatePlugin.triggerKeyword;
    stdout.writeln('Template ${definition.runtime} plugin package: ${templatePlugin.packagePath}');
    stdout.writeln('Template ${definition.runtime} plugin id: ${templatePlugin.id}');
    stdout.writeln('Template ${definition.runtime} trigger keyword: ${templatePlugin.triggerKeyword}');
  }

  Process? coreProcess;
  try {
    stdout.writeln('Building core smoke binary...');
    await _buildCoreBinary(workingDirectory: '${repoRoot.path}${Platform.pathSeparator}wox.core', outputPath: coreBinary.path, environment: environment);

    coreProcess = await _startCommand(coreBinary.path, [], workingDirectory: '${repoRoot.path}${Platform.pathSeparator}wox.core', environment: environment);
    _pipeProcessOutput(coreProcess, coreLog, '[core]');

    final ready = await _waitForPingReady(serverPort: serverPort, timeout: coreStartupTimeout);
    if (!ready) {
      stderr.writeln('wox.core did not become ready on port $serverPort.');
      return 1;
    }

    if (templatePlugins.isNotEmpty) {
      // Bug fix: /ping only proves the core HTTP server is ready. Template plugin
      // installation depends on the NodeJS/Python runtime hosts, which start
      // asynchronously after core startup, so wait for those hosts explicitly.
      final runtimeReady = await _waitForRuntimeHostsReady(
        serverPort: serverPort,
        runtimes: templatePlugins.map((plugin) => plugin.definition.runtime),
        timeout: runtimeStartupTimeout,
      );
      if (!runtimeReady) {
        await _printRuntimeStartupDiagnostics(woxDataDir);
        return 1;
      }
    }

    for (final templatePlugin in templatePlugins) {
      stdout.writeln('Installing packaged ${templatePlugin.definition.runtime} template plugin into isolated smoke environment...');
      await _installLocalPluginPackage(serverPort: serverPort, packagePath: templatePlugin.packagePath);
    }

    final flutterBuildConflict = await _findRunningFlutterBuildExecutable(repoRoot);
    if (flutterBuildConflict != null) {
      stderr.writeln('Close the running Flutter development UI before smoke test: $flutterBuildConflict');
      return 1;
    }

    final flutterArgs = <String>[
      'test',
      '--dart-define=$testServerPortEnv=$serverPort',
      if (testName != null) ...['--plain-name', testName],
      'integration_test/launcher_smoke_test.dart',
    ];

    final flutterProcess = await _startCommand(
      'flutter',
      flutterArgs,
      workingDirectory: '${repoRoot.path}${Platform.pathSeparator}wox.ui.flutter${Platform.pathSeparator}wox',
      environment: environment,
    );
    const completionMarkers = ['All tests passed!', 'Some tests failed.'];
    final testsFinished = _pipeProcessOutput(flutterProcess, testLog, '[flutter-test]', completionMarkers: completionMarkers);
    final completionDetected = Future.any<bool>([testsFinished.future, _waitForCompletionMarkerInFile(testLog, completionMarkers)]);

    // On macOS the integration test app may not exit on its own after all
    // tests finish.  Wait for the completion marker, then give the process a
    // short grace period before terminating it.
    int flutterExitCode;

    // First, wait for the process to exit naturally OR for tests to finish.
    // Also add a hard timeout so CI never hangs indefinitely.
    final processExit = flutterProcess.exitCode;
    final result = await Future.any([
      processExit.then((code) => ('exited', code)),
      completionDetected.then((passed) => ('finished', passed ? 0 : 1)),
      Future.delayed(const Duration(minutes: 10), () => ('timeout', 1)),
    ]);

    if (result.$1 == 'exited') {
      flutterExitCode = result.$2;
    } else {
      // Tests finished (or hard timeout reached) but process is still running.
      // Give it a short grace period to exit cleanly, then force-terminate.
      flutterExitCode = result.$2;
      if (result.$1 == 'timeout') {
        stderr.writeln('flutter test process hit hard timeout (5 min), terminating...');
      }
      try {
        await processExit.timeout(const Duration(seconds: 10));
      } on TimeoutException {
        stdout.writeln('flutter test process did not exit after tests completed, terminating...');
        await _terminateProcess(flutterProcess);
      }
    }
    return flutterExitCode;
  } finally {
    if (coreProcess != null) {
      await _terminateProcess(coreProcess);
    }
  }
}

Future<void> _buildCoreBinary({required String workingDirectory, required String outputPath, required Map<String, String> environment}) async {
  final buildArgs = ['build', '-o', outputPath, '.'];
  final buildProcess = await _startCommand('go', buildArgs, workingDirectory: workingDirectory, environment: environment);

  final stdoutBuffer = StringBuffer();
  final stderrBuffer = StringBuffer();
  buildProcess.stdout.transform(utf8.decoder).listen(stdoutBuffer.write);
  buildProcess.stderr.transform(utf8.decoder).listen(stderrBuffer.write);

  final exitCode = await buildProcess.exitCode;
  if (exitCode != 0) {
    final message = [
      'Failed to build smoke core binary (exit $exitCode).',
      if (stdoutBuffer.isNotEmpty) stdoutBuffer.toString().trimRight(),
      if (stderrBuffer.isNotEmpty) stderrBuffer.toString().trimRight(),
    ].where((part) => part.isNotEmpty).join('\n');
    throw ProcessException('go', buildArgs, message, exitCode);
  }
}

Future<Process> _startCommand(String command, List<String> arguments, {required String workingDirectory, required Map<String, String> environment}) {
  if (Platform.isWindows) {
    return Process.start('cmd.exe', ['/c', command, ...arguments], workingDirectory: workingDirectory, environment: environment);
  }

  return Process.start(command, arguments, workingDirectory: workingDirectory, environment: environment);
}

Future<String?> _findRunningFlutterBuildExecutable(Directory repoRoot) async {
  if (!Platform.isWindows) {
    return null;
  }

  final targetPath =
      '${repoRoot.path}${Platform.pathSeparator}wox.ui.flutter${Platform.pathSeparator}wox${Platform.pathSeparator}build${Platform.pathSeparator}windows${Platform.pathSeparator}x64${Platform.pathSeparator}runner${Platform.pathSeparator}Debug${Platform.pathSeparator}wox-ui.exe';
  final result = await Process.run('powershell.exe', ['-NoProfile', '-Command', 'Get-Process wox-ui -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Path']);

  final runningPaths = result.stdout.toString().split(RegExp(r'[\r\n]+')).map((line) => line.trim()).where((line) => line.isNotEmpty);
  for (final path in runningPaths) {
    if (path.toLowerCase() == targetPath.toLowerCase()) {
      return path;
    }
  }

  return null;
}

Directory _resolvePackageRoot() {
  final current = Directory.current;
  if (File('${current.path}${Platform.pathSeparator}pubspec.yaml').existsSync() && current.path.endsWith('${Platform.pathSeparator}wox.test')) {
    return current;
  }

  final nested = Directory('${current.path}${Platform.pathSeparator}wox.test');
  if (File('${nested.path}${Platform.pathSeparator}pubspec.yaml').existsSync()) {
    return nested;
  }

  throw StateError('Unable to locate wox.test package root from ${current.path}. Run this command from wox.test or the repository root.');
}

Future<Directory> _createFreshArtifactsDir(Directory packageRoot) async {
  final artifactsRoot = Directory('${packageRoot.path}${Platform.pathSeparator}artifacts');
  // Keep only the current smoke run artifacts to avoid unbounded accumulation.
  if (await artifactsRoot.exists()) {
    await artifactsRoot.delete(recursive: true);
  }
  await artifactsRoot.create(recursive: true);

  final timestamp = _formatLocalArtifactsTimestamp(DateTime.now());
  final dir = Directory('${artifactsRoot.path}${Platform.pathSeparator}$timestamp');
  await dir.create(recursive: true);
  return dir;
}

String _formatLocalArtifactsTimestamp(DateTime value) {
  String twoDigits(int part) => part.toString().padLeft(2, '0');

  return '${value.year}-${twoDigits(value.month)}-${twoDigits(value.day)} '
      '${twoDigits(value.hour)}-${twoDigits(value.minute)}-${twoDigits(value.second)}';
}

Future<int> _reserveServerPort() async {
  try {
    final preferredSocket = await ServerSocket.bind(InternetAddress.loopbackIPv4, defaultDevServerPort);
    final port = preferredSocket.port;
    await preferredSocket.close();
    return port;
  } on SocketException {
    final fallbackSocket = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final port = fallbackSocket.port;
    await fallbackSocket.close();
    return port;
  }
}

/// Pipes process stdout/stderr to [outputFile] and the console.
///
/// When [completionMarkers] is provided, the returned [Completer] completes
/// as soon as any marker string appears in stdout or stderr.  If
/// [completionMarkers] is null the completer never completes on its own.
Completer<bool> _pipeProcessOutput(Process process, File outputFile, String prefix, {List<String>? completionMarkers}) {
  final testsFinished = Completer<bool>();
  final sink = outputFile.openWrite(mode: FileMode.writeOnlyAppend);
  // Buffer recent output to detect markers that span chunk boundaries.
  final _recentOutput = StringBuffer();

  void _checkMarkers(String text) {
    if (completionMarkers == null || testsFinished.isCompleted) return;
    _recentOutput.write(text);
    // Keep only the last 4 KB to bound memory usage.
    if (_recentOutput.length > 4096) {
      final s = _recentOutput.toString();
      _recentOutput.clear();
      _recentOutput.write(s.substring(s.length - 2048));
    }
    final buffer = _recentOutput.toString();
    if (completionMarkers.any((m) => buffer.contains(m))) {
      testsFinished.complete(buffer.contains('All tests passed!'));
    }
  }

  void forward(List<int> data, IOSink target) {
    sink.add(data);
    target.add(data);
    _checkMarkers(utf8.decode(data, allowMalformed: true));
  }

  process.stdout.listen((data) => forward(data, stdout));
  process.stderr.listen((data) => forward(data, stderr));

  unawaited(
    process.exitCode.whenComplete(() async {
      await sink.flush();
      await sink.close();
    }),
  );
  stdout.writeln('$prefix output -> ${outputFile.path}');
  return testsFinished;
}

Future<bool> _waitForCompletionMarkerInFile(File outputFile, List<String> completionMarkers) async {
  var previousLength = -1;

  while (true) {
    try {
      if (await outputFile.exists()) {
        final text = await outputFile.readAsString();
        if (text.length != previousLength) {
          previousLength = text.length;
          if (text.contains('All tests passed!')) {
            return true;
          }
          if (text.contains('Some tests failed.')) {
            return false;
          }
          for (final marker in completionMarkers) {
            if (text.contains(marker)) {
              return marker == 'All tests passed!';
            }
          }
        }
      }
    } catch (_) {
      // Ignore transient file-read failures while the log file is still being written.
    }

    await Future<void>.delayed(const Duration(milliseconds: 250));
  }
}

Future<bool> _waitForPingReady({required int serverPort, required Duration timeout}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (await _isPingReady(serverPort)) {
      return true;
    }
    await Future<void>.delayed(const Duration(milliseconds: 500));
  }
  return false;
}

Future<bool> _isPingReady(int serverPort) async {
  final client = HttpClient();
  try {
    final request = await client.getUrl(Uri.parse('http://127.0.0.1:$serverPort/ping'));
    final response = await request.close();
    await response.drain<void>();
    return response.statusCode == 200;
  } catch (_) {
    return false;
  } finally {
    client.close(force: true);
  }
}

class _RuntimeHostStatus {
  const _RuntimeHostStatus({required this.runtime, required this.isStarted, required this.statusCode, required this.executablePath, required this.lastStartError});

  final String runtime;
  final bool isStarted;
  final String statusCode;
  final String executablePath;
  final String lastStartError;
}

Future<bool> _waitForRuntimeHostsReady({required int serverPort, required Iterable<String> runtimes, required Duration timeout}) async {
  final requiredRuntimes = runtimes.map((runtime) => runtime.toUpperCase()).toSet();
  final deadline = DateTime.now().add(timeout);
  var nextStatusLogAt = DateTime.now();
  var lastStatusSummary = 'runtime status unavailable';

  while (DateTime.now().isBefore(deadline)) {
    final statuses = await _fetchRuntimeStatuses(serverPort);
    if (statuses != null) {
      lastStatusSummary = _formatRuntimeStatusSummary(statuses);
      final startedRuntimes = statuses.where((status) => status.isStarted).map((status) => status.runtime.toUpperCase()).toSet();
      if (requiredRuntimes.every(startedRuntimes.contains)) {
        stdout.writeln('Runtime hosts ready: $lastStatusSummary');
        return true;
      }
    }

    if (!DateTime.now().isBefore(nextStatusLogAt)) {
      stdout.writeln('Waiting for runtime hosts (${requiredRuntimes.join(', ')}): $lastStatusSummary');
      nextStatusLogAt = DateTime.now().add(const Duration(seconds: 5));
    }
    await Future<void>.delayed(const Duration(milliseconds: 500));
  }

  stderr.writeln('Runtime hosts did not become ready within ${timeout.inSeconds}s: $lastStatusSummary');
  return false;
}

Future<List<_RuntimeHostStatus>?> _fetchRuntimeStatuses(int serverPort) async {
  final client = HttpClient();
  try {
    final request = await client.getUrl(Uri.parse('http://127.0.0.1:$serverPort/runtime/status'));
    final response = await request.close();
    final body = await response.transform(utf8.decoder).join();
    if (response.statusCode != 200 || !_isSuccessfulRestResponse(body)) {
      return null;
    }

    final decoded = jsonDecode(body);
    if (decoded is! Map<String, dynamic>) {
      return null;
    }
    final data = decoded['Data'];
    if (data is! List<dynamic>) {
      return null;
    }

    return data
        .whereType<Map<String, dynamic>>()
        .map((item) {
          return _RuntimeHostStatus(
            runtime: item['Runtime']?.toString() ?? '',
            isStarted: item['IsStarted'] == true,
            statusCode: item['StatusCode']?.toString() ?? '',
            executablePath: item['ExecutablePath']?.toString() ?? '',
            lastStartError: item['LastStartError']?.toString() ?? '',
          );
        })
        .where((status) => status.runtime.isNotEmpty)
        .toList();
  } catch (_) {
    return null;
  } finally {
    client.close(force: true);
  }
}

String _formatRuntimeStatusSummary(List<_RuntimeHostStatus> statuses) {
  if (statuses.isEmpty) {
    return 'no runtime statuses returned';
  }

  // Runtime startup diagnostics now include the structured status fields added
  // for user-facing errors, so CI can distinguish a missing interpreter from a
  // host process that failed after launch.
  return statuses
      .map((status) {
        final parts = <String>['${status.runtime}=${status.isStarted ? 'started' : 'stopped'}'];
        if (status.statusCode.isNotEmpty) {
          parts.add('status=${status.statusCode}');
        }
        if (status.executablePath.isNotEmpty) {
          parts.add('path=${status.executablePath}');
        }
        if (status.lastStartError.isNotEmpty) {
          parts.add('error=${status.lastStartError}');
        }
        return parts.join(' ');
      })
      .join(', ');
}

Future<void> _printRuntimeStartupDiagnostics(Directory woxDataDir) async {
  // Bug fix: runtime startup failures used to surface as a generic install
  // error. Printing the relevant log tails in CI keeps the failure actionable
  // without requiring a separate artifact download for the common case.
  stdout.writeln('Runtime startup diagnostics:');
  await _printLogTail(File('${woxDataDir.path}${Platform.pathSeparator}log${Platform.pathSeparator}log'), 'wox log');

  final hostLogDir = Directory('${woxDataDir.path}${Platform.pathSeparator}log${Platform.pathSeparator}hosts');
  if (!await hostLogDir.exists()) {
    stdout.writeln('[host logs] directory does not exist: ${hostLogDir.path}');
    return;
  }

  final hostLogs = await hostLogDir.list().where((entry) => entry is File && entry.path.endsWith('.log')).cast<File>().toList();
  hostLogs.sort((a, b) => a.path.compareTo(b.path));
  if (hostLogs.isEmpty) {
    stdout.writeln('[host logs] no host log files found in ${hostLogDir.path}');
    return;
  }
  for (final hostLog in hostLogs) {
    await _printLogTail(hostLog, hostLog.uri.pathSegments.last);
  }
}

Future<void> _printLogTail(File file, String label, {int maxLines = 80}) async {
  if (!await file.exists()) {
    stdout.writeln('[$label] file does not exist: ${file.path}');
    return;
  }

  try {
    final lines = await file.readAsLines();
    final start = lines.length > maxLines ? lines.length - maxLines : 0;
    stdout.writeln('--- $label tail (${lines.length - start} lines) ---');
    for (final line in lines.skip(start)) {
      stdout.writeln(line);
    }
    stdout.writeln('--- end $label tail ---');
  } catch (error) {
    stdout.writeln('[$label] failed to read ${file.path}: $error');
  }
}

Future<_SmokeTemplatePluginPackage> _prepareSmokeTemplatePlugin({
  required _SmokeTemplatePluginDefinition definition,
  required Directory artifactsDir,
  required Map<String, String> environment,
  required File logFile,
}) async {
  final pluginEnvironment = Map<String, String>.from(environment);
  if (Platform.isWindows && definition.runtime == 'nodejs') {
    // Bug fix: pnpm's default isolated linker creates a dense symlink tree that
    // repeatedly hits Windows EBUSY during smoke setup. Hoisted/copy mode keeps
    // the template package build deterministic while avoiding that fragile
    // symlink layout in the temporary artifacts directory.
    pluginEnvironment['npm_config_node_linker'] = 'hoisted';
    pluginEnvironment['npm_config_package_import_method'] = 'copy';
  }

  final workspaceDir = Directory('${artifactsDir.path}${Platform.pathSeparator}template-plugin-workspace');
  final repoName = definition.repoUrl.split('/').last;
  final pluginRoot = Directory('${workspaceDir.path}${Platform.pathSeparator}$repoName');
  await workspaceDir.create(recursive: true);
  if (await pluginRoot.exists()) {
    await pluginRoot.delete(recursive: true);
  }

  await _runLoggedCommand(
    'git',
    ['clone', '--depth', '1', definition.repoUrl, pluginRoot.path],
    workingDirectory: workspaceDir.path,
    environment: pluginEnvironment,
    outputFile: logFile,
    prefix: '[template-plugin:${definition.runtime}]',
  );

  final initInput = [definition.name, definition.description, definition.triggerKeyword, _templatePluginAuthor, _templatePluginWebsite, 'y'].join('\n');

  await _runLoggedCommand(
    'make',
    ['init'],
    workingDirectory: pluginRoot.path,
    environment: pluginEnvironment,
    outputFile: logFile,
    prefix: '[template-plugin:${definition.runtime}]',
    stdinData: '$initInput\n',
  );
  await _runTemplateInstallWithRetry(definition: definition, pluginRoot: pluginRoot, environment: pluginEnvironment, logFile: logFile);
  await _runLoggedCommand(
    'make',
    ['package'],
    workingDirectory: pluginRoot.path,
    environment: pluginEnvironment,
    outputFile: logFile,
    prefix: '[template-plugin:${definition.runtime}]',
  );

  final pluginJson = jsonDecode(await File('${pluginRoot.path}${Platform.pathSeparator}plugin.json').readAsString()) as Map<String, dynamic>;
  final packageName = definition.name.trim().toLowerCase();
  final packagePath = '${pluginRoot.path}${Platform.pathSeparator}wox.plugin.$packageName.wox';
  if (!await File(packagePath).exists()) {
    throw StateError('Expected packaged template plugin at $packagePath, but it was not created.');
  }

  return _SmokeTemplatePluginPackage(
    definition: definition,
    packagePath: packagePath,
    id: pluginJson['Id'] as String,
    name: pluginJson['Name'] as String,
    triggerKeyword:
        (() {
          final keywords = pluginJson['TriggerKeywords'] as List<dynamic>;
          if (keywords.isEmpty) {
            return '';
          }
          return keywords.first.toString();
        })(),
  );
}

Future<void> _runTemplateInstallWithRetry({
  required _SmokeTemplatePluginDefinition definition,
  required Directory pluginRoot,
  required Map<String, String> environment,
  required File logFile,
}) async {
  const maxAttempts = 3;
  for (var attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      await _runLoggedCommand(
        'make',
        ['install'],
        workingDirectory: pluginRoot.path,
        environment: environment,
        outputFile: logFile,
        prefix: '[template-plugin:${definition.runtime}]',
      );
      return;
    } on ProcessException {
      final canRetry = definition.runtime == 'nodejs' && attempt < maxAttempts && await _logContainsPnpmBusyFailure(logFile);
      if (!canRetry) {
        rethrow;
      }

      stderr.writeln('[template-plugin:${definition.runtime}] pnpm hit a transient EBUSY symlink failure; retrying install (${attempt + 1}/$maxAttempts).');
      final nodeModules = Directory('${pluginRoot.path}${Platform.pathSeparator}node_modules');
      if (await nodeModules.exists()) {
        // Bug fix: Windows can leave a partially-created pnpm symlink tree after
        // EBUSY. Removing node_modules before retrying avoids reusing the broken
        // install state while keeping the retry scoped to smoke setup only.
        await nodeModules.delete(recursive: true);
      }
      await Future<void>.delayed(Duration(seconds: attempt * 2));
    }
  }
}

Future<bool> _logContainsPnpmBusyFailure(File logFile) async {
  if (!await logFile.exists()) {
    return false;
  }

  final contents = await logFile.readAsString();
  return contents.contains('ERR_PNPM_EBUSY') || contents.contains('EBUSY: resource busy or locked');
}

Future<void> _installLocalPluginPackage({required int serverPort, required String packagePath}) async {
  final client = HttpClient();
  final deadline = DateTime.now().add(const Duration(seconds: 45));
  var lastError = 'unknown error';

  try {
    while (DateTime.now().isBefore(deadline)) {
      try {
        final request = await client.postUrl(Uri.parse('http://127.0.0.1:$serverPort/test/plugin/install_local'));
        request.headers.contentType = ContentType.json;
        request.write(jsonEncode({'FilePath': packagePath}));

        final response = await request.close();
        final body = await response.transform(utf8.decoder).join();
        if (response.statusCode == 200 && _isSuccessfulRestResponse(body)) {
          return;
        }

        lastError = _extractRestErrorMessage(body) ?? (body.trim().isEmpty ? 'HTTP ${response.statusCode}' : body.trim());
      } catch (error) {
        lastError = error.toString();
      }

      await Future<void>.delayed(const Duration(seconds: 1));
    }
  } finally {
    client.close(force: true);
  }

  throw StateError('Failed to install local smoke template plugin package: $lastError');
}

bool _isSuccessfulRestResponse(String body) {
  if (body.trim().isEmpty) {
    return false;
  }

  try {
    final decoded = jsonDecode(body);
    if (decoded is Map<String, dynamic>) {
      return decoded['Success'] == true;
    }
  } catch (_) {
    return false;
  }

  return false;
}

String? _extractRestErrorMessage(String body) {
  if (body.trim().isEmpty) {
    return null;
  }

  try {
    final decoded = jsonDecode(body);
    if (decoded is Map<String, dynamic>) {
      final message = decoded['Message']?.toString().trim() ?? '';
      if (message.isNotEmpty) {
        return message;
      }
    }
  } catch (_) {
    return null;
  }

  return null;
}

Future<void> _runLoggedCommand(
  String command,
  List<String> arguments, {
  required String workingDirectory,
  required Map<String, String> environment,
  required File outputFile,
  required String prefix,
  String? stdinData,
}) async {
  final process = await _startCommand(command, arguments, workingDirectory: workingDirectory, environment: environment);
  final sink = outputFile.openWrite(mode: FileMode.writeOnlyAppend);
  sink.writeln('> $command ${arguments.join(' ')}');

  if (stdinData != null) {
    process.stdin.write(stdinData);
  }
  await process.stdin.close();

  Future<void> forward(Stream<List<int>> stream, IOSink target) async {
    await for (final chunk in stream) {
      sink.add(chunk);
      target.add(chunk);
    }
  }

  final stdoutDone = forward(process.stdout, stdout);
  final stderrDone = forward(process.stderr, stderr);
  final exitCode = await process.exitCode;
  await Future.wait([stdoutDone, stderrDone]);
  sink.writeln('exitCode: $exitCode');
  await sink.flush();
  await sink.close();

  if (exitCode != 0) {
    throw ProcessException(command, arguments, '$prefix command failed with exit $exitCode. See ${outputFile.path} for details.', exitCode);
  }
}

Future<void> _terminateProcess(Process process) async {
  if (Platform.isWindows) {
    await Process.run('taskkill', ['/PID', '${process.pid}', '/T', '/F']);
    return;
  }

  process.kill();
  try {
    await process.exitCode.timeout(const Duration(seconds: 5));
  } on TimeoutException {
    process.kill(ProcessSignal.sigkill);
    await process.exitCode;
  }
}
