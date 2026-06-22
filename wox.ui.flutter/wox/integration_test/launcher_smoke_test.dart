import 'package:integration_test/integration_test.dart';

import 'launcher_startup_smoke_test.dart';
import 'launcher_onboarding_smoke_test.dart';
import 'launcher_core_smoke_test.dart';
import 'launcher_key_functionality_smoke_test.dart';
import 'launcher_plugin_smoke_test.dart';
import 'launcher_screenshot_smoke_test.dart';
import 'launcher_system_plugin_smoke_test.dart';
import 'launcher_grid_smoke_test.dart';
import 'launcher_refinement_smoke_test.dart';
import 'launcher_toolbar_msg_smoke_test.dart';
import 'launcher_resize_smoke_test.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  // Keep the startup smoke first. It must measure the very first launcher boot,
  // before any other smoke test mutates window state or warms up the UI.
  registerLauncherStartupSmokeTests();
  registerLauncherOnboardingSmokeTests();
  registerLauncherCoreSmokeTests();
  registerLauncherKeyFunctionalitySmokeTests();
  registerLauncherPluginSmokeTests();
  registerLauncherScreenshotSmokeTests();
  registerSystemPluginSmokeTests();
  registerLauncherGridSmokeTests();
  registerLauncherRefinementSmokeTests();
  registerLauncherToolbarMsgSmokeTests();
  registerLauncherResizeSmokeTests();
}
