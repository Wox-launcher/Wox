import 'package:wox/utils/env.dart';

class WoxTestConfig {
  static const String serverPortDefine = 'WOX_TEST_SERVER_PORT';
  static const String _serverPortValue = String.fromEnvironment(serverPortDefine, defaultValue: '${Env.defaultDevServerPort}');

  static int get serverPort => int.tryParse(_serverPortValue) ?? Env.defaultDevServerPort;
}
